package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
	"github.com/3toInf/hetu/internal/notify"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "t.sqlite")
}

func statusRunningForTest() session.Status {
	return session.StatusRunning
}

func statusCompletedForTest() session.Status {
	return session.StatusCompleted
}

func newMgr(t *testing.T) (*SessionManager, *fake.Session) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	r := project.NewResolver(st)
	fs := fake.NewSession("h1", "ext1", statusRunningForTest())
	fa := &fake.Agent{N: "claude", Drv: &fake.Driver{OnStart: func(_ context.Context, _ agent.StartRequest) (agent.Session, error) {
		return fs, nil
	}}}
	m := NewSessionManager(st, r, map[string]agent.Agent{"claude": fa})
	return m, fs
}

func TestEnsureAndSubscribe(t *testing.T) {
	m, fs := newMgr(t)
	ctx := context.Background()
	id, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := m.Subscribe(id)
	if err != nil {
		t.Fatal(err)
	}
	fs.Emit(agent.Event{Type: agent.EventText, Text: "hi"})
	fs.SetStatus(statusCompletedForTest())
	fs.Emit(agent.Event{Type: agent.EventStatus, Status: statusCompletedForTest()})
	select {
	case ev := <-sub:
		if ev.Type != agent.EventText || ev.Text != "hi" {
			t.Fatalf("received unexpected event: got Type=%v, Text=%q; want Type=%v, Text=%q", ev.Type, ev.Text, agent.EventText, "hi")
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive event")
	}
}

// TestEnsurePreservesDiscoveredID guards against resume renaming a session's
// hetu_id: a discovered session is seeded with a known id/external pair, and
// resuming it must return that same id (not a freshly minted uuid), otherwise
// the truncated id shown by `hetu sessions` becomes invalid.
func TestEnsurePreservesDiscoveredID(t *testing.T) {
	m, _ := newMgr(t)
	ctx := context.Background()

	const (
		seedHetu = "abcdef1234567890"
		ext      = "ext-discovered"
	)
	// Seed the store as discovery would: a completed, non-driven session.
	st := m.store
	if _, err := st.UpsertSession(ctx, store.Session{
		HetuID: seedHetu, Agent: "claude", ExternalID: ext, CWD: "/x",
		Status: session.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}

	id, err := m.Ensure(ctx, "claude", ext, "/x")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if id != seedHetu {
		t.Fatalf("Ensure renamed hetu_id: got %q, want %q (resume must preserve the discovered id)", id, seedHetu)
	}

	// The stored row must still carry the original id.
	got, ok, _ := st.GetSession(ctx, seedHetu)
	if !ok {
		t.Fatal("seeded session disappeared after resume")
	}
	if got.HetuID != seedHetu {
		t.Fatalf("store hetu_id changed to %q", got.HetuID)
	}
}

// newDrivenManager creates a SessionManager with a live session and returns both the manager and hetuID
func newDrivenManager(t *testing.T) (*SessionManager, string) {
	m, _ := newMgr(t)
	ctx := context.Background()
	hid, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}
	// Verify the session is actually live by checking status
	if _, ok := m.LiveStatus(hid); !ok {
		t.Fatal("session not live after Ensure")
	}
	return m, hid
}

// newDrivenManagerWithNotifier creates a SessionManager with a RecordingNotifier for testing
func newDrivenManagerWithNotifier(t *testing.T) (*SessionManager, string, *notify.RecordingNotifier, *fake.Session) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	r := project.NewResolver(st)
	fs := fake.NewSession("h1", "ext1", statusRunningForTest())
	fa := &fake.Agent{N: "claude", Drv: &fake.Driver{OnStart: func(_ context.Context, _ agent.StartRequest) (agent.Session, error) {
		return fs, nil
	}}}

	// Create a recording notifier for testing
	fakeNotifier := &notify.RecordingNotifier{}

	m := newSessionManagerWithNotifier(st, r, map[string]agent.Agent{"claude": fa}, fakeNotifier)

	hid, err := m.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		t.Fatal(err)
	}
	// Verify the session is actually live by checking status
	if _, ok := m.LiveStatus(hid); !ok {
		t.Fatal("session not live after Ensure")
	}
	return m, hid, fakeNotifier, fs
}

