<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { Movie } from '../types'

const route = useRoute()
const router = useRouter()
const movie = ref<Movie | null>(null)

onMounted(async () => {
  movie.value = await api.getMovie(route.params.id as string)
})

function play() {
  router.push({ name: 'watch-movie', params: { id: route.params.id } })
}
</script>

<template>
  <div
    v-if="movie"
    class="detail"
    :style="movie.backdrop_url ? { backgroundImage: `url(${movie.backdrop_url})` } : undefined"
  >
    <div class="overlay">
      <img v-if="movie.poster_url" :src="movie.poster_url" :alt="movie.title" class="poster" />
      <div>
        <h1>{{ movie.title }}</h1>
        <p class="meta">{{ movie.release_date }} · ⭐ {{ movie.vote_average.toFixed(1) }} · {{ movie.genres }}</p>
        <p class="overview">{{ movie.overview }}</p>
        <button @click="play">▶ Play</button>
      </div>
    </div>
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
  margin-bottom: 1rem;
}
.overview {
  margin-bottom: 1.5rem;
  max-width: 60ch;
}
</style>
