package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
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

// waitForApprovalPending waits until an approval pending event is seen for the given toolUseID
func waitForApprovalPending(t *testing.T, m *SessionManager, hetuID, toolUseID string) {
	t.Helper()
	sub, err := m.Subscribe(hetuID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		go func() {
			for range sub {
			}
		}()
	}() // drain to avoid goroutine leak

	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-sub:
			if ev.Type == agent.EventApproval && ev.ApprovalState == "pending" && ev.ToolUseID == toolUseID {
				return
			}
		case <-deadline:
			t.Fatalf("timeout waiting for approval pending event for tool %s", toolUseID)
		}
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
