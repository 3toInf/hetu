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
