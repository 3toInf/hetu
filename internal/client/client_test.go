package client

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/daemon"
	"github.com/3toInf/hetu/internal/project"
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