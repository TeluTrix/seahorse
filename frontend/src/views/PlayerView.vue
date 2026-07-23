<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, streamURL, subtitleURL, TOKEN_KEY } from '../api/client'
import type { MediaType, SubtitleTrack } from '../types'

const route = useRoute()
// Captured once, not as computed(): Vue Router's `route` is a single shared
// reactive object, and it already reflects the *destination* route by the
// time this component's unmount cleanup runs (e.g. reporting final progress
// on navigating away) — a computed() bound to route.name/params would read
// the wrong values at exactly that moment, silently reporting progress
// under the wrong media type. These never change over this component's
// lifetime anyway, so a one-time snapshot is both correct and simpler.
const kind: 'movies' | 'episodes' = route.name === 'watch-movie' ? 'movies' : 'episodes'
const mediaType: MediaType = route.name === 'watch-movie' ? 'movie' : 'episode'
const mediaId = route.params.id as string
const restart = route.query.restart === '1'

const src = streamURL(kind, mediaId)
const tracks = ref<SubtitleTrack[]>([])

const videoEl = ref<HTMLVideoElement | null>(null)
let resumePosition = 0
let lastReported = 0

function report(position: number, duration: number) {
  if (!duration || Number.isNaN(duration)) return
  api.saveProgress(mediaType, mediaId, position, duration).catch(() => {})
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
      media_type: mediaType,
      media_id: mediaId,
      position_seconds: video.currentTime,
      duration_seconds: video.duration,
    }),
  }).catch(() => {})
}

// Applies the fetched resume position to the video, if both are ready.
// Called from two places because of a race between the progress fetch (an
// async network call) and the video's own "loadedmetadata" event: whichever
// finishes first has to defer to whichever finishes second, since
// "loadedmetadata" only fires once and video.duration is unknown until then.
function applyResumeIfReady() {
  const video = videoEl.value
  if (!video || restart) return
  if (video.readyState < 1 || !video.duration) return // HAVE_METADATA not reached yet
  if (resumePosition > 5 && resumePosition < video.duration - 5) {
    video.currentTime = resumePosition
  }
}

function onLoadedMetadata() {
  applyResumeIfReady()
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
  if (!restart) {
    const progress = await api.getProgress(mediaType, mediaId)
    // Mirror the detail page's own "should we offer to resume" condition
    // (see MovieDetailView/TVShowDetailView's hasResumePoint) — otherwise an
    // item marked completed shows "Play" on the detail page but silently
    // jumps back to the old position here anyway.
    if (progress && !progress.completed) resumePosition = progress.position_seconds
    applyResumeIfReady() // metadata may have already loaded while this was in flight
  }
  tracks.value = await api.listSubtitles(kind, mediaId).catch(() => [])

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
        :label="track.label"
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
