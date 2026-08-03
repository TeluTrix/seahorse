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
  <div class="flex max-w-[520px] flex-col items-start gap-4">
    <h1 class="text-3xl font-black tracking-tight">Library</h1>
    <div class="flex gap-3">
      <button class="btn-primary" :disabled="status?.state === 'running'" @click="startScan(false)">
        {{ status?.state === 'running' ? 'Scanning…' : 'Scan Library' }}
      </button>
      <button class="btn-secondary" :disabled="status?.state === 'running'" @click="startScan(true)">
        Full Rescan
      </button>
    </div>
    <p class="text-sm text-text-dim">
      "Scan Library" only adds new movies/shows/episodes. "Full Rescan" wipes all cached covers and metadata and
      re-fetches everything from TMDB.
    </p>
    <p v-if="error" class="rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">{{ error }}</p>
    <div v-if="status" class="w-full">
      <p>Status: <strong>{{ status.state }}</strong></p>
      <p v-if="status.state === 'running' && status.current_item">Scanning: {{ status.current_item }}</p>
      <p v-if="status.state === 'running' || status.state === 'done'">
        Found {{ status.movies_found }} movies, {{ status.shows_found }} shows,
        {{ status.episodes_found }} episodes{{ status.state === 'running' ? ' so far…' : '.' }}
      </p>
      <div v-for="job in status.remux_jobs" :key="job.file" class="mt-1 w-full">
        <div class="mb-1 overflow-hidden text-ellipsis whitespace-nowrap text-[0.85rem] text-text-dim">
          Remuxing audio: {{ job.file }}
        </div>
        <div class="h-2 w-full overflow-hidden rounded-full bg-border">
          <div class="h-full bg-accent transition-[width] duration-300 ease-out" :style="{ width: job.percent + '%' }" />
        </div>
        <div class="mt-0.5 text-[0.8rem] text-text-dim">{{ Math.round(job.percent) }}%</div>
      </div>
      <p v-if="status.state === 'error'" class="mt-2 rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
        {{ status.error }}
      </p>
    </div>
  </div>
</template>
