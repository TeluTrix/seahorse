<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL, DEFAULT_PAGE_SIZE } from '../api/client'
import PosterCard from '../components/PosterCard.vue'
import type { Movie } from '../types'
import { yearOf } from '../utils/format'

const route = useRoute()
const router = useRouter()

const q = ref((route.query.q as string) ?? '')
const year = ref((route.query.year as string) ?? '')
const genre = ref((route.query.genre as string) ?? '')
const page = ref(Number(route.query.page) || 1)
const pageSize = DEFAULT_PAGE_SIZE

const genres = ref<string[]>([])
const movies = ref<Movie[]>([])
const total = ref(0)
const loading = ref(true)

function poster(movie: Movie): string {
  return movie.has_local_cover ? coverURL('movies', movie.id) : movie.poster_url
}

function maxPages(): number {
  return Math.max(1, Math.ceil(total.value / pageSize))
}

function syncQueryString() {
  router.replace({
    query: {
      ...(q.value ? { q: q.value } : {}),
      ...(year.value ? { year: year.value } : {}),
      ...(genre.value ? { genre: genre.value } : {}),
      ...(page.value > 1 ? { page: String(page.value) } : {}),
    },
  })
}

async function load() {
  loading.value = true
  syncQueryString()
  try {
    const result = await api.search({ type: 'movies', q: q.value, year: year.value, genre: genre.value, page: page.value, pageSize })
    movies.value = result.movies
    total.value = result.movies_total
  } finally {
    loading.value = false
  }
}

function handleSubmit() {
  page.value = 1
  load()
}

function goToPage(p: number) {
  page.value = p
  load()
}

onMounted(async () => {
  genres.value = await api.listGenres().catch(() => [])
  await load()
})
</script>

<template>
  <div class="movies-view">
    <h1>Movies</h1>
    <form class="filters" @submit.prevent="handleSubmit">
      <input v-model="q" type="text" placeholder="Search by title..." class="q-input" />
      <input v-model="year" type="text" placeholder="Year" maxlength="4" class="year-input" />
      <select v-model="genre">
        <option value="">All genres</option>
        <option v-for="g in genres" :key="g" :value="g">{{ g }}</option>
      </select>
      <button type="submit">Search</button>
    </form>

    <div v-if="loading" class="center"><div class="spinner" /></div>
    <template v-else>
      <p v-if="!movies.length" class="empty">No matching movies.</p>
      <div class="grid">
        <PosterCard
          v-for="movie in movies"
          :key="movie.id"
          :title="movie.title"
          :poster-url="poster(movie)"
          :year="yearOf(movie.release_date)"
          :watched="movie.progress?.completed"
          @click="router.push({ name: 'movie', params: { id: movie.id } })"
        />
      </div>
      <div v-if="maxPages() > 1" class="pagination">
        <button class="secondary" :disabled="page <= 1" @click="goToPage(page - 1)">Prev</button>
        <span class="page-indicator">Page {{ page }} of {{ maxPages() }}</span>
        <button class="secondary" :disabled="page >= maxPages()" @click="goToPage(page + 1)">Next</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  gap: 0.75rem;
  margin-bottom: 2rem;
  flex-wrap: wrap;
  align-items: center;
}
.filters input,
.filters select {
  width: auto;
}
.q-input {
  min-width: 220px;
}
.year-input {
  width: 100px !important;
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
