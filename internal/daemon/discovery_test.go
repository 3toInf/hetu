package daemon

import (
	"context"
	"testing"

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
