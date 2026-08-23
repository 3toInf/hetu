package store

import (
	"context"
	"strings"
	"testing"

	"github.com/3toInf/hetu/internal/session"
)

func TestSyncMessagesReplace(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "e1", Status: session.StatusCompleted})

	if err := st.SyncMessages(ctx, "h1", []Message{{Seq: 0, Role: "user", Content: "hello"}, {Seq: 1, Role: "assistant", Content: "hi there"}}); err != nil {
		t.Fatal(err)
	}
	// replace with a DIFFERENT list (simulates JSONL rewrite where old seqs vanish)
	if err := st.SyncMessages(ctx, "h1", []Message{{Seq: 0, Role: "user", Content: "rewritten"}}); err != nil {
		t.Fatal(err)
	}
	ms, _ := st.RecentMessages(ctx, "h1", 0)
	if len(ms) != 1 || ms[0].Content != "rewritten" {
		t.Fatalf("replace semantics broken: %+v", ms)
	}
	// FTS index must also reflect the replacement
	var n int
	st.db.QueryRowContext(ctx, `SELECT count(*) FROM session_messages_fts WHERE session_messages_fts MATCH '"rewritten"'`).Scan(&n)
	if n != 1 {
		t.Fatalf("FTS not replaced: %d rows", n)
	}
}

func TestRecentMessagesOrderAndLimit(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "e1", Status: session.StatusCompleted})
	st.SyncMessages(ctx, "h1", []Message{{Seq: 0, Role: "user", Content: "a"}, {Seq: 1, Role: "assistant", Content: "b"}, {Seq: 2, Role: "user", Content: "c"}})
	ms, _ := st.RecentMessages(ctx, "h1", 2)
	if len(ms) != 2 || ms[0].Seq != 2 || ms[1].Seq != 1 {
		t.Fatalf("expected newest-first limit 2, got %+v", ms)
	}
}

func TestContentSyncedMarkers(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "e1", Status: session.StatusCompleted})
	if _, ok, _ := st.ContentSyncedAt(ctx, "h1"); ok {
		t.Fatal("expected not-synced before SetContentSynced")
	}
	if err := st.SetContentSynced(ctx, "h1", 999); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := st.ContentSyncedAt(ctx, "h1"); !ok || v != 999 {
		t.Fatalf("expected 999, got %d %v", v, ok)
	}
}

func TestSearchMessagesPhrase(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "e1", Status: session.StatusCompleted})
	st.UpsertSession(ctx, Session{HetuID: "h2", Agent: "claude", ExternalID: "e2", Status: session.StatusCompleted})
	st.SyncMessages(ctx, "h1", []Message{{Seq: 0, Role: "user", Content: "websocket keeps dropping"}, {Seq: 1, Role: "assistant", Content: "check the websocket handshake logs"}})
	st.SyncMessages(ctx, "h2", []Message{{Seq: 0, Role: "user", Content: "unrelated"}})

	hits, err := st.SearchMessages(ctx, "websocket", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].HetuID != "h1" || hits[0].Hits != 2 {
		t.Fatalf("expected 1 session with 2 hits, got %+v", hits)
	}
	if !strings.Contains(hits[0].Snippet, "websocket") {
		t.Fatalf("snippet missing match: %q", hits[0].Snippet)
	}
}

func TestSearchMessagesTreatsOperatorLiterally(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertSession(ctx, Session{HetuID: "h1", Agent: "claude", ExternalID: "e1", Status: session.StatusCompleted})
	st.SyncMessages(ctx, "h1", []Message{{Seq: 0, Role: "user", Content: "should we use redis OR postgres?"}})
	// "redis OR postgres" as a phrase must NOT be parsed as an FTS5 OR operator
	hits, err := st.SearchMessages(ctx, "redis OR postgres", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("phrase with 'OR' should match literally, got %+v", hits)
	}
}
