<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { api, scanEventsURL } from '../api/client'
import type { ScanStatus } from '../types'

const status = ref<ScanStatus | null>(null)
const error = ref('')
let eventSource: EventSource | null = null

// A single persistent connection the server pushes updates over — no
// polling. Opened once on mount (so live status shows up even if a scan is
// already running, e.g. after a page reload) rather than only around each
// button click.
function connect() {
  eventSource = new EventSource(scanEventsURL())
  eventSource.onmessage = (event) => {
    status.value = JSON.parse(event.data)
  }
}

async function startScan(full: boolean) {
  if (full && !confirm('This deletes all cached covers and metadata, then re-fetches everything from TMDB. Continue?')) {
    return
  }
  error.value = ''
  try {
    await api.scanLibrary(full) // status updates arrive over the SSE stream, not this response
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'could not start scan'
  }
}

onMounted(connect)
onBeforeUnmount(() => eventSource?.close())
</script>

<template>
  <div class="admin">
    <h1>Library</h1>
    <div class="actions">
      <button :disabled="status?.state === 'running'" @click="startScan(false)">
        {{ status?.state === 'running' ? 'Scanning…' : 'Scan Library' }}
      </button>
      <button class="secondary" :disabled="status?.state === 'running'" @click="startScan(true)">
        Full Rescan
      </button>
    </div>
    <p class="hint">
      "Scan Library" only adds new movies/shows/episodes. "Full Rescan" wipes all cached covers and metadata and
      re-fetches everything from TMDB.
    </p>
    <p v-if="error" class="error-message">{{ error }}</p>
    <div v-if="status" class="status">
      <p>Status: <strong>{{ status.state }}</strong></p>
      <p v-if="status.state === 'running' && status.current_item">Scanning: {{ status.current_item }}</p>
      <p v-if="status.state === 'running' || status.state === 'done'">
        Found {{ status.movies_found }} movies, {{ status.shows_found }} shows,
        {{ status.episodes_found }} episodes{{ status.state === 'running' ? ' so far…' : '.' }}
      </p>
      <p v-if="status.state === 'error'" class="error-message">{{ status.error }}</p>
    </div>
  </div>
</template>

<style scoped>
.admin {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1rem;
  max-width: 520px;
}
.actions {
  display: flex;
  gap: 0.75rem;
}
.hint {
  color: var(--text-dim);
  font-size: 0.9rem;
}
</style>
