// TS interfaces mirroring the backend JSON DTOs (internal/api, snake_case).
// Keep these in sync with internal/api/types.go.

export interface Session {
  hetu_id: string
  agent: string
  external_id: string
  project_path: string
  cwd: string
  title: string
  status: string
  driven: boolean
  unread: boolean
  needs_attention: boolean
  last_viewed_at?: number
  updated_at: number
}

export interface Project {
  id: number
  name: string
  path: string
  session_count: number
  attention_count: number
}

export interface Message {
  role: string
  content: string
  seq: number
  ts: number
}

export interface SearchResult {
  hetu_id: string
  agent: string
  title: string
  project_path: string
  hits: number
  snippet: string
}

export interface PendingApproval {
  tool_use_id: string
  tool_name: string
  tool_input: string
}

// Mirrors internal/agent/event.go Event JSON (SSE `data:` payloads).
export interface AgentEvent {
  type: string
  text?: string
  tool_name?: string
  tool_json?: string
  tool_use_id?: string
  approval_state?: string
  status?: string
  err?: string
  seq: number
}
