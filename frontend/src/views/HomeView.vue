<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/client'
import type { Movie, TVShow } from '../types'

const movies = ref<Movie[]>([])
const shows = ref<TVShow[]>([])
const loading = ref(true)
const router = useRouter()

onMounted(async () => {
  try {
    const [movieResults, showResults] = await Promise.all([api.listMovies(), api.listTVShows()])
    movies.value = movieResults
    shows.value = showResults
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading" class="center"><div class="spinner" /></div>
  <template v-else>
    <section>
      <h2>Movies</h2>
      <p v-if="!movies.length" class="empty">No movies yet. Ask an admin to scan the library.</p>
      <div class="grid">
        <div
          v-for="movie in movies"
          :key="movie.id"
          class="card"
          @click="router.push({ name: 'movie', params: { id: movie.id } })"
        >
          <img v-if="movie.poster_url" :src="movie.poster_url" :alt="movie.title" />
          <div class="card-title">{{ movie.title }}</div>
        </div>
      </div>
    </section>

    <section>
      <h2>TV Shows</h2>
      <p v-if="!shows.length" class="empty">No tv shows yet. Ask an admin to scan the library.</p>
      <div class="grid">
        <div
          v-for="show in shows"
          :key="show.id"
          class="card"
          @click="router.push({ name: 'tvshow', params: { id: show.id } })"
        >
          <img v-if="show.poster_url" :src="show.poster_url" :alt="show.title" />
          <div class="card-title">{{ show.title }}</div>
        </div>
      </div>
    </section>
  </template>
</template>

<style scoped>
section {
  margin-bottom: 2.5rem;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 1.25rem;
}
.center {
  display: flex;
  justify-content: center;
  padding: 4rem;
}
</style>
