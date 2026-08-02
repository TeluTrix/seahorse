<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import CastList from '../components/CastList.vue'
import RemuxStatusBadge from '../components/RemuxStatusBadge.vue'
import { useAuthStore } from '../stores/auth'
import { useConfigStore } from '../stores/config'
import type { MediaInfo, Movie } from '../types'
import { formatBytes, formatCurrency, formatLanguage, formatRuntime, formatTime } from '../utils/format'

const auth = useAuthStore()
const config = useConfigStore()
const route = useRoute()
const router = useRouter()
const movie = ref<Movie | null>(null)

// Media info is fetched lazily (only once the box is first expanded) since
// it runs ffprobe server-side and most viewers never open it — no reason
// to pay that cost on every single movie page view.
const mediaInfoOpen = ref(false)
const mediaInfoLoading = ref(false)
const mediaInfoError = ref(false)
const mediaInfo = ref<MediaInfo | null>(null)

async function toggleMediaInfo() {
  mediaInfoOpen.value = !mediaInfoOpen.value
  if (mediaInfoOpen.value && !mediaInfo.value && !mediaInfoError.value) {
    mediaInfoLoading.value = true
    try {
      mediaInfo.value = await api.getMovieMediaInfo(movie.value!.id)
    } catch {
      mediaInfoError.value = true
    } finally {
      mediaInfoLoading.value = false
    }
  }
}

const refreshing = ref(false)
const refreshError = ref('')
// The cover URL is stable (keyed by movie id, not content), so the browser
// happily serves its cached image even after a refresh replaces the file on
// disk — this cache-busts it by appending a version the browser hasn't seen.
const coverCacheBust = ref(0)

async function refreshMetadata() {
  if (!movie.value) return
  refreshing.value = true
  refreshError.value = ''
  try {
    movie.value = await api.refreshMovie(movie.value.id)
    coverCacheBust.value = Date.now()
    mediaInfo.value = null
    mediaInfoError.value = false
  } catch (e) {
    refreshError.value = e instanceof Error ? e.message : 'could not refresh metadata'
  } finally {
    refreshing.value = false
  }
}

const replacing = ref(false)
const replaceError = ref('')

async function replaceMetadata() {
  if (!movie.value) return
  if (
    !confirm(
      "This deletes this movie's cached cover and metadata, then re-discovers it from scratch via a new TMDB search (the only way to fix a wrong match). Any watch progress for it will be lost. Continue?",
    )
  ) {
    return
  }
  replacing.value = true
  replaceError.value = ''
  try {
    const updated = await api.replaceMovie(movie.value.id)
    // Unlike refreshMetadata, this assigns the movie a brand new id (a full
    // rediscovery, not an in-place update) — move the URL to match so a
    // refresh or the back button doesn't land on the now-deleted old id.
    // App.vue keys <RouterView> on the full path, so this remounts the page
    // fresh rather than leaving stale local state around.
    movie.value = updated
    await router.replace({ name: 'movie', params: { id: updated.id } })
  } catch (e) {
    replaceError.value = e instanceof Error ? e.message : 'could not replace metadata'
  } finally {
    replacing.value = false
  }
}

const posterUrl = computed(() => {
  if (!movie.value) return ''
  if (!movie.value.has_local_cover) return movie.value.poster_url
  const url = coverURL('movies', movie.value.id)
  return coverCacheBust.value ? `${url}&v=${coverCacheBust.value}` : url
})

const hasResumePoint = computed(() => {
  const p = movie.value?.progress
  return !!p && !p.completed && p.position_seconds > config.resumeThresholdSeconds
})

onMounted(async () => {
  movie.value = await api.getMovie(route.params.id as string)
})

function play(restart: boolean) {
  router.push({ name: 'watch-movie', params: { id: route.params.id }, query: restart ? { restart: '1' } : {} })
}
</script>

