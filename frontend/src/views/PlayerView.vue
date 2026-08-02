<script setup lang="ts">
import { nextTick, onMounted, onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, streamURL, subtitleURL, TOKEN_KEY } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import { useConfigStore } from '../stores/config'
import type { Episode, MediaType, NextEpisode, SubtitleTrack } from '../types'

const config = useConfigStore()
const route = useRoute()
const router = useRouter()
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

// The player only ever receives a media id, so the breadcrumb trail and
// (for episodes) "Watch Next" data are fetched once on mount.
const trail = ref<{ label: string; to: string }[]>([])
const currentTitle = ref('')
const fallback = ref(kind === 'movies' ? '/movies' : '/tvshows')

const nextEpisode = ref<NextEpisode | null>(null)
const showNextPrompt = ref(false)
const dismissedNext = ref(false)
// How long before the end of an episode the "Watch Next" prompt appears —
// long enough to catch the start of end credits, short enough not to
// interrupt the episode itself.
const NEXT_PROMPT_WINDOW_SECONDS = 20

const overview = ref('')
// The rest of the current season, for the episode strip below the player
// (movies never populate this).
const seasonEpisodes = ref<Episode[]>([])

async function loadContext() {
  if (kind === 'movies') {
    const movie = await api.getMovie(mediaId)
    trail.value = [{ label: 'Movies', to: '/movies' }]
    currentTitle.value = movie.title
    fallback.value = `/movies/${mediaId}`
    overview.value = movie.overview
  } else {
    const episode = await api.getEpisode(mediaId)
    trail.value = [
      { label: 'TV Shows', to: '/tvshows' },
      { label: episode.show_title, to: `/tvshows/${episode.show_id}` },
    ]
    currentTitle.value = `S${episode.season_number}E${episode.episode_number} · ${episode.title}`
    fallback.value = `/tvshows/${episode.show_id}`
    overview.value = episode.overview
    nextEpisode.value = await api.getNextEpisode(mediaId)

    const show = await api.getTVShow(episode.show_id)
    const season = show.seasons?.find((s) => s.season_number === episode.season_number)
    seasonEpisodes.value = season?.episodes ?? []
  }
}

// Plain navigation for clicking an episode in the strip — unlike playNext()
// this doesn't force the current episode to "completed", since jumping
// around a season's episode list isn't the same explicit "I'm done with
// this one" signal as pressing the dedicated Watch Next action.
function playEpisode(id: string, restart: boolean) {
  router.push({ name: 'watch-episode', params: { id }, query: restart ? { restart: '1' } : {} })
}

function report(position: number, duration: number) {
  if (!duration || Number.isNaN(duration)) return
  api.saveProgress(mediaType, mediaId, position, duration).catch(() => {})
}

// Set once playNext() explicitly marks the episode watched, so the unmount
// cleanup below doesn't immediately overwrite that with the real (earlier)
// playback position — reportOnUnload always runs on unmount, including the
// one this navigation itself triggers.
let markedComplete = false

function reportOnUnload() {
  if (markedComplete) return
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
  const threshold = config.resumeThresholdSeconds
  if (resumePosition > threshold && resumePosition < video.duration - threshold) {
    video.currentTime = resumePosition
  }
}

function onLoadedMetadata() {
  applyResumeIfReady()
}

// Subtitle language is remembered per-browser (not per-item): whichever
// language was last showing gets applied automatically to the next thing
// played, whether that's resuming this same item later or moving on via
// "Watch Next" to a freshly-mounted episode with its own track list. Native
// <track> elements are used instead of a custom picker (see the template),
// so this works through the browser's own TextTrack API rather than any
// selection state we maintain ourselves.
const SUBTITLE_LANG_KEY = 'seahorse_subtitle_lang'

function applySubtitlePreference() {
  const video = videoEl.value
  if (!video) return
  const preferred = localStorage.getItem(SUBTITLE_LANG_KEY)
  if (!preferred) return
  for (let i = 0; i < video.textTracks.length; i++) {
    const track = video.textTracks[i]
    track.mode = track.language === preferred ? 'showing' : 'disabled'
  }
}

