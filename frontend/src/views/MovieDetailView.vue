<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import CastList from '../components/CastList.vue'
import RemuxStatusBadge from '../components/RemuxStatusBadge.vue'
import { useConfigStore } from '../stores/config'
import type { MediaInfo, Movie } from '../types'
import { formatBytes, formatCurrency, formatLanguage, formatRuntime, formatTime } from '../utils/format'

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

const posterUrl = computed(() => {
  if (!movie.value) return ''
  return movie.value.has_local_cover ? coverURL('movies', movie.value.id) : movie.value.poster_url
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
          </div>
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
