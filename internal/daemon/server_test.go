package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/api"
	"github.com/3toInf/hetu/internal/project"
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

func TestServerListProjects(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	st.UpsertProject(ctx, "Alpha", "/x/alpha")
	mgr := NewSessionManager(st, project.NewResolver(st), nil)
	srv := NewServer(st, mgr, nil)
	sock := tempPath(t) + ".sock"
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