// Reacts to any textTracks change — the user picking a track via the native
// CC menu, or the browser's own delayed auto-selection heuristic, which
// (unlike kind="captions") isn't mutually exclusive for kind="subtitles":
// left alone, the browser can enable a second language on top of the one
// already showing, rendering both overlapping on screen. So this doesn't
// just persist the preference, it also enforces "only one track showing"
// itself, keeping whichever was already showing first.
function onTextTracksChange() {
  const video = videoEl.value
  if (!video) return
  let shown = ''
  for (let i = 0; i < video.textTracks.length; i++) {
    const track = video.textTracks[i]
    if (track.mode !== 'showing') continue
    if (!shown) {
      shown = track.language
    } else {
      track.mode = 'disabled'
    }
  }
  if (shown) {
    localStorage.setItem(SUBTITLE_LANG_KEY, shown)
  } else {
    localStorage.removeItem(SUBTITLE_LANG_KEY)
  }
}

// Surfaces the "Watch Next" overlay once playback enters the last
// NEXT_PROMPT_WINDOW_SECONDS of the episode — there's no browser API to
// inject a control into the native <video> controls themselves, so this
// draws on top of the player instead, Netflix/Plex-style.
function checkNextPrompt() {
  const video = videoEl.value
  if (!video || !nextEpisode.value || dismissedNext.value || !video.duration) return
  if (video.duration - video.currentTime <= NEXT_PROMPT_WINDOW_SECONDS) {
    showNextPrompt.value = true
  }
}

function onTimeUpdate() {
  const video = videoEl.value
  if (!video) return
  if (video.currentTime - lastReported >= config.progressReportIntervalSeconds) {
    lastReported = video.currentTime
    report(video.currentTime, video.duration)
  }
  checkNextPrompt()
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
  checkNextPrompt()
}

function dismissNextPrompt() {
  dismissedNext.value = true
  showNextPrompt.value = false
}

function playNext() {
  if (!nextEpisode.value) return
  // Clicking "Watch Next" means the user isn't coming back to finish this
  // one — mark it watched regardless of actual playback position, the same
  // "completed" report onEnded sends for a naturally-finished episode.
  const video = videoEl.value
  if (video) {
    markedComplete = true
    report(video.duration, video.duration)
  }
  router.push({ name: 'watch-episode', params: { id: nextEpisode.value.id } })
}

onMounted(async () => {
  loadContext().catch(() => {})

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
  await nextTick() // let the v-for below render the <track> elements first
  // Attached before applying the saved preference so it also catches (and
  // corrects) the browser's own delayed auto-selection heuristic, which can
  // otherwise enable a second track after ours has already been applied.
  videoEl.value?.textTracks.addEventListener('change', onTextTracksChange)
  applySubtitlePreference()

  document.addEventListener('visibilitychange', handleVisibilityChange)
  window.addEventListener('beforeunload', reportOnUnload)
  window.addEventListener('keydown', handleKeydown)
})

function handleVisibilityChange() {
  if (document.visibilityState === 'hidden') reportOnUnload()
}

// Left/Right arrow keys seek by config.playerSeekSeconds instead of the
// browser's native (and inconsistent, e.g. Chrome's default is 5s) skip
// amount, and Space toggles play/pause. Listens on window rather than the
// video element since focus isn't reliably on the video itself (e.g. right
// after a click elsewhere on the page).
function handleKeydown(e: KeyboardEvent) {
  const target = e.target as HTMLElement | null
  if (target && ['INPUT', 'TEXTAREA'].includes(target.tagName)) return

  const video = videoEl.value
  if (!video) return

  if (e.key === ' ') {
    e.preventDefault() // otherwise Space also scrolls the page
    if (video.paused) {
      video.play()
    } else {
      video.pause()
    }
    return
  }

  if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
  e.preventDefault()
  const delta = e.key === 'ArrowLeft' ? -config.playerSeekSeconds : config.playerSeekSeconds
  const duration = video.duration || Infinity
  video.currentTime = Math.min(duration, Math.max(0, video.currentTime + delta))
}

onBeforeUnmount(() => {
  reportOnUnload()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  window.removeEventListener('beforeunload', reportOnUnload)
  window.removeEventListener('keydown', handleKeydown)
  videoEl.value?.textTracks.removeEventListener('change', onTextTracksChange)
})
</script>

