package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrateCreatesTables(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	var n int
	err = s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('projects','agents','sessions','session_events')`).Scan(&n)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 4 {
		t.Fatalf("expected 4 tables, got %d", n)
	}

	// Re-open is idempotent (migrations re-run safely).
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}