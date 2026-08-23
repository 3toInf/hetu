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
	msgs := []api.MessageDTO{
		{Seq: 0, Role: "user", Content: "hello"},
		{Seq: 1, Role: "assistant", Content: "hi"},
	}
	printSessionDetailsWithMessages(&b, s, msgs, 2)
	out := b.String()
	if !strings.Contains(out, "user: hello") || !strings.Contains(out, "assistant: hi") {
		t.Fatalf("transcript not rendered: %q", out)
	}
	if !strings.Contains(out, "Messages") {
		t.Fatalf("missing header: %q", out)
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
