<script setup lang="ts">
// SearchView: full-text search over session messages. Submit → search(q);
// results list with title, hit count and snippet; row click → /sessions/:hetu_id.

import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { search } from '../api'
import type { SearchResult } from '../types'

const router = useRouter()
const q = ref('')
const results = ref<SearchResult[]>([])
const searched = ref(false)
const loading = ref(false)
const error = ref('')

async function run() {
  if (!q.value.trim()) return
  loading.value = true
  error.value = ''
  searched.value = true
  try {
    results.value = await search(q.value.trim())
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    results.value = []
  } finally {
    loading.value = false
  }
}

function open(r: SearchResult) {
  router.push(`/sessions/${encodeURIComponent(r.hetu_id)}`)
}
</script>

<template>
  <div>
    <h2>Search</h2>

    <form class="filter" @submit.prevent="run">
      <input v-model="q" type="search" placeholder="Search messages…" aria-label="search" autofocus />
      <button type="submit" :disabled="loading">Search</button>
    </form>

    <p v-if="loading" class="muted">Searching…</p>
    <p v-else-if="error" class="error">Error: {{ error }}</p>
    <p v-else-if="searched && results.length === 0" class="muted">No results.</p>
    <table v-else-if="results.length > 0" class="list">
      <thead>
        <tr>
          <th>Title</th>
          <th>Hits</th>
          <th>Snippet</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in results" :key="r.hetu_id" class="rowlink" @click="open(r)">
          <td>{{ r.title || '(untitled)' }}<span class="muted shortid"> {{ r.agent }}</span></td>
          <td class="muted">({{ r.hits }} hits)</td>
          <td class="snippet">{{ r.snippet }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
