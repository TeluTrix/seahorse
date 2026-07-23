<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import { useConfigStore } from '../stores/config'
import PosterCard from '../components/PosterCard.vue'
import type { TVShow } from '../types'
import { yearOf } from '../utils/format'

const route = useRoute()
const router = useRouter()

const q = ref((route.query.q as string) ?? '')
const year = ref((route.query.year as string) ?? '')
const genre = ref((route.query.genre as string) ?? '')
const page = ref(Number(route.query.page) || 1)
const pageSize = useConfigStore().defaultPageSize

const genres = ref<string[]>([])
const shows = ref<TVShow[]>([])
const total = ref(0)
const loading = ref(true)

function poster(show: TVShow): string {
  return show.has_local_cover ? coverURL('tvshows', show.id) : show.poster_url
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
    const result = await api.search({ type: 'tvshows', q: q.value, year: year.value, genre: genre.value, page: page.value, pageSize })
    shows.value = result.tv_shows
    total.value = result.tv_shows_total
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
  <div class="tvshows-view">
    <h1>TV Shows</h1>
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
      <p v-if="!shows.length" class="empty">No matching tv shows.</p>
      <div class="grid">
        <PosterCard
          v-for="show in shows"
          :key="show.id"
          :title="show.title"
          :poster-url="poster(show)"
          :year="yearOf(show.first_air_date)"
          @click="router.push({ name: 'tvshow', params: { id: show.id } })"
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
