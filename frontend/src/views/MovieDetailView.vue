<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import CastList from '../components/CastList.vue'
import type { Movie } from '../types'
import { formatRuntime, formatTime } from '../utils/format'

const route = useRoute()
const router = useRouter()
const movie = ref<Movie | null>(null)

const posterUrl = computed(() => {
  if (!movie.value) return ''
  return movie.value.has_local_cover ? coverURL('movies', movie.value.id) : movie.value.poster_url
})

const hasResumePoint = computed(() => {
  const p = movie.value?.progress
  return !!p && !p.completed && p.position_seconds > 5
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
</style>
