package project

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/3toInf/hetu/internal/store"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "t.sqlite")
}

func TestResolveCreatesAndReuses(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(ctx, tempPath(t))
	defer st.Close()
	r := NewResolver(st)
	p1, err := r.ResolveByCWD(ctx, "/home/u/code/alpha")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := r.ResolveByCWD(ctx, "/home/u/code/alpha")
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID != p2.ID {
		t.Fatal("same path should resolve to same project")
	}
	if p1.Name != "alpha" {
		t.Fatalf("name should default to base dir 'alpha', got %q", p1.Name)
	}
}
