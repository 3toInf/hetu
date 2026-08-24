package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/api"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

func dial(t *testing.T, sock string) net.Conn {
	t.Helper()
	var c net.Conn
	var err error
	for i := 0; i < 50; i++ {
		c, err = net.Dial("unix", sock)
		if err == nil {
			return c
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("dial: %v", err)
	return nil
}

// TestShutdownConcurrent guards against double-closing s.done: the ctx-watcher
// goroutine in Serve and main.go's post-select Shutdown can both call Shutdown
// at once. The select{case <-s.done: default: close(s.done)} guard is not
// atomic across callers, so concurrent Shutdown used to panic. Run it in a loop
// so the race detector has ample opportunity to catch the old code.
func TestShutdownConcurrent(t *testing.T) {
	for i := 0; i < 200; i++ {
		srv := NewServer(nil, nil, nil)
		const goroutines = 8
		start := make(chan struct{})
		panics := make(chan any, goroutines)
		var wg sync.WaitGroup
		for j := 0; j < goroutines; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panics <- r
					}
				}()
				<-start
				srv.Shutdown(context.Background())
			}()
		}
		close(start)
		wg.Wait()
		close(panics)
		for r := range panics {
			t.Fatalf("iteration %d: Shutdown panicked: %v", i, r)
		}
	}
}

func TestServerListProjects(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	st.UpsertProject(ctx, "Alpha", "/x/alpha")
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	sock := sockPath(t)
	go srv.Serve(ctx, sock)
	t.Cleanup(func() { srv.Shutdown(ctx) })

	c := dial(t, sock)
	defer c.Close()
	api.Encode(c, api.Request{Op: "list_projects", Body: json.RawMessage(`{}`)})
	br := bufio.NewReader(c)
	line, _ := br.ReadBytes('\n')
	var resp api.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("not ok: %s", resp.Err)
	}
}

func TestNeedsAttention(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	sock := sockPath(t)
	go srv.Serve(ctx, sock)
	t.Cleanup(func() { srv.Shutdown(ctx) })

	// Test cases: status -> expected NeedsAttention value
	testCases := []struct {
		status           session.Status
		needsAttention   bool
		description      string
	}{
		{session.StatusWaitingForApproval, true, "WaitingForApproval should have NeedsAttention=true"},
		{session.StatusWaitingForInput, true, "WaitingForInput should have NeedsAttention=true"},
		{session.StatusError, true, "Error should have NeedsAttention=true"},
		{session.StatusCompleted, false, "Completed should have NeedsAttention=false"},
		{session.StatusRunning, false, "Running should have NeedsAttention=false"},
		{session.StatusIdle, false, "Idle should have NeedsAttention=false"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Create a session with the test status
			hid, err := st.UpsertSession(ctx, store.Session{
				HetuID: "test-" + string(tc.status),
				Agent:  "claude",
				CWD:    "/x",
				Status: tc.status,
				Driven: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			// Get the session DTO via list_sessions
			c := dial(t, sock)
			defer c.Close()
			api.Encode(c, api.Request{Op: "list_sessions", Body: json.RawMessage(`{}`)})
			br := bufio.NewReader(c)
			line, _ := br.ReadBytes('\n')
			var resp api.Response
			if err := json.Unmarshal(line, &resp); err != nil {
				t.Fatal(err)
			}
			if !resp.OK {
				t.Fatalf("not ok: %s", resp.Err)
			}

			// Parse the sessions array from the response
			var sessions []api.SessionDTO
			if err := json.Unmarshal(resp.Body, &sessions); err != nil {
				t.Fatal(err)
			}

			// Find our test session
			var found *api.SessionDTO
			for _, s := range sessions {
				if s.HetuID == hid.HetuID {
					found = &s
					break
				}
			}
			if found == nil {
				t.Fatal("test session not found in list_sessions response")
			}

			// Assert NeedsAttention matches expected value
			if found.NeedsAttention != tc.needsAttention {
				t.Errorf("NeedsAttention=%v for status %s, expected %v", found.NeedsAttention, tc.status, tc.needsAttention)
			}
		})
	}
}

func TestListSessionsUnknownProjectPath(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	st.UpsertProject(ctx, "Alpha", "/x/alpha")
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	sock := sockPath(t)
	go srv.Serve(ctx, sock)
	t.Cleanup(func() { srv.Shutdown(ctx) })

	c := dial(t, sock)
	defer c.Close()
	api.Encode(c, api.Request{Op: "list_sessions", Body: json.RawMessage(`{"project_path":"/typo/path"}`)})
	br := bufio.NewReader(c)
	line, _ := br.ReadBytes('\n')
	var resp api.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK {
		t.Fatalf("expected error for unknown project path, got OK (body: %s)", resp.Body)
	}
	if !strings.Contains(resp.Err, "unknown project path") {
		t.Fatalf("error should name the problem, got: %s", resp.Err)
	}
}
