<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import type { Movie, TVShow } from '../types'

const movies = ref<Movie[]>([])
const shows = ref<TVShow[]>([])
const loading = ref(true)
const router = useRouter()

function moviePoster(movie: Movie): string {
  return movie.has_local_cover ? coverURL('movies', movie.id) : movie.poster_url
}
function showPoster(show: TVShow): string {
  return show.has_local_cover ? coverURL('tvshows', show.id) : show.poster_url
}

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
          <img v-if="moviePoster(movie)" :src="moviePoster(movie)" :alt="movie.title" />
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
          <img v-if="showPoster(show)" :src="showPoster(show)" :alt="show.title" />
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
