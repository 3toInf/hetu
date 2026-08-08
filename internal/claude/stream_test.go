package claude

import (
	"encoding/json"
	"testing"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

func TestParseStreamLine(t *testing.T) {
	cases := []struct {
		in     string
		want   agent.EventType
		status session.Status
	}{
		{`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`, agent.EventText, ""},
		{`{"type":"user","message":{"content":[{"type":"tool_result"}]}}`, agent.EventTool, ""},
		{`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`, agent.EventTool, ""},
		{`{"type":"result","subtype":"success"}`, agent.EventStatus, session.StatusCompleted},
		{`{"type":"result","subtype":"error"}`, agent.EventStatus, session.StatusError},
	}
	for i, c := range cases {
		ev, ok := ParseStreamLine([]byte(c.in))
		if !ok {
			t.Fatalf("case %d: not recognized", i)
		}
		if ev.Type != c.want {
			t.Errorf("case %d: type %v want %v", i, ev.Type, c.want)
		}
		if c.status != "" && ev.Status != c.status {
			t.Errorf("case %d: status %v want %v", i, ev.Status, c.status)
		}
	}
	_ = json.Marshal // keep import used if extended
}
