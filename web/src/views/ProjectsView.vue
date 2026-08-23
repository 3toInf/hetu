<script setup lang="ts">
// ProjectsView: table of all projects; row click narrows the sessions list to
// that project (navigates to /sessions?project_path=<path>).

import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { projects } from '../api'
import type { Project } from '../types'

const router = useRouter()
const items = ref<Project[]>([])
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    items.value = await projects()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

function open(p: Project) {
  router.push({ path: '/sessions', query: { project_path: p.path } })
}
</script>

<template>
  <div>
    <h2>Projects</h2>
    <p v-if="loading" class="muted">Loading…</p>
    <p v-else-if="error" class="error">Error: {{ error }}</p>
    <p v-else-if="items.length === 0" class="muted">No projects yet.</p>
    <table v-else class="list">
      <thead>
        <tr>
          <th>Name</th>
          <th>Path</th>
          <th>Sessions</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in items" :key="p.path" class="rowlink" @click="open(p)">
          <td>{{ p.name }}</td>
          <td class="muted">{{ p.path }}</td>
          <td>{{ p.session_count }}</td>
          <td>
            <span v-if="p.attention_count > 0" class="badge attention" title="needs attention">⚑{{ p.attention_count }}</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