<template>
  <div v-if="movie">
    <Breadcrumbs :trail="[{ label: 'Movies', to: '/movies' }]" :current="movie.title" fallback="/movies" />
    <div
      class="detail"
      :style="movie.backdrop_url ? { backgroundImage: `url(${movie.backdrop_url})` } : undefined"
    >
      <div class="overlay">
        <img v-if="posterUrl" :src="posterUrl" :alt="movie.title" class="poster" />
        <div>
          <h1>{{ movie.title }}</h1>
          <p class="meta">
            {{ movie.release_date }}
            <template v-if="movie.runtime_minutes"> · {{ formatRuntime(movie.runtime_minutes) }}</template>
            · ⭐ {{ movie.vote_average.toFixed(1) }} · {{ movie.genres }}
          </p>
          <p v-if="movie.director" class="director">Directed by {{ movie.director }}</p>
          <RemuxStatusBadge :status="movie.remux_status" class="remux-notice" />
          <p v-if="movie.tagline" class="tagline">"{{ movie.tagline }}"</p>
          <p class="overview">{{ movie.overview }}</p>
          <div class="actions">
            <template v-if="hasResumePoint">
              <button @click="play(false)">▶ Resume from {{ formatTime(movie.progress!.position_seconds) }}</button>
              <button class="secondary" @click="play(true)">Start Over</button>
            </template>
            <button v-else @click="play(false)">▶ Play</button>
            <button
              v-if="auth.isAdmin"
              class="secondary"
              :disabled="refreshing || replacing"
              title="Re-fetch this movie's metadata and cover from TMDB, keeping the same match"
              @click="refreshMetadata"
            >
              {{ refreshing ? 'Refreshing…' : '⟳ Refresh Metadata' }}
            </button>
            <button
              v-if="auth.isAdmin"
              class="secondary"
              :disabled="refreshing || replacing"
              title="Delete and re-discover this movie from scratch (a new TMDB search) — fixes a wrong match"
              @click="replaceMetadata"
            >
              {{ replacing ? 'Rescanning…' : '⟲ Full Rescan' }}
            </button>
          </div>
          <p v-if="refreshError" class="error-message">{{ refreshError }}</p>
          <p v-if="replaceError" class="error-message">{{ replaceError }}</p>
        </div>
      </div>
    </div>

    <CastList :cast="movie.cast" />

    <section
      v-if="
        movie.original_language || movie.budget || movie.revenue || movie.production_companies || movie.production_countries
      "
      class="details-section"
    >
      <h2>Details</h2>
      <dl class="details-grid">
        <template v-if="movie.original_language">
          <dt>Original Language</dt>
          <dd>{{ formatLanguage(movie.original_language) }}</dd>
        </template>
        <template v-if="movie.budget">
          <dt>Budget</dt>
          <dd>{{ formatCurrency(movie.budget) }}</dd>
        </template>
        <template v-if="movie.revenue">
          <dt>Revenue</dt>
          <dd>{{ formatCurrency(movie.revenue) }}</dd>
        </template>
        <template v-if="movie.production_companies">
          <dt>Production</dt>
          <dd>{{ movie.production_companies }}</dd>
        </template>
        <template v-if="movie.production_countries">
          <dt>Countries</dt>
          <dd>{{ movie.production_countries }}</dd>
        </template>
      </dl>
    </section>

    <section class="media-info-section">
      <button class="media-info-toggle" @click="toggleMediaInfo">
        <span class="chevron" :class="{ open: mediaInfoOpen }">▸</span> Media Info
      </button>
      <div v-if="mediaInfoOpen" class="media-info-body">
        <div v-if="mediaInfoLoading" class="spinner" />
        <p v-else-if="mediaInfoError" class="empty">Media info unavailable.</p>
        <dl v-else-if="mediaInfo" class="details-grid">
          <template v-if="mediaInfo.width && mediaInfo.height">
            <dt>Resolution</dt>
            <dd>{{ mediaInfo.width }}×{{ mediaInfo.height }}</dd>
          </template>
          <template v-if="mediaInfo.video_codec">
            <dt>Video Codec</dt>
            <dd>{{ mediaInfo.video_codec }}</dd>
          </template>
          <template v-if="mediaInfo.audio_codec">
            <dt>Audio Codec</dt>
            <dd>{{ mediaInfo.audio_codec }}<span v-if="mediaInfo.audio_channels"> · {{ mediaInfo.audio_channels }}ch</span></dd>
          </template>
          <template v-if="mediaInfo.container">
            <dt>Container</dt>
            <dd>{{ mediaInfo.container }}</dd>
          </template>
          <dt>File Size</dt>
          <dd>{{ formatBytes(mediaInfo.file_size_bytes) }}</dd>
          <template v-if="mediaInfo.bitrate_kbps">
            <dt>Bitrate</dt>
            <dd>{{ mediaInfo.bitrate_kbps }} kbps</dd>
          </template>
        </dl>
      </div>
    </section>
  </div>
</template>

<style scoped>
.detail {
  background-size: cover;
  background-position: center;
  border-radius: 8px;
}
.overlay {
  display: flex;
  gap: 2rem;
  padding: 2rem;
  background: rgba(0, 0, 0, 0.6);
  border-radius: 8px;
  color: #fff;
}
.poster {
  width: 220px;
  border-radius: 6px;
  height: fit-content;
}
.meta {
  opacity: 0.8;
  margin-bottom: 0.5rem;
}
.director {
  opacity: 0.8;
  margin-bottom: 1rem;
  font-size: 0.9rem;
}
.remux-notice {
  display: block;
  width: fit-content;
  margin-bottom: 1rem;
}
.tagline {
  font-style: italic;
  opacity: 0.7;
  margin-bottom: 0.75rem;
}
.overview {
  margin-bottom: 1.5rem;
  max-width: 60ch;
}
.actions {
  display: flex;
  gap: 0.75rem;
}
button.secondary {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.4);
}

.details-section,
.media-info-section {
  margin-top: 2rem;
}
.details-grid {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.5rem 1.5rem;
  margin: 0;
}
.details-grid dt {
  color: var(--text-dim);
  font-size: 0.85rem;
}
.details-grid dd {
  margin: 0;
}

.media-info-toggle {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  font-size: 0.9rem;
  padding: 0.5rem 0.9rem;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}
.chevron {
  display: inline-block;
  transition: transform 0.15s ease;
  font-size: 0.75rem;
}
.chevron.open {
  transform: rotate(90deg);
}
.media-info-body {
  margin-top: 1rem;
}
</style>
