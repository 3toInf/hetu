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

func TestResolveSession(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	full := "abcdef1234567890"
	_, _ = s.UpsertSession(ctx, Session{HetuID: full, Agent: "claude", ExternalID: "ext", CWD: "/x", Status: session.StatusRunning})

	// exact id
	if se, ok, err := s.ResolveSession(ctx, full); !ok || err != nil || se.HetuID != full {
		t.Fatalf("exact match: ok=%v err=%v id=%q", ok, err, se.HetuID)
	}
	// unique prefix (the truncated id shown in the sessions table)
	if se, ok, err := s.ResolveSession(ctx, "abcdef12"); !ok || err != nil || se.HetuID != full {
		t.Fatalf("prefix match: ok=%v err=%v id=%q", ok, err, se.HetuID)
	}
	// no match
	if _, ok, err := s.ResolveSession(ctx, "deadbeef"); ok || err != nil {
		t.Fatalf("expected no match, got ok=%v err=%v", ok, err)
	}
}

func TestResolveSessionAmbiguous(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	_, _ = s.UpsertSession(ctx, Session{HetuID: "aaa111", Agent: "claude", ExternalID: "e1", CWD: "/x", Status: session.StatusRunning})
	_, _ = s.UpsertSession(ctx, Session{HetuID: "aaa222", Agent: "claude", ExternalID: "e2", CWD: "/x", Status: session.StatusRunning})

	if _, ok, err := s.ResolveSession(ctx, "aaa"); ok || err != ErrAmbiguousID {
		t.Fatalf("expected ambiguity error, got ok=%v err=%v", ok, err)
	}
	// a longer prefix disambiguates
	if _, ok, err := s.ResolveSession(ctx, "aaa1"); !ok || err != nil {
		t.Fatalf("expected unique match, got ok=%v err=%v", ok, err)
	}
}
