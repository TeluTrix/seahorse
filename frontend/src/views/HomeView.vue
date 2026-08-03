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
  <div v-if="loading" class="flex justify-center p-16"><div class="spinner" /></div>
  <template v-else>
    <section class="mb-10">
      <h2 class="mb-3 text-lg font-black tracking-tight">
        <RouterLink :to="{ name: 'movies-overview' }" class="no-underline hover:underline">Movies</RouterLink>
      </h2>
      <p v-if="!movies.length" class="text-text-dim">No movies yet. Ask an admin to scan the library.</p>
      <div class="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-5">
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

    <section class="mb-10">
      <h2 class="mb-3 text-lg font-black tracking-tight">
        <RouterLink :to="{ name: 'tvshows-overview' }" class="no-underline hover:underline">TV Shows</RouterLink>
      </h2>
      <p v-if="!shows.length" class="text-text-dim">No tv shows yet. Ask an admin to scan the library.</p>
      <div class="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-5">
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
