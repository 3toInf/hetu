<script setup lang="ts">
// SessionView: metadata card + chronological transcript + HITL controls
// (approve/deny, send, resume) + live SSE updates for driven sessions.
//
// CRITICAL: the backend returns messages newest-first (RecentMessages orders
// seq DESC), so stored messages are reversed to oldest-first before rendering;
// SSE-appended messages go at the END (newest last). SSE message seqs are
// negative local counters so they can never collide with stored seq keys.

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { approve, markRead, resume, send, session } from '../api'
import type { Detail } from '../api'
import type { Message, PendingApproval } from '../types'

const route = useRoute()
const id = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))

const data = ref<Detail | null>(null)
const loading = ref(true)
const error = ref('')
const liveMsgs = ref<Message[]>([]) // SSE-appended, oldest-first
const pending = ref<PendingApproval[]>([])
const status = ref('')
const busy = ref('') // '' | 'send' | 'resume' | `approve:<tool_use_id>`
const actionError = ref('')
const draft = ref('')

let es: EventSource | null = null
let liveSeq = -1

function closeSSE() {
  es?.close()
  es = null
}

function connectSSE() {
  es = new EventSource(`/api/sessions/${encodeURIComponent(id.value)}/events`)
  es.onmessage = (e) => {
    const ev = JSON.parse(e.data)
    if (ev.type === 'status') {
      if (ev.status) status.value = ev.status
    } else if (ev.type === 'text' && ev.text) {
      liveMsgs.value.push({ role: 'assistant', content: ev.text, seq: liveSeq--, ts: Math.floor(Date.now() / 1000) })
    } else if (ev.type === 'approval') {
      if (ev.approval_state === 'pending') {
        pending.value.push({ tool_use_id: ev.tool_use_id ?? '', tool_name: ev.tool_name ?? '', tool_input: ev.tool_json ?? '' })
      } else {
        pending.value = pending.value.filter((p) => p.tool_use_id !== ev.tool_use_id)
      }
    }
  }
  es.addEventListener('end', () => {
    closeSSE()
    load() // session finished: re-fetch final persisted state
  })
}

async function load() {
  closeSSE()
  loading.value = true
  error.value = ''
  data.value = null
  liveMsgs.value = []
  actionError.value = ''
  try {
    data.value = await session(id.value)
    status.value = data.value.session.status
    pending.value = data.value.pending ?? []
    if (data.value.session.driven) connectSSE()
    if (data.value.session.unread) {
      markRead(id.value).catch(() => {}) // best-effort; local flag flips either way
      data.value.session.unread = false
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

watch(id, load)
onMounted(load)
onBeforeUnmount(closeSSE)

// Stored messages newest-first → reverse; SSE tail is already oldest-first.
const transcript = computed<Message[]>(() =>
  data.value ? [...data.value.messages].reverse().concat(liveMsgs.value) : []
)

async function act(flag: string, fn: () => Promise<unknown>) {
  if (busy.value) return
  busy.value = flag
  actionError.value = ''
  try {
    await fn()
  } catch (e) {
    actionError.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = ''
  }
}

const doApprove = (p: PendingApproval, allow: boolean) =>
  act(`approve:${p.tool_use_id}`, async () => {
    await approve(id.value, allow, p.tool_use_id)
    pending.value = pending.value.filter((x) => x.tool_use_id !== p.tool_use_id)
  })

const doSend = () =>
  act('send', async () => {
    const text = draft.value.trim()
    if (!text) return
    await send(id.value, text)
    liveMsgs.value.push({ role: 'user', content: text, seq: liveSeq--, ts: Math.floor(Date.now() / 1000) })
    draft.value = ''
  })

const doResume = () =>
  act('resume', async () => {
    await resume(id.value)
    await load()
  })

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
        <div><dt>Status</dt><dd><span class="badge" :class="{ attn: data.session.needs_attention }">{{ status }}</span></dd></div>
        <div v-if="data.session.project_path"><dt>Project</dt><dd>{{ data.session.project_path }}</dd></div>
        <div><dt>CWD</dt><dd class="shortid">{{ data.session.cwd }}</dd></div>
        <div><dt>Updated</dt><dd>{{ reltime(data.session.updated_at) }}</dd></div>
      </dl>

      <div v-if="pending.length > 0" class="pending-list">
        <div v-for="p in pending" :key="p.tool_use_id" class="pending-card">
          <div class="pending-head">
            <strong>{{ p.tool_name }}</strong>
            <span class="muted">wants approval</span>
          </div>
          <pre class="pending-input">{{ p.tool_input }}</pre>
          <div class="pending-actions">
            <button class="btn" :disabled="busy !== ''" @click="doApprove(p, true)">Approve</button>
            <button class="btn btn-danger" :disabled="busy !== ''" @click="doApprove(p, false)">Deny</button>
          </div>
        </div>
      </div>

      <div class="sendbox">
        <textarea
          v-model="draft"
          rows="2"
          placeholder="Send a message…"
          aria-label="message"
          @keydown.enter.exact.prevent="doSend"
        ></textarea>
        <button v-if="data.session.driven" class="btn" :disabled="busy !== '' || draft.trim() === ''" @click="doSend">Send</button>
        <button v-else class="btn" :disabled="busy !== ''" @click="doResume">Resume</button>
      </div>
      <p v-if="actionError" class="error">{{ actionError }}</p>

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
