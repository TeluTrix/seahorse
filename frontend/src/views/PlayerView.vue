<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, streamURL, subtitleURL, TOKEN_KEY } from '../api/client'
import type { MediaType, SubtitleTrack } from '../types'

const route = useRoute()
const kind = computed<'movies' | 'episodes'>(() => (route.name === 'watch-movie' ? 'movies' : 'episodes'))
const mediaType = computed<MediaType>(() => (route.name === 'watch-movie' ? 'movie' : 'episode'))
const mediaId = computed(() => route.params.id as string)
const restart = computed(() => route.query.restart === '1')

const src = computed(() => streamURL(kind.value, mediaId.value))
const tracks = ref<SubtitleTrack[]>([])

const videoEl = ref<HTMLVideoElement | null>(null)
let resumePosition = 0
let lastReported = 0

function report(position: number, duration: number) {
  if (!duration || Number.isNaN(duration)) return
  api.saveProgress(mediaType.value, mediaId.value, position, duration).catch(() => {})
}

function reportOnUnload() {
  const video = videoEl.value
  if (!video || !video.duration) return
  const token = localStorage.getItem(TOKEN_KEY) ?? ''
  fetch('/api/progress', {
    method: 'PUT',
    keepalive: true,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({
      media_type: mediaType.value,
      media_id: mediaId.value,
      position_seconds: video.currentTime,
      duration_seconds: video.duration,
    }),
  }).catch(() => {})
}

function onLoadedMetadata() {
  const video = videoEl.value
  if (!video) return
  if (!restart.value && resumePosition > 5 && resumePosition < video.duration - 5) {
    video.currentTime = resumePosition
  }
}

function onTimeUpdate() {
  const video = videoEl.value
  if (!video) return
  if (video.currentTime - lastReported >= 10) {
    lastReported = video.currentTime
    report(video.currentTime, video.duration)
  }
}

function onPause() {
  const video = videoEl.value
  if (!video) return
  report(video.currentTime, video.duration)
}

function onEnded() {
  const video = videoEl.value
  if (!video) return
  report(video.duration, video.duration)
}

onMounted(async () => {
  if (!restart.value) {
    const progress = await api.getProgress(mediaType.value, mediaId.value)
    if (progress) resumePosition = progress.position_seconds
  }
  tracks.value = await api.listSubtitles(kind.value, mediaId.value).catch(() => [])

  document.addEventListener('visibilitychange', handleVisibilityChange)
  window.addEventListener('beforeunload', reportOnUnload)
})

function handleVisibilityChange() {
  if (document.visibilityState === 'hidden') reportOnUnload()
}

onBeforeUnmount(() => {
  reportOnUnload()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  window.removeEventListener('beforeunload', reportOnUnload)
})
</script>

<template>
  <div class="player">
    <video
      ref="videoEl"
      :src="src"
      controls
      autoplay
      class="video"
      @loadedmetadata="onLoadedMetadata"
      @timeupdate="onTimeUpdate"
      @pause="onPause"
      @ended="onEnded"
    >
      <track
        v-for="track in tracks"
        :key="track.id"
        kind="subtitles"
        :src="subtitleURL(kind, mediaId, track.id)"
        :srclang="track.language"
        :label="`${track.label} (${track.source})`"
      />
    </video>
  </div>
</template>

<style scoped>
.player {
  display: flex;
  justify-content: center;
}
.video {
  width: 100%;
  max-height: 80vh;
  background: #000;
}
</style>
