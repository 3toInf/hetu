package api

import "encoding/json"

type Request struct {
	Op   string          `json:"op"`
	Body json.RawMessage `json:"body,omitempty"`
}

type Response struct {
	OK   bool            `json:"ok"`
	Err  string          `json:"err,omitempty"`
	Body json.RawMessage `json:"body,omitempty"`
}

// Per-op body types (all plain JSON):
type ListSessionsReq struct{ ProjectPath string `json:"project_path"`; Status string `json:"status"`; Agent string `json:"agent"` }
type ListProjectsReq struct{}
type ListAgentsReq struct{}
type GetSessionReq struct{ ID string `json:"id"` }
type ResumeReq struct{ ID string `json:"id"` }
type SendReq struct{ ID string `json:"id"`; Prompt string `json:"prompt"` }
type CreateReq struct{ ProjectPath string `json:"project_path"`; Prompt string `json:"prompt"` }
type SearchReq struct{ Q string `json:"q"` }
type DiscoverReq struct{}
type SubscribeReq struct{ ID string `json:"id"` }
type ApproveReq struct{ ID string `json:"id"`; ToolUseID string `json:"tool_use_id,omitempty"`; Allow bool `json:"allow"`; Reason string `json:"reason,omitempty"` }
type MarkReadReq struct{ ID string `json:"id"` }

type SessionDTO struct {
	HetuID        string `json:"hetu_id"`
	Agent         string `json:"agent"`
	ExternalID    string `json:"external_id"`
	ProjectPath   string `json:"project_path"`
	CWD           string `json:"cwd"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Driven        bool   `json:"driven"`
	Unread        bool   `json:"unread"`
	NeedsAttention bool  `json:"needs_attention"`
	LastViewedAt  int64  `json:"last_viewed_at,omitempty"`
	UpdatedAt     int64  `json:"updated_at"`
}
type ProjectDTO struct{ ID int64 `json:"id"`; Name string `json:"name"`; Path string `json:"path"`; SessionCount int `json:"session_count"` }
type AgentDTO struct{ Name string `json:"name"`; Available bool `json:"available"`; Binary string `json:"binary"` }
