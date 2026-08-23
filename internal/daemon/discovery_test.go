package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/agent/fake"
	"github.com/3toInf/hetu/internal/project"
	"github.com/3toInf/hetu/internal/session"
	"github.com/3toInf/hetu/internal/store"
)

func TestDiscoverySchedulerUpserts(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	r := project.NewResolver(st)
	disc := &fake.DiscoverySource{Sessions: []agent.DiscoveredSession{
		{Agent: "claude", ExternalID: "e1", CWD: "/x", Title: "T1", Status: session.StatusCompleted, MessageCount: 2},
	}}
	sch := NewDiscoveryScheduler(st, r, map[string]agent.DiscoverySource{"claude": disc})
	if err := sch.Run(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := st.ListSessions(ctx, store.ListFilter{})
	if len(got) != 1 || got[0].Title != "T1" || got[0].CWD != "/x" {
		t.Fatalf("unexpected: %+v", got)
	}
	// project auto-created
	ps, _ := st.ListProjects(ctx)
	if len(ps) != 1 || ps[0].Path != "/x" {
		t.Fatalf("project not created: %+v", ps)
	}
}

func TestDiscoveryStableHetuIDAndSync(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	t.Cleanup(func() { st.Close() })
	r := project.NewResolver(st)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	disc := &fake.DiscoverySource{Sessions: []agent.DiscoveredSession{{
		Agent: "claude", ExternalID: "ext1", CWD: "/x", Status: session.StatusCompleted,
		Messages:        []agent.DiscoveredMessage{{Role: "user", Content: "hello", Seq: 0}},
		TranscriptMtime: fixed,
	}}}
	sch := NewDiscoveryScheduler(st, r, map[string]agent.DiscoverySource{"claude": disc})
	if err := sch.Run(ctx); err != nil {
		t.Fatal(err)
	}

	se, ok, _ := st.GetSessionByExternal(ctx, "claude", "ext1")
	if !ok {
		t.Fatal("session not stored")
	}
	id1 := se.HetuID
	ms, _ := st.RecentMessages(ctx, id1, 0)
	if len(ms) != 1 || ms[0].Content != "hello" {
		t.Fatalf("messages not synced: %+v", ms)
	}

	// run again with the SAME mtime → no message re-sync (id stays stable, content unchanged)
	if err := sch.Run(ctx); err != nil {
		t.Fatal(err)
	}
	se2, _, _ := st.GetSessionByExternal(ctx, "claude", "ext1")
	if se2.HetuID != id1 {
		t.Fatalf("hetu_id changed across discovers: %q -> %q", id1, se2.HetuID)
	}
	ms2, _ := st.RecentMessages(ctx, id1, 0)
	if len(ms2) != 1 {
		t.Fatalf("unexpected re-sync: %+v", ms2)
	}

	// newer mtime → re-sync replaces content
	disc.Sessions[0].TranscriptMtime = fixed.Add(time.Hour)
	disc.Sessions[0].Messages = []agent.DiscoveredMessage{{Role: "user", Content: "rewritten", Seq: 0}}
	if err := sch.Run(ctx); err != nil {
		t.Fatal(err)
	}
	ms3, _ := st.RecentMessages(ctx, id1, 0)
	if len(ms3) != 1 || ms3[0].Content != "rewritten" {
		t.Fatalf("re-sync failed: %+v", ms3)
	}
}
