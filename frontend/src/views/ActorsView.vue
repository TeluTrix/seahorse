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
  <div>
    <h1 class="mb-5 text-3xl font-black tracking-tight">Actors</h1>
    <form class="mb-8 flex flex-wrap items-center gap-3" @submit.prevent="handleSubmit">
      <input v-model="q" type="text" placeholder="Search actors..." class="field w-auto min-w-[220px]" />
      <button type="submit" class="btn-primary">Search</button>
    </form>

    <div v-if="loading" class="flex justify-center p-16"><div class="spinner" /></div>
    <template v-else>
      <p v-if="!actors.length" class="text-text-dim">No matching actors.</p>
      <div class="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-6">
        <div
          v-for="actor in actors"
          :key="actor.name"
          class="cursor-pointer text-center transition-transform duration-200 ease-out hover:scale-[1.04]"
          @click="router.push({ name: 'actor', params: { name: actor.name } })"
        >
          <div class="mx-auto mb-2.5 h-[120px] w-[120px] overflow-hidden rounded-full bg-bg-alt ring-1 ring-white/10">
            <img
              v-if="actor.profile_url"
              :src="actor.profile_url"
              :alt="actor.name"
              class="block h-full w-full object-cover"
            />
            <div v-else class="flex h-full w-full items-center justify-center text-[2.2rem] font-bold opacity-50">
              {{ actor.name.charAt(0) }}
            </div>
          </div>
          <div class="overflow-hidden text-ellipsis whitespace-nowrap text-[0.92rem] font-bold">
            {{ actor.name }}
          </div>
          <div class="text-[0.8rem] text-text-dim">
            {{ actor.credits }} title{{ actor.credits === 1 ? '' : 's' }}
          </div>
        </div>
      </div>
      <div v-if="maxPages() > 1" class="mt-4 flex items-center gap-4">
        <button class="btn-secondary" :disabled="page <= 1" @click="goToPage(page - 1)">Prev</button>
        <span class="text-sm text-text-dim">Page {{ page }} of {{ maxPages() }}</span>
        <button class="btn-secondary" :disabled="page >= maxPages()" @click="goToPage(page + 1)">Next</button>
      </div>
    </template>
  </div>
</template>
