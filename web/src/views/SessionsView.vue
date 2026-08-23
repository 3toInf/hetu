<script setup lang="ts">
// SessionsView: filterable session list with load-more pagination, a 10s
// visibility-gated poll for live badges, and a New session action. Filters
// come from the route query (project_path/status/agent) so Projects can
// deep-link in. Row click → /sessions/:id (hetu_id).
//
// Pagination is "grow the limit and re-fetch": the SQL ordering (status
// priority, updated_at DESC) is stable, so a bigger limit is always a
// superset — no offset/cursor bookkeeping needed at single-machine scale.

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createSession, sessions } from '../api'
import type { Session } from '../types'

const PAGE = 50

const route = useRoute()
const router = useRouter()

const qp = (k: string) => (typeof route.query[k] === 'string' ? (route.query[k] as string) : '')
const projectPath = ref(qp('project_path'))
const status = ref(qp('status'))
const agent = ref(qp('agent'))

const items = ref<Session[]>([])
const limit = ref(PAGE)
const loading = ref(true)
const error = ref('')
let timer: number | undefined

async function load() {
  loading.value = true
  error.value = ''
  const p = new URLSearchParams()
  if (projectPath.value) p.set('project_path', projectPath.value)
  if (status.value) p.set('status', status.value)
  if (agent.value) p.set('agent', agent.value)
  // Sync the URL to the FILTERS only — the pagination limit is view state,
  // not a shareable link parameter.
  const q: Record<string, string> = {}
  p.forEach((v, k) => {
    q[k] = v
  })
  router.replace({ query: q })
  p.set('limit', String(limit.value))
  try {
    items.value = await sessions(p)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

// Fewer rows than the requested limit ⇒ the listing is exhausted.
const exhausted = computed(() => items.value.length < limit.value)

function more() {
  limit.value += PAGE
  load()
}

// Filter changes restart from page one.
watch([projectPath, status, agent], () => {
  limit.value = PAGE
  load()
})
onMounted(() => {
  load()
  timer = window.setInterval(() => {
    if (document.visibilityState === 'visible' && !loading.value) load()
  }, 10_000)
})
onBeforeUnmount(() => window.clearInterval(timer))

function reltime(unix: number): string {
  if (!unix) return '-'
  const d = Date.now() / 1000 - unix
  if (d < 60) return `${Math.floor(d)}s`
  if (d < 3600) return `${Math.floor(d / 60)}m`
  return `${Math.floor(d / 3600)}h`
}

const short = (s: string) => (s.length > 8 ? s.slice(0, 8) : s)

function open(s: Session) {
  router.push(`/sessions/${encodeURIComponent(s.hetu_id)}`)
}

async function newSession() {
  const p = window.prompt('Project path (absolute)', '/')
  if (!p) return
  try {
    const { id } = await createSession(p)
    router.push(`/sessions/${encodeURIComponent(id)}`)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div>
    <h2>Sessions</h2>

    <form class="filter" @submit.prevent="load">
      <input v-model="projectPath" type="text" placeholder="project path" aria-label="project path" />
      <select v-model="status" aria-label="status">
        <option value="">all statuses</option>
        <option>Running</option>
        <option>Completed</option>
        <option>Idle</option>
        <option>Error</option>
      </select>
      <input v-model="agent" type="text" placeholder="agent" aria-label="agent" />
      <button type="submit">Filter</button>
      <button type="button" @click="newSession">New session</button>
    </form>

    <p v-if="loading && items.length === 0" class="muted">Loading…</p>
    <p v-else-if="error" class="error">Error: {{ error }}</p>
    <p v-else-if="items.length === 0" class="muted">No sessions match.</p>
    <table v-else class="list">
      <thead>
        <tr>
          <th>Title</th>
          <th>Status</th>
          <th>Updated</th>
          <th>ID</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="s in items"
          :key="s.hetu_id"
          class="rowlink"
          :class="{ 'row-attn': s.needs_attention }"
          @click="open(s)"
        >
          <td>
            <span :class="{ unread: s.unread }">{{ s.title || '(untitled)' }}</span>
            <span v-if="s.unread" class="dot" title="unread">●</span>
          </td>
          <td><span class="badge" :class="{ 'badge-attn': s.needs_attention }">{{ s.status }}</span></td>
          <td class="muted">{{ reltime(s.updated_at) }}</td>
          <td class="muted shortid">{{ short(s.hetu_id) }}</td>
        </tr>
      </tbody>
    </table>

    <button v-if="items.length > 0 && !exhausted" class="btn" style="margin-top: 1rem" @click="more">
      Load more
    </button>
  </div>
</template>
