<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { api } from '../api/client'
import type { ScanStatus } from '../types'

const status = ref<ScanStatus | null>(null)
const error = ref('')
let pollHandle: number | undefined

async function startScan() {
  error.value = ''
  try {
    status.value = await api.scanLibrary()
    poll()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'could not start scan'
  }
}

function poll() {
  if (pollHandle) window.clearInterval(pollHandle)
  pollHandle = window.setInterval(async () => {
    status.value = await api.scanStatus()
    if (status.value.state !== 'running' && pollHandle) {
      window.clearInterval(pollHandle)
    }
  }, 2000)
}

onUnmounted(() => {
  if (pollHandle) window.clearInterval(pollHandle)
})
</script>

<template>
  <div class="admin">
    <h1>Admin dashboard</h1>
    <button :disabled="status?.state === 'running'" @click="startScan">
      {{ status?.state === 'running' ? 'Scanning…' : 'Scan Library' }}
    </button>
    <p v-if="error" class="error-message">{{ error }}</p>
    <div v-if="status" class="status">
      <p>Status: <strong>{{ status.state }}</strong></p>
      <p v-if="status.state === 'done'">
        Found {{ status.movies_found }} movies, {{ status.shows_found }} shows,
        {{ status.episodes_found }} episodes.
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
  max-width: 480px;
}
</style>
