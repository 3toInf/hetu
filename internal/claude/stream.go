package claude

import (
	"encoding/json"

	"github.com/3toInf/hetu/internal/agent"
	"github.com/3toInf/hetu/internal/session"
)

type rawStream struct {
	Type     string                 `json:"type"`
	Subtype  string                 `json:"subtype"`
	Message  map[string]interface{} `json:"message"`
}

// ParseStreamLine converts one stream-json line into an Event.
// Returns ok=false for lines we don't model (they are ignored by the driver).
func ParseStreamLine(b []byte) (agent.Event, bool) {
	var r rawStream
	if err := json.Unmarshal(b, &r); err != nil {
		return agent.Event{}, false
	}
	switch r.Type {
	case "assistant":
		return assistantEvent(r.Message), true
	case "user":
		if hasToolResult(r.Message) {
			return agent.Event{Type: agent.EventTool}, true
		}
		return agent.Event{}, false
	case "result":
		st := session.StatusCompleted
		if r.Subtype == "error" || r.Subtype == "error_during_execution" {
			st = session.StatusError
		}
		return agent.Event{Type: agent.EventStatus, Status: st}, true
	case "system", "stream_event":
		// progress/heartbeat: ignored
		return agent.Event{}, false
	}
	return agent.Event{}, false
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
