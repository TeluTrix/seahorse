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
  <div>
    <h1 class="mb-5 text-3xl font-black tracking-tight">TV Shows</h1>
    <form class="mb-8 flex flex-wrap items-center gap-3" @submit.prevent="handleSubmit">
      <input v-model="q" type="text" placeholder="Search by title..." class="field w-auto min-w-[220px]" />
      <input v-model="year" type="text" placeholder="Year" maxlength="4" class="field w-[100px]" />
      <select v-model="genre" class="field w-auto cursor-pointer">
        <option value="">All genres</option>
        <option v-for="g in genres" :key="g" :value="g">{{ g }}</option>
      </select>
      <button type="submit" class="btn-primary">Search</button>
    </form>

    <div v-if="loading" class="flex justify-center p-16"><div class="spinner" /></div>
    <template v-else>
      <p v-if="!shows.length" class="text-text-dim">No matching tv shows.</p>
      <div class="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-5">
        <PosterCard
          v-for="show in shows"
          :key="show.id"
          :title="show.title"
          :poster-url="poster(show)"
          :year="yearOf(show.first_air_date)"
          @click="router.push({ name: 'tvshow', params: { id: show.id } })"
        />
      </div>
      <div v-if="maxPages() > 1" class="mt-4 flex items-center gap-4">
        <button class="btn-secondary" :disabled="page <= 1" @click="goToPage(page - 1)">Prev</button>
        <span class="text-sm text-text-dim">Page {{ page }} of {{ maxPages() }}</span>
        <button class="btn-secondary" :disabled="page >= maxPages()" @click="goToPage(page + 1)">Next</button>
      </div>
    </template>
  </div>
</template>