<template>
  <div class="player-page">
    <Breadcrumbs v-if="trail.length" :trail="trail" :current="currentTitle" :fallback="fallback" />

    <div class="player">
      <div class="video-wrap">
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

        <div v-if="showNextPrompt && nextEpisode" class="next-overlay">
          <button class="next-overlay-dismiss" title="Dismiss" @click="dismissNextPrompt">✕</button>
          <img
            v-if="nextEpisode.still_url"
            :src="nextEpisode.still_url"
            :alt="nextEpisode.title"
            class="next-overlay-thumb"
          />
          <div class="next-overlay-info">
            <span class="next-overlay-label">Up Next</span>
            <strong>S{{ nextEpisode.season_number }}E{{ nextEpisode.episode_number }} · {{ nextEpisode.title }}</strong>
            <button @click="playNext">▶ Play Next</button>
          </div>
        </div>
      </div>

      <div v-if="nextEpisode" class="next-below">
        <span>Up next: S{{ nextEpisode.season_number }}E{{ nextEpisode.episode_number }} · {{ nextEpisode.title }}</span>
        <button class="secondary" @click="playNext">Watch Next ▶</button>
      </div>
    </div>

    <p v-if="overview" class="synopsis">{{ overview }}</p>

    <section v-if="seasonEpisodes.length" class="season-episodes">
      <h2>This Season</h2>
      <div class="episode-strip">
        <button
          v-for="ep in seasonEpisodes"
          :key="ep.id"
          class="episode-card"
          :class="{ current: ep.id === mediaId, watched: ep.progress?.completed }"
          @click="ep.id !== mediaId && playEpisode(ep.id, false)"
        >
          <div class="episode-thumb-wrap">
            <img v-if="ep.still_url" :src="ep.still_url" :alt="ep.title" class="episode-thumb" />
            <span v-if="ep.progress?.completed" class="episode-watched-badge">✓</span>
          </div>
          <span class="episode-card-title">E{{ ep.episode_number }} · {{ ep.title }}</span>
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.player-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.player {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
}
.video-wrap {
  position: relative;
  width: 100%;
}
.video {
  width: 100%;
  max-height: 80vh;
  background: #000;
  display: block;
}

.next-overlay {
  position: absolute;
  right: 1.25rem;
  bottom: 4.5rem;
  display: flex;
  gap: 0.85rem;
  background: rgba(17, 18, 24, 0.92);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 10px;
  padding: 0.85rem 1rem;
  max-width: 360px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  animation: next-overlay-in 0.25s ease;
}
@keyframes next-overlay-in {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
.next-overlay-dismiss {
  position: absolute;
  top: 0.4rem;
  right: 0.5rem;
  background: transparent;
  color: rgba(255, 255, 255, 0.6);
  border: none;
  padding: 0.2rem;
  font-size: 0.9rem;
  line-height: 1;
}
.next-overlay-dismiss:hover {
  color: #fff;
}
.next-overlay-thumb {
  width: 96px;
  border-radius: 6px;
  flex-shrink: 0;
  height: fit-content;
}
.next-overlay-info {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  color: #fff;
  min-width: 0;
}
.next-overlay-label {
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--accent);
  font-weight: 600;
}
.next-overlay-info strong {
  font-size: 0.88rem;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.next-overlay-info button {
  align-self: flex-start;
  font-size: 0.82rem;
  padding: 0.4rem 0.75rem;
  margin-top: 0.1rem;
}

.next-below {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  background: var(--bg-alt);
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 0.9rem;
}

.synopsis {
  max-width: 70ch;
  color: var(--text-dim);
  line-height: 1.5;
}

.season-episodes h2 {
  font-size: 1.05rem;
  margin-bottom: 0.75rem;
}
.episode-strip {
  display: flex;
  gap: 0.9rem;
  overflow-x: auto;
  padding-bottom: 0.5rem;
}
.episode-card {
  flex: 0 0 auto;
  width: 180px;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  background: transparent;
  border: none;
  padding: 0;
  text-align: left;
  cursor: pointer;
  opacity: 0.85;
}
.episode-card:hover {
  opacity: 1;
}
.episode-card.current {
  opacity: 1;
  cursor: default;
}
.episode-thumb-wrap {
  position: relative;
  border-radius: 6px;
  overflow: hidden;
  background: var(--bg-alt);
  aspect-ratio: 16 / 9;
}
.episode-card.current .episode-thumb-wrap {
  outline: 2px solid var(--accent);
}
.episode-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.episode-watched-badge {
  position: absolute;
  top: 0.35rem;
  right: 0.35rem;
  background: var(--accent);
  color: #fff;
  border-radius: 50%;
  width: 1.2rem;
  height: 1.2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.7rem;
}
.episode-card-title {
  font-size: 0.82rem;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.episode-card.watched .episode-card-title {
  color: var(--text-dim);
}
</style>
