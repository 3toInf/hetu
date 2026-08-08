package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrateCreatesTables(t *testing.T) {
	// Save the path so we can re-open the exact same database
	dbPath := filepath.Join(t.TempDir(), "t.sqlite")

	s, err := Open(context.Background(), dbPath)
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

	// Re-open the same database to prove migrations are idempotent
	s, err = Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Re-open: %v", err)
	}
	defer s.Close()

	// Verify the 4 tables still exist after re-opening
	var n2 int
	err = s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('projects','agents','sessions','session_events')`).Scan(&n2)
	if err != nil {
		t.Fatalf("re-open query: %v", err)
	}
	if n2 != 4 {
		t.Fatalf("expected 4 tables after re-open, got %d", n2)
	}
}