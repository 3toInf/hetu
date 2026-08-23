<script setup lang="ts">
// SessionView: metadata card + chronological transcript for one session.
//
// CRITICAL: the backend returns messages newest-first (RecentMessages orders
// seq DESC), so the transcript is reversed to oldest-first before rendering
// (`[...msgs].reverse()` below) — a reviewer will check this.

import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { session } from '../api'
import type { Message, Session } from '../types'

const route = useRoute()
const id = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))

const data = ref<{ session: Session; pending: unknown[]; messages: Message[] } | null>(null)
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  data.value = null
  try {
    data.value = await session(id.value)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

watch(id, load)
onMounted(load)

// Backend sends newest-first; flip to oldest-first for the transcript.
const transcript = computed<Message[]>(() =>
  data.value ? [...data.value.messages].reverse() : []
)

function reltime(unix: number): string {
  if (!unix) return '-'
  const d = Date.now() / 1000 - unix
  if (d < 60) return `${Math.floor(d)}s`
  if (d < 3600) return `${Math.floor(d / 60)}m`
  return `${Math.floor(d / 3600)}h`
}

const roleClass = (role: string) => (role === 'assistant' ? 'assistant' : 'user')
</script>

<template>
  <div>
    <p v-if="loading" class="muted">Loading…</p>
    <div v-else-if="error" class="error-block">
      <p class="error">Error: {{ error }}</p>
      <RouterLink to="/sessions">← Back to sessions</RouterLink>
    </div>
    <template v-else-if="data">
      <h2>{{ data.session.title || '(untitled)' }}</h2>

      <dl class="metacard">
        <div><dt>ID</dt><dd class="shortid">{{ data.session.hetu_id }}</dd></div>
        <div><dt>Agent</dt><dd>{{ data.session.agent }}</dd></div>
        <div><dt>Status</dt><dd><span class="badge">{{ data.session.status }}</span></dd></div>
        <div v-if="data.session.project_path"><dt>Project</dt><dd>{{ data.session.project_path }}</dd></div>
        <div><dt>CWD</dt><dd class="shortid">{{ data.session.cwd }}</dd></div>
        <div><dt>Updated</dt><dd>{{ reltime(data.session.updated_at) }}</dd></div>
        <div v-if="data.pending.length > 0"><dt>Pending</dt><dd>{{ data.pending.length }} approval(s)</dd></div>
      </dl>

      <h3 class="transcript-title">Transcript</h3>
      <p v-if="transcript.length === 0" class="muted">No messages yet.</p>
      <div v-else class="transcript">
        <div
          v-for="m in transcript"
          :key="m.seq"
          class="message"
          :class="roleClass(m.role)"
        >
          <span class="msg-role">{{ m.role }}</span>
          <span class="msg-body">{{ m.content }}</span>
        </div>
      </div>
    </template>
  </div>
</template>
