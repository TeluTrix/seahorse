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
  <div class="actor-detail">
    <h1>{{ name }}</h1>
    <div v-if="loading" class="center"><div class="spinner" /></div>
    <template v-else-if="filmography">
      <section>
        <h2>Movies</h2>
        <p v-if="!filmography.movies.length" class="empty">No movies found.</p>
        <div class="grid">
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
      <section>
        <h2>TV Shows</h2>
        <p v-if="!filmography.tv_shows.length" class="empty">No tv shows found.</p>
        <div class="grid">
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
