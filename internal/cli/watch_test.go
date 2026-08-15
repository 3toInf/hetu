package cli

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
)

// syncBuffer is a goroutine-safe bytes.Buffer: runWatch writes from its own
// goroutine while the test polls String().
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitSubscribed probe-emits text until the watcher renders it. Subscription
// is forward-only: an event emitted before subscribing is missed, so tests
// must confirm the subscriber is live before asserting on later events.
func waitSubscribed(t *testing.T, fs *fake.Session, out *syncBuffer, probe string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(out.String(), probe) {
		if time.Now().After(deadline) {
			t.Fatalf("watch never rendered probe %q, output: %q", probe, out.String())
		}
		fs.Emit(agent.Event{Type: agent.EventText, Text: probe})
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatchPrintsEvents(t *testing.T) {
	srv := startTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hid := srv.ensureDrivenSession(ctx)
	t.Setenv("HETU_SOCKET", srv.sock)

	fakeSess, ok := srv.Mgr.LiveSession(hid)
	if !ok {
		t.Fatal("failed to get live session")
	}
	fs, ok := fakeSess.(*fake.Session)
	if !ok {
		t.Fatal("live session is not a fake session")
	}

	out := &syncBuffer{}
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"watch", hid})
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetContext(ctx)

	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()

	waitSubscribed(t, fs, out, "step1")

	// Subscriber confirmed live: the status event is guaranteed delivered.
	fs.Emit(agent.Event{Type: agent.EventStatus, Status: "Completed"})

	// Wait for the status event to be rendered BEFORE cancelling — cancel
	// closes the watch stream and would drop the in-flight event.
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(out.String(), "Completed") {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("status event never rendered, output: %s", out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("watch command failed: %v, output: %s", err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "step1") {
		t.Fatalf("missing step1 in output: %s", s)
	}
	if !strings.Contains(s, "Completed") {
		t.Fatalf("missing status rendering in output: %s", s)
	}
}

func TestWatchInlineApprove(t *testing.T) {
	srv := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hid := srv.ensureDrivenSession(ctx)
	t.Setenv("HETU_SOCKET", srv.sock)

	fakeSess, ok := srv.Mgr.LiveSession(hid)
	if !ok {
		t.Fatal("failed to get live session")
	}
	fs, ok := fakeSess.(*fake.Session)
	if !ok {
		t.Fatal("live session is not a fake session")
	}

	stdin := strings.NewReader("y\n")
	out := &syncBuffer{}

	done := make(chan error, 1)
	go func() { done <- runWatch(ctx, srv.Client, hid, stdin, out) }()

	waitSubscribed(t, fs, out, "probe")

	// Subscriber confirmed live: the approval broadcast will reach it.
	resolved := make(chan struct{})
	go func() {
		defer close(resolved)
		_, _ = srv.Mgr.RequestApproval(ctx, hid, "tu1", "Bash", "{}")
	}()

	// Wait until the watcher answered (it prints "approved" after sending y).
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(out.String(), "approved") {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("watch never approved, output: %s", out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case <-resolved:
	case <-time.After(time.Second):
		t.Fatal("RequestApproval never resolved")
	}
	if pending := srv.Mgr.PendingApprovals(hid); len(pending) != 0 {
		t.Fatalf("expected no pending approvals after watch approve, got: %v", pending)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runWatch failed: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Bash") {
		t.Fatalf("expected output to contain 'Bash', got: %s", s)
	}
	if !strings.Contains(s, "approve") {
		t.Fatalf("expected output to contain 'approve', got: %s", s)
	}
}

func TestRenderEventApprovalStates(t *testing.T) {
	tests := []struct {
		name          string
		approvalState string
		wantContains  string
	}{
		{
			name:          "pending",
			approvalState: "pending",
			wantContains:  "[approval pending]",
		},
		{
			name:          "allowed",
			approvalState: "allowed",
			wantContains:  "[approval allowed]",
		},
		{
			name:          "denied",
			approvalState: "denied",
			wantContains:  "[approval denied]",
		},
		{
			name:          "timeout",
			approvalState: "timeout",
			wantContains:  "[approval timeout]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ev := agent.Event{
				Type:          agent.EventApproval,
				ApprovalState: tt.approvalState,
				ToolUseID:     "tu1",
				ToolName:      "Bash",
			}
			renderEvent(&buf, ev)
			got := buf.String()
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("renderEvent() output = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}