// waitForApprovalPending waits until an approval pending event is seen for the given toolUseID
func waitForApprovalPending(t *testing.T, m *SessionManager, hetuID, toolUseID string) {
	t.Helper()
	// Poll the pending map (not a subscriber): RequestApproval registers the
	// pending BEFORE broadcasting, so map visibility is deterministic even if
	// a subscriber attaches after the broadcast.
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, p := range m.PendingApprovals(hetuID) {
			if p.ToolUseID == toolUseID {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("pending approval %s never registered", toolUseID)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitForTrue polls cond until true or the deadline — pump persistence
// (markUnread / notify) happens AFTER broadcast, so observing the event on a
// subscriber does not imply the store write has landed.
func waitForTrue(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestRequestApprovalBlocksUntilResolved(t *testing.T) {
	mgr, hid := newDrivenManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type result struct{ d ApprovalDecision; err error }
	got := make(chan result, 1)
	go func() {
		d, err := mgr.RequestApproval(ctx, hid, "toolu_1", "Bash", `{"command":"ls"}`)
		got <- result{d, err}
	}()

	// drain broadcasts until we see the pending approval event
	waitForApprovalPending(t, mgr, hid, "toolu_1")

	ok := mgr.ResolveApproval(hid, "toolu_1", ApprovalDecision{Allow: true})
	if !ok {
		t.Fatal("ResolveApproval reported no matching pending")
	}
	select {
	case r := <-got:
		if !r.d.Allow || r.err != nil {
			t.Fatalf("RequestApproval returned %+v %v, want allow and no error", r.d, r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("RequestApproval did not unblock after ResolveApproval")
	}
}

func TestRequestApprovalTimeoutDenies(t *testing.T) {
	mgr, hid := newDrivenManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	d, err := mgr.RequestApproval(ctx, hid, "toolu_2", "Write", `{}`)
	if err == nil && d.Allow {
		t.Fatal("expected deny on ctx cancel, got allow")
	}
}

func TestRequestPermissionReadOnlyShortCircuits(t *testing.T) {
	mgr, hid := newDrivenManager(t)
	ctx := cancelCtx(context.Background())

	// Read-only tools should allow immediately and register NO pending
	d, err := mgr.RequestPermission(ctx, hid, "toolu_read", "Read", `{"file_path":"/tmp/test"}`)
	if err != nil {
		t.Fatalf("RequestPermission for Read failed: %v", err)
	}
	if !d.Allow {
		t.Fatal("Read tool should be auto-allowed")
	}
	pending := mgr.PendingApprovals(hid)
	if len(pending) != 0 {
		t.Fatalf("Read tool should not register pending, got %d", len(pending))
	}

	// Bash should register a pending (blocks, resolved by ResolveApproval)
	type result struct{ d ApprovalDecision; err error }
	got := make(chan result, 1)
	go func() {
		d, err := mgr.RequestPermission(ctx, hid, "toolu_bash", "Bash", `{"command":"ls"}`)
		got <- result{d, err}
	}()

	// Wait for the pending to be registered
	waitForApprovalPending(t, mgr, hid, "toolu_bash")

	pending = mgr.PendingApprovals(hid)
	if len(pending) != 1 || pending[0].ToolUseID != "toolu_bash" {
		t.Fatalf("expected 1 pending for Bash, got %+v", pending)
	}

	// Resolve it
	ok := mgr.ResolveApproval(hid, "toolu_bash", ApprovalDecision{Allow: true})
	if !ok {
		t.Fatal("ResolveApproval reported no matching pending")
	}

	select {
	case r := <-got:
		if !r.d.Allow || r.err != nil {
			t.Fatalf("RequestPermission for Bash returned %+v %v", r.d, r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("RequestPermission did not unblock after ResolveApproval")
	}
}

func TestPendingApprovals(t *testing.T) {
	mgr, hid := newDrivenManager(t)
	ctx := cancelCtx(context.Background())

	// Create multiple pending approvals
	go func() {
		mgr.RequestApproval(ctx, hid, "toolu_1", "Bash", `{"command":"ls"}`)
	}()
	waitForApprovalPending(t, mgr, hid, "toolu_1")

	go func() {
		mgr.RequestApproval(ctx, hid, "toolu_2", "Write", `{"path":"/tmp/test"}`)
	}()
	waitForApprovalPending(t, mgr, hid, "toolu_2")

	pending := mgr.PendingApprovals(hid)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending approvals, got %d", len(pending))
	}

	// Clean up
	mgr.ResolveApproval(hid, "toolu_1", ApprovalDecision{Allow: true})
	mgr.ResolveApproval(hid, "toolu_2", ApprovalDecision{Allow: false})
}

func cancelCtx(ctx context.Context) context.Context {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	go func() {
		<-time.After(5 * time.Second)
		cancel()
	}()
	return ctx
}

func TestPumpSetsUnreadAndNotifiesOnWaiting(t *testing.T) {
	mgr, hid, fake, fs := newDrivenManagerWithNotifier(t)
	ctx := context.Background()
	evs, err := mgr.Subscribe(hid)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a text event to set unread
	fs.Emit(agent.Event{Type: agent.EventText, Text: "hi", Seq: 1})

	// Wait for the event to be processed
	select {
	case ev := <-evs:
		if ev.Type != agent.EventText {
			t.Fatalf("expected text event, got %v", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for text event")
	}

	// Check that unread=1 — poll: markUnread lands AFTER the broadcast.
	waitForTrue(t, func() bool {
		se, ok, _ := mgr.store.GetSession(ctx, hid)
		return ok && se.Unread
	}, "expected unread=1 after pump event")

	// Now simulate a status transition to WaitingForApproval
	fs.Emit(agent.Event{Type: agent.EventStatus, Status: session.StatusWaitingForApproval, Seq: 2})

	// Wait for the status event
	select {
	case ev := <-evs:
		if ev.Type != agent.EventStatus {
			t.Fatalf("expected status event, got %v", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for status event")
	}

	// Check that a notification was sent — poll: maybeNotify runs AFTER the
	// broadcast (use the thread-safe getter).
	waitForTrue(t, func() bool { return len(fake.GetSent()) > 0 }, "expected at least one notification")
	sent := fake.GetSent()
	if len(sent) == 0 {
		t.Errorf("expected at least one notification, got none")
	} else {
		lastNotif := sent[len(sent)-1]
		if lastNotif.Body == "" {
			t.Errorf("expected notification with non-empty body, got %+v", lastNotif)
		}
		if lastNotif.Title != "hetu" {
			t.Errorf("expected title 'hetu', got %q", lastNotif.Title)
		}
	}
}

// TestMetaEventPersistsExternalID verifies that EventMeta causes the external_id to be persisted.
func TestMetaEventPersistsExternalID(t *testing.T) {
	mgr, hid, _, fs := newDrivenManagerWithNotifier(t)
	ctx := context.Background()

	// Initial session should have external_id="ext1" (as created in newDrivenManagerWithNotifier)
	se, ok, _ := mgr.store.GetSession(ctx, hid)
	if !ok {
		t.Fatal("session not found")
	}
	initialID := se.ExternalID
	if initialID != "ext1" {
		t.Fatalf("initial external_id should be ext1, got %q", initialID)
	}

	// Emit a meta event with a different session_id
	fs.Emit(agent.Event{Type: agent.EventMeta, ExternalID: "claude-session-abc123", Seq: 1})

	// Give the pump time to process
	time.Sleep(100 * time.Millisecond)

	// Verify the external_id was persisted
	se, ok, _ = mgr.store.GetSession(ctx, hid)
	if !ok {
		t.Fatal("session not found after meta event")
	}
	if se.ExternalID != "claude-session-abc123" {
		t.Fatalf("external_id not persisted: got %q, want %q", se.ExternalID, "claude-session-abc123")
	}

	// Emit another meta event with a different session_id (should be updated)
	fs.Emit(agent.Event{Type: agent.EventMeta, ExternalID: "claude-session-def456", Seq: 2})

	// Give the pump time to process
	time.Sleep(100 * time.Millisecond)

	// Verify the external_id was updated
	se, ok, _ = mgr.store.GetSession(ctx, hid)
	if !ok {
		t.Fatal("session not found after second meta event")
	}
	if se.ExternalID != "claude-session-def456" {
		t.Fatalf("external_id not updated: got %q, want %q", se.ExternalID, "claude-session-def456")
	}

	// Verify meta events are NOT broadcast to subscribers (internal bookkeeping)
	sub, err := mgr.Subscribe(hid)
	if err != nil {
		t.Fatal(err)
	}
	// Drain any events in the subscription
	for {
		select {
		case ev := <-sub:
			// If we receive any event, it should NOT be EventMeta
			if ev.Type == agent.EventMeta {
				t.Error("meta events should not be broadcast to subscribers")
			}
		default:
			goto done
		}
	}
done:
	// Test passes
}
