<script setup lang="ts">
// SessionsView: filterable session list. Filters come from the route query
// (project_path/status/agent) so the Projects view can deep-link into a
// project's sessions. Row click → /sessions/:id (hetu_id).

import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { sessions } from '../api'
import type { Session } from '../types'

const route = useRoute()
const router = useRouter()

const qp = (k: string) => (typeof route.query[k] === 'string' ? (route.query[k] as string) : '')
const projectPath = ref(qp('project_path'))
const status = ref(qp('status'))
const agent = ref(qp('agent'))

const items = ref<Session[]>([])
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  const p = new URLSearchParams()
  if (projectPath.value) p.set('project_path', projectPath.value)
  if (status.value) p.set('status', status.value)
  if (agent.value) p.set('agent', agent.value)
  router.replace({ query: p.size ? Object.fromEntries(p) : {} })
  try {
    items.value = await sessions(p)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

watch([projectPath, status, agent], load)
onMounted(load)

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
    </form>

    <p v-if="loading" class="muted">Loading…</p>
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
        <tr v-for="s in items" :key="s.hetu_id" class="rowlink" @click="open(s)">
          <td>
            <span :class="{ unread: s.unread }">{{ s.title || '(untitled)' }}</span>
            <span v-if="s.unread" class="dot" title="unread">●</span>
          </td>
          <td><span class="badge">{{ s.status }}</span></td>
          <td class="muted">{{ reltime(s.updated_at) }}</td>
          <td class="muted shortid">{{ short(s.hetu_id) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
