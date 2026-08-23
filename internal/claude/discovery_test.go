package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/3toInf/hetu/internal/agent"
)

func TestDiscoverYieldsSessions(t *testing.T) {
	d := &Discovery{ProjectsDir: "testdata"}
	ch, err := d.Discover(context.Background(), DiscoverOptsAll)
	if err != nil {
		t.Fatal(err)
	}
	var got []agent.DiscoveredSession
	for s := range ch {
		got = append(got, s)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 session, got %d", len(got))
	}
	s := got[0]
	if s.ExternalID != "a1" {
		t.Errorf("ExternalID=%q want a1", s.ExternalID)
	}
	if s.CWD != "/home/u/code/alpha" {
		t.Errorf("CWD=%q want /home/u/code/alpha", s.CWD)
	}
	if s.Title != "Refactor auth" {
		t.Errorf("Title=%q want 'Refactor auth'", s.Title)
	}
	if s.MessageCount != 2 {
		t.Errorf("MessageCount=%d want 2", s.MessageCount)
	}
}

// writeJSONL writes a JSONL fixture, creating the parent slug dir so Discovery
// can walk it (the dir name encodes the cwd, e.g. "x" -> "/x").
func writeJSONL(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseTranscriptExtractsMessages(t *testing.T) {
	dir := t.TempDir()
	// two text blocks in one user message, a tool_use that must be skipped, an assistant text
	writeJSONL(t, filepath.Join(dir, "x", "abc.jsonl"),
		`{"type":"user","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}}`+"\n"+
			`{"type":"assistant","timestamp":"2026-01-01T00:00:01Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`+"\n"+
			`{"type":"assistant","timestamp":"2026-01-01T00:00:02Z","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`+"\n")
	d := &Discovery{ProjectsDir: filepath.Join(dir, "x")}
	ch, err := d.Discover(context.Background(), agent.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var got agent.DiscoveredSession
	for s := range ch {
		got = s
	}
	if len(got.Messages) != 2 {
		t.Fatalf("expected 2 text messages, got %d: %+v", len(got.Messages), got.Messages)
	}
	// first user message: two text blocks joined by \n; seq 0
	if got.Messages[0].Role != "user" || got.Messages[0].Content != "first\nsecond" || got.Messages[0].Seq != 0 {
		t.Fatalf("bad first message: %+v", got.Messages[0])
	}
	// the tool_use-only assistant line is skipped; the text assistant is seq 2
	if got.Messages[1].Role != "assistant" || got.Messages[1].Content != "done" || got.Messages[1].Seq != 2 {
		t.Fatalf("bad second message: %+v", got.Messages[1])
	}
}
