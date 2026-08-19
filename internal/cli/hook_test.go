package cli

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
	"github.com/3toInf/hetu/internal/client"
	"github.com/3toInf/hetu/internal/daemon"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

func tempPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir + "/hetu"
}

// testServer wraps a real server, session manager, and client for integration testing
type testServer struct {
	Mgr    *daemon.SessionManager
	Client *client.Client
	sock   string
	ctx    context.Context
	cancel context.CancelFunc
	st     *store.Store // Exposed for test setup
}

// startTestServer creates a real Server + SessionManager with a fake agent on a temp socket
func startTestServer(t *testing.T) *testServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })

	// Create a fake agent and session
	fs := fake.NewSession("h1", "ext1", session.StatusRunning)
	fa := &fake.Agent{N: "claude", Drv: &fake.Driver{OnStart: func(_ context.Context, _ agent.StartRequest) (agent.Session, error) {
		return fs, nil
	}}}

	mgr := daemon.NewSessionManager(st, project.NewResolver(st), map[string]agent.Agent{"claude": fa})
	srv := daemon.NewServer(st, mgr, nil)

	sock := tempPath(t) + ".sock"
	go srv.Serve(ctx, sock)

	// Wait for server to be ready to avoid race during cleanup
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Additional wait to ensure Server.ln is fully assigned (avoid race in cleanup)
	time.Sleep(50 * time.Millisecond)

	t.Cleanup(func() { srv.Shutdown(ctx); cancel() })

	c := client.New(sock)
	c.NoAutostart = true

	return &testServer{Mgr: mgr, Client: c, sock: sock, ctx: ctx, cancel: cancel, st: st}
}

// ensureDrivenSession creates a live fake-driven session and returns its hetuID
func (ts *testServer) ensureDrivenSession(ctx context.Context) string {
	hid, err := ts.Mgr.Ensure(ctx, "claude", "ext1", "/x")
	if err != nil {
		panic(err) // test helper
	}
	return hid
}

// waitForPending waits for a pending approval to appear for the given toolUseID
func (ts *testServer) waitForPending(t *testing.T, hetuID, toolUseID string) {
	t.Helper()
	sub, err := ts.Mgr.Subscribe(hetuID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		go func() {
			for range sub {
			}
		}()
	}()

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

func TestHookClientRoundTrip(t *testing.T) {
	srv := startTestServer(t)
	ctx := context.Background()
	hid := srv.ensureDrivenSession(ctx)

	// Create hook input JSON
	hookInput := `{"session_id":"ext1","tool_name":"Bash","tool_input":{"command":"ls"},"tool_use_id":"tu9"}`

	// Start the hook client in a goroutine
	var out bytes.Buffer
	in := strings.NewReader(hookInput)
	errChan := make(chan error, 1)

	go func() {
		errChan <- runHookPermission(ctx, srv.sock, in, &out)
	}()

	// Wait for pending approval to appear
	srv.waitForPending(t, hid, "tu9")

	// Resolve the approval
	srv.Mgr.ResolveApproval(hid, "tu9", daemon.ApprovalDecision{Allow: true, Reason: "test approved"})

	// Wait for hook client to complete
	if err := <-errChan; err != nil {
		t.Fatalf("hook client failed: %v", err)
	}

	// Assert the output contains the decision JSON
	output := out.String()
	if !strings.Contains(output, `"permissionDecision":"allow"`) {
		t.Fatalf("expected decision JSON with allow, got: %s", output)
	}
	if !strings.Contains(output, `"permissionDecisionReason":"test approved"`) {
		t.Fatalf("expected decision JSON with reason, got: %s", output)
	}
}

func TestReadOnlyShortCircuit(t *testing.T) {
	srv := startTestServer(t)
	ctx := context.Background()
	hid := srv.ensureDrivenSession(ctx)

	// Create hook input JSON for Read tool (read-only should short-circuit)
	hookInput := `{"session_id":"ext1","tool_name":"Read","tool_input":{"file_path":"/tmp/test"},"tool_use_id":"tu10"}`

	var out bytes.Buffer
	in := strings.NewReader(hookInput)

	if err := runHookPermission(ctx, srv.sock, in, &out); err != nil {
		t.Fatalf("hook client failed: %v", err)
	}

	// Should return immediately with allow (no pending approval)
	output := out.String()
	if !strings.Contains(output, `"permissionDecision":"allow"`) {
		t.Fatalf("expected immediate allow for Read tool, got: %s", output)
	}

	// Assert no pending approval was created
	if pending := srv.Mgr.PendingApprovals(hid); len(pending) != 0 {
		t.Fatalf("expected no pending approvals for read-only tool, got: %v", pending)
	}
}

func TestHookDaemonUnreachable(t *testing.T) {
	ctx := context.Background()

	// Create hook input JSON
	hookInput := `{"session_id":"ext1","tool_name":"Bash","tool_input":{"command":"ls"},"tool_use_id":"tu9"}`

	var out bytes.Buffer
	in := strings.NewReader(hookInput)

	// Use a bogus socket path (daemon unreachable)
	bogusSocket := t.TempDir() + "/bogus.sock"

	if err := runHookPermission(ctx, bogusSocket, in, &out); err != nil {
		t.Fatalf("hook client should return nil on daemon unreachable, got: %v", err)
	}

	// Assert stdout is empty (silent hook)
	if output := out.String(); output != "" {
		t.Fatalf("expected empty output on daemon unreachable, got: %s", output)
	}
}
