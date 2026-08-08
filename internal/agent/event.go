package agent

import "github.com/3toInf/hetu/internal/session"

type EventType string

const (
	EventText   EventType = "text"
	EventTool   EventType = "tool"
	EventStatus EventType = "status"
	EventError  EventType = "error"
)

func (e EventType) String() string { return string(e) }

// Event is one observation from a driven session's event stream.
type Event struct {
	Type     EventType        `json:"type"`
	Text     string           `json:"text,omitempty"`      // EventText
	ToolName string           `json:"tool_name,omitempty"` // EventTool
	ToolJSON string           `json:"tool_json,omitempty"` // raw tool input/result JSON
	Status   session.Status   `json:"status,omitempty"`    // EventStatus
	Err      string           `json:"err,omitempty"`       // EventError
	Seq      int              `json:"seq"`
}
