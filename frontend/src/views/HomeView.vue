<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import PosterCard from '../components/PosterCard.vue'
import type { Movie, TVShow } from '../types'
import { yearOf } from '../utils/format'

const PREVIEW_COUNT = 6

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
    const [movieResult, showResult] = await Promise.all([
      api.listMovies(1, PREVIEW_COUNT, 'newest'),
      api.listTVShows(1, PREVIEW_COUNT, 'newest'),
    ])
    movies.value = movieResult.movies
    shows.value = showResult.tv_shows
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading" class="center"><div class="spinner" /></div>
  <template v-else>
    <section>
      <h2><RouterLink :to="{ name: 'movies-overview' }">Movies</RouterLink></h2>
      <p v-if="!movies.length" class="empty">No movies yet. Ask an admin to scan the library.</p>
      <div class="grid">
        <PosterCard
          v-for="movie in movies"
          :key="movie.id"
          :title="movie.title"
          :poster-url="moviePoster(movie)"
          :year="yearOf(movie.release_date)"
          :watched="movie.progress?.completed"
          @click="router.push({ name: 'movie', params: { id: movie.id } })"
        />
      </div>
    </section>

    <section>
      <h2><RouterLink :to="{ name: 'tvshows-overview' }">TV Shows</RouterLink></h2>
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
    </section>
  </template>
</template>

<style scoped>
section {
  margin-bottom: 2.5rem;
}
h2 a {
  text-decoration: none;
}
h2 a:hover {
  text-decoration: underline;
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
