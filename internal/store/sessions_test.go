package store

import (
	"context"
	"testing"

	"github.com/3toInf/hetu/internal/session"
)

func TestUpsertSessionIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a, _ := s.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "ext1", CWD: "/x", Title: "T", Status: session.StatusRunning})
	b, _ := s.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "ext1", CWD: "/x", Title: "T2", Status: session.StatusCompleted})
	if a.HetuID != b.HetuID {
		t.Fatalf("upsert should reuse hetu_id")
	}
	got, ok, _ := s.GetSession(ctx, "h1")
	if !ok || got.Title != "T2" || got.Status != session.StatusCompleted {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestListSessionsByProject(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	p, _ := s.UpsertProject(ctx, "P", "/p")
	_, _ = s.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "e1", ProjectID: p.ID, Status: session.StatusCompleted})
	_, _ = s.UpsertSession(ctx, Session{HetuID: "h2", Agent: "claude", ExternalID: "e2", ProjectID: p.ID, Status: session.StatusError})
	got, err := s.ListSessions(ctx, ListFilter{ProjectID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	// Error should rank before Completed (priority ordering)
	if got[0].Status != session.StatusError {
		t.Fatalf("expected Error first, got %v", got[0].Status)
	}
}
