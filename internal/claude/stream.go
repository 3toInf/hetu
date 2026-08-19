package claude

import (
	"encoding/json"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

type rawStream struct {
	Type      string                 `json:"type"`
	Subtype   string                 `json:"subtype"`
	Message   map[string]interface{} `json:"message"`
	SessionID string                 `json:"session_id,omitempty"`
}

// ParsedStream is the result of ParseStreamLine, potentially including a session_id.
type ParsedStream struct {
	Event     agent.Event
	SessionID string
}

// ParseStreamLine converts one stream-json line into an Event.
// Returns ok=false for lines we don't model (they are ignored by the driver).
// When a system/init or result message carries a session_id, it's returned in SessionID.
func ParseStreamLine(b []byte) (ParsedStream, bool) {
	var r rawStream
	if err := json.Unmarshal(b, &r); err != nil {
		return ParsedStream{}, false
	}
	var ev agent.Event
	var sessionID string
	switch r.Type {
	case "assistant":
		ev = assistantEvent(r.Message)
		return ParsedStream{Event: ev}, true
	case "user":
		if hasToolResult(r.Message) {
			return ParsedStream{Event: agent.Event{Type: agent.EventTool}}, true
		}
		return ParsedStream{}, false
	case "result":
		st := session.StatusCompleted
		if r.Subtype == "error" || r.Subtype == "error_during_execution" {
			st = session.StatusError
		}
		ev = agent.Event{Type: agent.EventStatus, Status: st}
		sessionID = r.SessionID // result messages can carry session_id
		return ParsedStream{Event: ev, SessionID: sessionID}, true
	case "system":
		if r.Subtype == "init" {
			sessionID = r.SessionID // system/init messages carry session_id
			return ParsedStream{Event: agent.Event{Type: agent.EventStatus, Status: session.StatusRunning}, SessionID: sessionID}, true
		}
		// other system messages: ignored
		return ParsedStream{}, false
	case "stream_event":
		// progress/heartbeat: ignored
		return ParsedStream{}, false
	}
	return ParsedStream{}, false
}

func assistantEvent(msg map[string]interface{}) agent.Event {
	ev := agent.Event{Type: agent.EventText}
	content, _ := msg["content"].([]interface{})
	for _, item := range content {
		m, _ := item.(map[string]interface{})
		switch m["type"] {
		case "text":
			if t, ok := m["text"].(string); ok {
				ev.Text += t
			}
		case "tool_use":
			ev.Type = agent.EventTool
			if n, ok := m["name"].(string); ok {
				ev.ToolName = n
			}
			if inp, err := json.Marshal(m["input"]); err == nil {
				ev.ToolJSON = string(inp)
			}
		}
	}
	if ev.Type == agent.EventText && ev.Text == "" {
		// assistant message with no modelable content: ignore
	}
	return ev
}

func hasToolResult(msg map[string]interface{}) bool {
	content, _ := msg["content"].([]interface{})
	for _, item := range content {
		if m, ok := item.(map[string]interface{}); ok {
			if m["type"] == "tool_result" {
				return true
			}
		}
	}
	return false
}
