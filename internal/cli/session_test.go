package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/3toInf/hetu/internal/api"
)

func TestPrintSessionDetailsWithMessages(t *testing.T) {
	var b bytes.Buffer
	s := api.SessionDTO{HetuID: "abc", Agent: "claude", Title: "T", Status: "Completed"}
	// Messages arrive newest-first from the server (RecentMessages ORDER BY seq DESC).
	msgs := []api.MessageDTO{
		{Seq: 3, Role: "assistant", Content: "latest"},
		{Seq: 2, Role: "user", Content: "middle"},
		{Seq: 1, Role: "assistant", Content: "old"},
	}
	printSessionDetailsWithMessages(&b, s, msgs, 2)
	out := b.String()
	if !strings.Contains(out, "Messages") {
		t.Fatalf("missing header: %q", out)
	}
	// truncation: limit 2 keeps the two NEWEST (seq 3, 2), dropping seq 1
	if strings.Contains(out, "old") {
		t.Fatalf("limit not honored (oldest shown): %q", out)
	}
	// chronological: newest-first input renders oldest-first
	latest := strings.Index(out, "assistant: latest")
	middle := strings.Index(out, "user: middle")
	if latest < 0 || middle < 0 {
		t.Fatalf("transcript not rendered: %q", out)
	}
	if !(middle < latest) {
		t.Fatalf("not chronological (oldest-first): %q", out)
	}

	// limit 0 renders everything, still oldest-first
	var b2 bytes.Buffer
	printSessionDetailsWithMessages(&b2, s, msgs, 0)
	out2 := b2.String()
	if !strings.Contains(out2, "old") {
		t.Fatalf("limit 0 should render all: %q", out2)
	}
	if strings.Index(out2, "assistant: old") >= strings.Index(out2, "assistant: latest") {
		t.Fatalf("limit 0 not chronological: %q", out2)
	}
}

func TestPrintSearchResults(t *testing.T) {
	var b bytes.Buffer
	res := []api.SearchResultDTO{{Agent: "claude", Title: "T", Hits: 3, Snippet: "… [websocket] …"}}
	printSearchResults(&b, res)
	out := b.String()
	if !strings.Contains(out, "3 hits") || !strings.Contains(out, "websocket") {
		t.Fatalf("search result not rendered: %q", out)
	}
}
