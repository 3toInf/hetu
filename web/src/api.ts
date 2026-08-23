// fetch wrappers for the hetud HTTP API (/api/*). All endpoints return JSON
// arrays/objects matching the DTOs in types.ts; non-2xx throws.
//
// post() always sends Content-Type: application/json — the daemon's CSRF
// guard rejects API POSTs with any other content type (415).

import type { AgentEvent, Message, PendingApproval, Project, SearchResult, Session } from './types'

export interface Detail {
  session: Session
  pending: PendingApproval[]
  messages: Message[]
}

async function j<T>(url: string): Promise<T> {
  const r = await fetch(url)
  if (!r.ok) throw new Error((await r.text()) || r.statusText)
  return r.json()
}

async function post<T>(url: string, body: unknown): Promise<T> {
  const r = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!r.ok) throw new Error((await r.text()) || r.statusText)
  return r.json()
}

export const projects = () => j<Project[]>('/api/projects')

export const sessions = (p: URLSearchParams) => j<Session[]>(`/api/sessions?${p}`)

export const session = (id: string, limit?: number) =>
  j<Detail>(`/api/sessions/${encodeURIComponent(id)}${limit ? `?limit=${limit}` : ''}`)

export const search = (q: string) => j<SearchResult[]>(`/api/search?q=${encodeURIComponent(q)}`)

export const discover = () => post<{ ok: boolean }>('/api/discover', {})

export const approve = (id: string, allow: boolean, toolUseID?: string) =>
  post<{ ok: boolean }>(`/api/sessions/${encodeURIComponent(id)}/approve`, {
    allow,
    tool_use_id: toolUseID,
  })

export const send = (id: string, prompt: string) =>
  post<{ ok: boolean }>(`/api/sessions/${encodeURIComponent(id)}/send`, { prompt })

export const resume = (id: string) =>
  post<{ id: string; driven: boolean }>(`/api/sessions/${encodeURIComponent(id)}/resume`, {})

export const markRead = (id: string) =>
  post<{ ok: boolean }>(`/api/sessions/${encodeURIComponent(id)}/mark_read`, {})

export const createSession = (projectPath: string) =>
  post<{ id: string }>('/api/sessions', { project_path: projectPath })

export type { AgentEvent, PendingApproval }
