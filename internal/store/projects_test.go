package store

import (
	"context"
	"testing"
)

func TestUpsertProjectIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	p1, err := s.UpsertProject(ctx, "Alpha", "/x/alpha")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.UpsertProject(ctx, "Alpha2", "/x/alpha") // same path -> reuse, rename
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID != p2.ID {
		t.Fatalf("expected same id, got %d and %d", p1.ID, p2.ID)
	}
	got, ok, err := s.GetProjectByPath(ctx, "/x/alpha")
	if err != nil || !ok {
		t.Fatalf("get: %v %v", got, err)
	}
	if got.Name != "Alpha2" {
		t.Fatalf("expected rename to Alpha2, got %q", got.Name)
	}
	ps, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("expected 1 project, got %d", len(ps))
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepathInTemp(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
