// fetch wrappers for the hetud HTTP API (/api/*). All endpoints return JSON
// arrays/objects matching the DTOs in types.ts; non-2xx throws.

import type { Project, Session, Message, SearchResult } from './types'

async function j<T>(url: string): Promise<T> {
  const r = await fetch(url)
  if (!r.ok) throw new Error((await r.text()) || r.statusText)
  return r.json()
}

export const projects = () => j<Project[]>('/api/projects')

export const sessions = (p: URLSearchParams) => j<Session[]>(`/api/sessions?${p}`)

export const session = (id: string) =>
  j<{ session: Session; pending: unknown[]; messages: Message[] }>(`/api/sessions/${id}`)

export const search = (q: string) => j<SearchResult[]>(`/api/search?q=${encodeURIComponent(q)}`)

export const discover = () => fetch('/api/discover', { method: 'POST' })
