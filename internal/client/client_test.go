package client

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
	"github.com/3toInf/hetu/internal/api"
	"github.com/3toInf/hetu/internal/daemon"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

func tempPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "hetu")
}

func TestClientListProjects(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	st.UpsertProject(ctx, "Alpha", "/x")
	mgr := daemon.NewSessionManager(st, project.NewResolver(st), nil)
	srv := daemon.NewServer(st, mgr, nil)
	sock := tempPath(t) + ".sock"
	go srv.Serve(ctx, sock)
	t.Cleanup(func() { srv.Shutdown(ctx) })

	// Wait for socket to be available
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	c := New(sock) // NewWithAutostart disabled in test
	c.NoAutostart = true
	ps, err := c.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "Alpha" {
		t.Fatalf("got %+v", ps)
	}
}

// testServer wraps a real server, session manager, and client for integration testing
type testServer struct {
	Mgr    *daemon.SessionManager
	Client *Client
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
	t.Cleanup(func() { srv.Shutdown(ctx); cancel() })

	// Wait for socket to be available
	for i := 0; i < 50; i++ {
		if _, err := net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	c := New(sock)
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

// seedUnreadSession creates a session with unread=true for testing mark_read
func (ts *testServer) seedUnreadSession(ctx context.Context) string {
	// Create a session with unread=true
	hid, err := ts.st.UpsertSession(ctx, store.Session{
		HetuID: "unread-session",
		Agent:  "claude",
		CWD:    "/x",
		Status: session.StatusRunning,
		Unread: true,
		Driven: true,
	})
	if err != nil {
		panic(err)
	}
	return hid.HetuID
}

func TestApproveResolvesPending(t *testing.T) {
	srv := startTestServer(t)
	ctx := context.Background()
	hid := srv.ensureDrivenSession(ctx)

	// arm a pending approval
	go func() { _, _ = srv.Mgr.RequestApproval(context.Background(), hid, "tu", "Bash", "{}") }()
	srv.waitForPending(t, hid, "tu")

	if err := srv.Client.Approve(ctx, hid, "tu", true, ""); err != nil {
		t.Fatal(err)
	}

	// approval now resolved (RequestApproval goroutine returned allow) — assert via no leftover pending
	if left := srv.Mgr.PendingApprovals(hid); len(left) != 0 {
		t.Fatalf("pending left: %v", left)
	}
}

func TestMarkReadClearsUnread(t *testing.T) {
	srv := startTestServer(t)
	hid := srv.seedUnreadSession(context.Background())

	if err := srv.Client.MarkRead(context.Background(), hid); err != nil {
		t.Fatal(err)
	}

	// assert via get_session DTO Unread=false
	sessions, err := srv.Client.ListSessions(context.Background(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	var found *api.SessionDTO
	for _, s := range sessions {
		if s.HetuID == hid {
			found = &s
			break
		}
	}
	if found == nil {
		t.Fatal("session not found")
	}
	if found.Unread {
		t.Errorf("expected Unread=false after MarkRead, got true")
	}
}