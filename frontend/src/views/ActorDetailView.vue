<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import PosterCard from '../components/PosterCard.vue'
import type { ActorFilmography, Movie, TVShow } from '../types'
import { yearOf } from '../utils/format'

const route = useRoute()
const router = useRouter()
const name = route.params.name as string

const filmography = ref<ActorFilmography | null>(null)
const loading = ref(true)

function moviePoster(movie: Movie): string {
  return movie.has_local_cover ? coverURL('movies', movie.id) : movie.poster_url
}
function showPoster(show: TVShow): string {
  return show.has_local_cover ? coverURL('tvshows', show.id) : show.poster_url
}

onMounted(async () => {
  loading.value = true
  try {
    filmography.value = await api.getActorFilmography(name)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <h1 class="mb-5 text-3xl font-black tracking-tight">{{ name }}</h1>
    <div v-if="loading" class="flex justify-center p-16"><div class="spinner" /></div>
    <template v-else-if="filmography">
      <section class="mb-10">
        <h2 class="mb-3 text-lg font-black tracking-tight">Movies</h2>
        <p v-if="!filmography.movies.length" class="text-text-dim">No movies found.</p>
        <div class="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-5">
          <PosterCard
            v-for="movie in filmography.movies"
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
        <h2 class="mb-3 text-lg font-black tracking-tight">TV Shows</h2>
        <p v-if="!filmography.tv_shows.length" class="text-text-dim">No tv shows found.</p>
        <div class="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-5">
          <PosterCard
            v-for="show in filmography.tv_shows"
            :key="show.id"
            :title="show.title"
            :poster-url="showPoster(show)"
            :year="yearOf(show.first_air_date)"
            @click="router.push({ name: 'tvshow', params: { id: show.id } })"
          />
        </div>
      </section>
    </template>
  </div>
</template>
