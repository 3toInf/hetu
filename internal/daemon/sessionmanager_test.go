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
		_ = ev
	case <-time.After(time.Second):
		t.Fatal("did not receive event")
	}
}
