<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { useConfigStore } from '../stores/config'
import type { Actor } from '../types'

const route = useRoute()
const router = useRouter()

const q = ref((route.query.q as string) ?? '')
const page = ref(Number(route.query.page) || 1)
const pageSize = useConfigStore().defaultPageSize

const actors = ref<Actor[]>([])
const total = ref(0)
const loading = ref(true)

function maxPages(): number {
  return Math.max(1, Math.ceil(total.value / pageSize))
}

function syncQueryString() {
  router.replace({
    query: {
      ...(q.value ? { q: q.value } : {}),
      ...(page.value > 1 ? { page: String(page.value) } : {}),
    },
  })
}

async function load() {
  loading.value = true
  syncQueryString()
  try {
    const result = await api.listActors({ q: q.value, page: page.value, pageSize })
    actors.value = result.actors
    total.value = result.total
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

onMounted(load)
</script>

<template>
  <div class="actors-view">
    <h1>Actors</h1>
    <form class="filters" @submit.prevent="handleSubmit">
      <input v-model="q" type="text" placeholder="Search actors..." class="q-input" />
      <button type="submit">Search</button>
    </form>

    <div v-if="loading" class="center"><div class="spinner" /></div>
    <template v-else>
      <p v-if="!actors.length" class="empty">No matching actors.</p>
      <div class="grid">
        <div
          v-for="actor in actors"
          :key="actor.name"
          class="actor-card"
          @click="router.push({ name: 'actor', params: { name: actor.name } })"
        >
          <div class="headshot-wrap">
            <img v-if="actor.profile_url" :src="actor.profile_url" :alt="actor.name" />
            <div v-else class="headshot-placeholder">{{ actor.name.charAt(0) }}</div>
          </div>
          <div class="actor-name">{{ actor.name }}</div>
          <div class="actor-credits">{{ actor.credits }} title{{ actor.credits === 1 ? '' : 's' }}</div>
        </div>
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
.filters input {
  width: auto;
  min-width: 220px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 1.5rem;
}
.actor-card {
  text-align: center;
  cursor: pointer;
}
.headshot-wrap {
  width: 120px;
  height: 120px;
  margin: 0 auto 0.6rem;
  border-radius: 50%;
  overflow: hidden;
  background: var(--bg-alt);
}
.headshot-wrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.headshot-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2.2rem;
  font-weight: 600;
  opacity: 0.5;
}
.actor-name {
  font-weight: 600;
  font-size: 0.92rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.actor-credits {
  font-size: 0.8rem;
  opacity: 0.6;
}
.center {
  display: flex;
  justify-content: center;
  padding: 4rem;
}
</style>
