<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, streamURL, subtitleURL, TOKEN_KEY } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import { useConfigStore } from '../stores/config'
import type { Episode, MediaType, NextEpisode, SubtitleTrack } from '../types'
import { formatLanguage } from '../utils/format'

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

// Reactive (not a plain const) because switching audio tracks means
// reloading against a different URL — see selectAudioTrack below.
const src = ref(streamURL(kind, mediaId))
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
  // A pending audio-track switch takes priority over the ordinary saved-
  // progress resume: it means this loadedmetadata firing is for a source
  // reloaded mid-playback (see selectAudioTrack), so the position to
  // restore is wherever the viewer just was, not the original watch
  // progress from whenever this page first mounted.
  const video = videoEl.value
  if (video && pendingAudioSeek.value) {
    video.currentTime = pendingAudioSeek.value.time
    if (pendingAudioSeek.value.resume) video.play()
    pendingAudioSeek.value = null
    return
  }
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

// Audio language is remembered per-browser (not per-item), the exact same
// mechanism as SUBTITLE_LANG_KEY above — whichever language was last
// selected carries forward into the next thing played automatically, with
// no special handling needed in playNext().
//
// Unlike subtitles, there's no native browser menu for picking an audio
// track (every major browser shows one for subtitles; none do for audio),
// hence the small custom picker in the template. There's also no way to
// switch which audio stream plays out of a single already-loaded file:
// HTMLMediaElement.audioTracks/videoTracks are unimplemented in mainstream
// Chrome and Firefox despite being spec'd (confirmed empirically against a
// real, current Chromium build, not just assumed) — so "pick a language"
// here means swapping the <video>'s src to ask the server for that specific
// track (see streamURL's optional track argument and
// internal/transcode.EnsureAudioTrack), not a client-side track switch.
const AUDIO_LANG_KEY = 'seahorse_audio_lang'

interface AudioTrackOption {
  id: string
  label: string
  language: string
}

const audioTracks = ref<AudioTrackOption[]>([])
const audioMenuOpen = ref(false)
const selectedAudioTrackId = ref('')
const selectedAudioLabel = computed(
  () => audioTracks.value.find((t) => t.id === selectedAudioTrackId.value)?.label ?? '',
)

// Set by selectAudioTrack() right before it swaps `src` to reload against a
// different track — captures where playback was so onLoadedMetadata can
// restore it once the new source's metadata is ready, instead of restarting
// from 0. Left null the rest of the time, so the ordinary
// applyResumeIfReady() flow runs unobstructed (including right after the
// very first load, when a saved preference resolves to a non-default track
// — see onMounted below; there's no prior position to preserve at that
// point, just the normal saved watch-progress resume to fall back to).
const pendingAudioSeek = ref<{ time: number; resume: boolean } | null>(null)

