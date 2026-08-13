package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEventTextPayload(t *testing.T) {
	e := Event{Type: EventText, Text: "hello"}
	if e.Type.String() != "text" {
		t.Fatalf("got %q", e.Type.String())
	}
}

func TestApprovalEventSerializes(t *testing.T) {
	ev := Event{Type: EventApproval, ToolName: "Bash", ToolUseID: "toolu_1", ApprovalState: "pending"}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"type":"approval"`, `"tool_use_id":"toolu_1"`, `"approval_state":"pending"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("event JSON %q missing %q", s, want)
		}
	}
}
