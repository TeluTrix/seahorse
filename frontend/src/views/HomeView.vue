<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import PosterCard from '../components/PosterCard.vue'
import type { Movie, TVShow } from '../types'
import { yearOf } from '../utils/format'

const movies = ref<Movie[]>([])
const moviesTotal = ref(0)
const moviesPage = ref(1)

const shows = ref<TVShow[]>([])
const showsTotal = ref(0)
const showsPage = ref(1)

const pageSize = 50
const loading = ref(true)
const router = useRouter()

function moviePoster(movie: Movie): string {
  return movie.has_local_cover ? coverURL('movies', movie.id) : movie.poster_url
}
function showPoster(show: TVShow): string {
  return show.has_local_cover ? coverURL('tvshows', show.id) : show.poster_url
}

async function loadMovies() {
  const result = await api.listMovies(moviesPage.value, pageSize)
  movies.value = result.movies
  moviesTotal.value = result.total
}

async function loadShows() {
  const result = await api.listTVShows(showsPage.value, pageSize)
  shows.value = result.tv_shows
  showsTotal.value = result.total
}

watch(moviesPage, loadMovies)
watch(showsPage, loadShows)

onMounted(async () => {
  try {
    await Promise.all([loadMovies(), loadShows()])
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
        <PosterCard
          v-for="movie in movies"
          :key="movie.id"
          :title="movie.title"
          :poster-url="moviePoster(movie)"
          :year="yearOf(movie.release_date)"
          @click="router.push({ name: 'movie', params: { id: movie.id } })"
        />
      </div>
      <div v-if="moviesTotal > pageSize" class="pagination">
        <button class="secondary" :disabled="moviesPage <= 1" @click="moviesPage--">Prev</button>
        <span class="page-indicator">Page {{ moviesPage }} of {{ Math.ceil(moviesTotal / pageSize) }}</span>
        <button class="secondary" :disabled="moviesPage >= Math.ceil(moviesTotal / pageSize)" @click="moviesPage++">
          Next
        </button>
      </div>
    </section>

    <section>
      <h2>TV Shows</h2>
      <p v-if="!shows.length" class="empty">No tv shows yet. Ask an admin to scan the library.</p>
      <div class="grid">
        <PosterCard
          v-for="show in shows"
          :key="show.id"
          :title="show.title"
          :poster-url="showPoster(show)"
          :year="yearOf(show.first_air_date)"
          @click="router.push({ name: 'tvshow', params: { id: show.id } })"
        />
      </div>
      <div v-if="showsTotal > pageSize" class="pagination">
        <button class="secondary" :disabled="showsPage <= 1" @click="showsPage--">Prev</button>
        <span class="page-indicator">Page {{ showsPage }} of {{ Math.ceil(showsTotal / pageSize) }}</span>
        <button class="secondary" :disabled="showsPage >= Math.ceil(showsTotal / pageSize)" @click="showsPage++">
          Next
        </button>
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
