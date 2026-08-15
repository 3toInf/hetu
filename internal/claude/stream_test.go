package claude

import (
	"encoding/json"
	"testing"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

func TestParseStreamLine(t *testing.T) {
	cases := []struct {
		name          string
		in            string
		wantEventType agent.EventType
		wantStatus    session.Status
		wantSessionID string
	}{
		{"assistant text", `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`, agent.EventText, "", ""},
		{"user tool_result", `{"type":"user","message":{"content":[{"type":"tool_result"}]}}`, agent.EventTool, "", ""},
		{"assistant tool_use", `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`, agent.EventTool, "", ""},
		{"result success", `{"type":"result","subtype":"success"}`, agent.EventStatus, session.StatusCompleted, ""},
		{"result error", `{"type":"result","subtype":"error"}`, agent.EventStatus, session.StatusError, ""},
		{"system init with session_id", `{"type":"system","subtype":"init","session_id":"claude-session-123"}`, agent.EventStatus, session.StatusRunning, "claude-session-123"},
		{"result with session_id", `{"type":"result","subtype":"success","session_id":"claude-session-456"}`, agent.EventStatus, session.StatusCompleted, "claude-session-456"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ps, ok := ParseStreamLine([]byte(c.in))
			if !ok {
				t.Fatalf("not recognized")
			}
			if ps.Event.Type != c.wantEventType {
				t.Errorf("type %v want %v", ps.Event.Type, c.wantEventType)
			}
			if c.wantStatus != "" && ps.Event.Status != c.wantStatus {
				t.Errorf("status %v want %v", ps.Event.Status, c.wantStatus)
			}
			if ps.SessionID != c.wantSessionID {
				t.Errorf("session_id %q want %q", ps.SessionID, c.wantSessionID)
			}
		})
	}
	_ = json.Marshal // keep import used if extended
}