function selectAudioTrack(id: string) {
  audioMenuOpen.value = false
  if (id === selectedAudioTrackId.value) return
  const track = audioTracks.value.find((t) => t.id === id)
  if (!track) return
  const video = videoEl.value
  if (video) {
    pendingAudioSeek.value = { time: video.currentTime, resume: !video.paused }
  }
  selectedAudioTrackId.value = id
  localStorage.setItem(AUDIO_LANG_KEY, track.language)
  src.value = streamURL(kind, mediaId, id)
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

  // Only bother resolving a specific audio track (and reloading against it)
  // when there's actually more than one to choose from — for the
  // overwhelmingly common case of a file with zero or one audio stream,
  // `src` never changes from its initial value and playback starts exactly
  // as it did before this feature existed.
  const rawAudioTracks = await api.listAudioTracks(kind, mediaId).catch(() => [])
  if (rawAudioTracks.length > 1) {
    audioTracks.value = rawAudioTracks.map((t) => ({ id: t.id, language: t.language, label: formatLanguage(t.language) }))
    const preferred = localStorage.getItem(AUDIO_LANG_KEY)
    const match = audioTracks.value.find((t) => t.language === preferred)
    selectedAudioTrackId.value = (match ?? audioTracks.value[0]).id
    src.value = streamURL(kind, mediaId, selectedAudioTrackId.value)
  }

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
  <div class="flex flex-col gap-4">
    <Breadcrumbs v-if="trail.length" :trail="trail" :current="currentTitle" :fallback="fallback" />

    <div class="flex flex-col items-center gap-3">
      <div class="relative w-full">
        <video
          ref="videoEl"
          :src="src"
          controls
          autoplay
          class="block max-h-[80vh] w-full bg-black"
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

        <!-- No browser shows a native menu for picking an audio track the
             way every browser does for subtitles, so this is a small custom
             control — only shown when there's actually a choice to make. -->
        <div v-if="audioTracks.length > 1" class="absolute top-3.5 right-3.5">
          <button
            class="rounded-md border border-white/15 bg-black/75 px-3 py-1.5 text-[0.82rem] font-semibold text-white"
            @click="audioMenuOpen = !audioMenuOpen"
          >
            🔊 {{ selectedAudioLabel || 'Audio' }} ▾
          </button>
          <ul
            v-if="audioMenuOpen"
            class="absolute top-[calc(100%+0.4rem)] right-0 m-0 min-w-[140px] rounded-lg border border-white/12 bg-black/95 p-1.5 shadow-[0_8px_24px_rgb(0_0_0/0.4)]"
          >
            <li v-for="track in audioTracks" :key="track.id">
              <button
                class="block w-full rounded px-2.5 py-1.5 text-left text-[0.85rem] font-normal text-white hover:bg-white/10"
                @click="selectAudioTrack(track.id)"
              >
                {{ track.label }}
              </button>
            </li>
          </ul>
        </div>

        <div
          v-if="showNextPrompt && nextEpisode"
          class="next-overlay-in absolute right-5 bottom-18 flex max-w-[360px] gap-3.5 rounded-xl border border-white/12 bg-black/92 p-4 shadow-[0_8px_24px_rgb(0_0_0/0.4)]"
        >
          <button
            title="Dismiss"
            class="absolute top-1.5 right-2 bg-transparent p-0.5 text-[0.9rem] leading-none text-white/60 hover:text-white"
            @click="dismissNextPrompt"
          >
            ✕
          </button>
          <img
            v-if="nextEpisode.still_url"
            :src="nextEpisode.still_url"
            :alt="nextEpisode.title"
            class="h-fit w-24 shrink-0 rounded-md"
          />
          <div class="flex min-w-0 flex-col gap-1.5 text-white">
            <span class="text-xs font-bold tracking-wide text-accent uppercase">Up Next</span>
            <strong class="line-clamp-2 overflow-hidden text-[0.88rem]"
              >S{{ nextEpisode.season_number }}E{{ nextEpisode.episode_number }} · {{ nextEpisode.title }}</strong
            >
            <button class="btn-primary mt-0.5 w-fit px-3 py-1.5 text-[0.82rem]" @click="playNext">▶ Play Next</button>
          </div>
        </div>
      </div>

      <div
        v-if="nextEpisode"
        class="flex w-full items-center justify-between gap-4 rounded-lg border border-border bg-bg-alt px-4 py-3 text-sm"
      >
        <span>Up next: S{{ nextEpisode.season_number }}E{{ nextEpisode.episode_number }} · {{ nextEpisode.title }}</span>
        <button class="btn-secondary" @click="playNext">Watch Next ▶</button>
      </div>
    </div>

    <p v-if="overview" class="max-w-[70ch] leading-relaxed text-text-dim">{{ overview }}</p>

    <section v-if="seasonEpisodes.length">
      <h2 class="mb-3 text-[1.05rem] font-black tracking-tight">This Season</h2>
      <div class="flex gap-3.5 overflow-x-auto pb-2">
        <button
          v-for="ep in seasonEpisodes"
          :key="ep.id"
          class="flex w-44 shrink-0 flex-col gap-1.5 border-none bg-transparent p-0 text-left"
          :class="ep.id === mediaId ? 'cursor-default opacity-100' : 'cursor-pointer opacity-85 hover:opacity-100'"
          @click="ep.id !== mediaId && playEpisode(ep.id, false)"
        >
          <div
            class="relative aspect-video overflow-hidden rounded-md bg-bg-alt"
            :class="{ 'outline-2 outline-accent': ep.id === mediaId }"
          >
            <img v-if="ep.still_url" :src="ep.still_url" :alt="ep.title" class="block h-full w-full object-cover" />
            <span
              v-if="ep.progress?.completed"
              class="absolute top-1.5 right-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-accent text-[0.7rem] text-white"
              >✓</span
            >
          </div>
          <span
            class="line-clamp-2 overflow-hidden text-[0.82rem]"
            :class="ep.progress?.completed ? 'text-text-dim' : 'text-text'"
            >E{{ ep.episode_number }} · {{ ep.title }}</span
          >
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* Tailwind has no built-in "slide up + fade in" utility combo tied to a
   single custom keyframe, so this one animation stays as plain CSS rather
   than composing several arbitrary-value utilities. */
.next-overlay-in {
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
</style>
