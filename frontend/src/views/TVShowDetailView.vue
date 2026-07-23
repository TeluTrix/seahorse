<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import type { TVShow } from '../types'

const route = useRoute()
const router = useRouter()
const show = ref<TVShow | null>(null)

onMounted(async () => {
  show.value = await api.getTVShow(route.params.id as string)
})

function playEpisode(id: string) {
  router.push({ name: 'watch-episode', params: { id } })
}
</script>

<template>
  <div v-if="show" class="show-detail">
    <div class="header">
      <img v-if="show.poster_url" :src="show.poster_url" :alt="show.title" class="poster" />
      <div>
        <h1>{{ show.title }}</h1>
        <p class="meta">{{ show.first_air_date }} · ⭐ {{ show.vote_average.toFixed(1) }} · {{ show.genres }}</p>
        <p>{{ show.overview }}</p>
      </div>
    </div>

    <div v-for="season in show.seasons" :key="season.id" class="season">
      <h2>Season {{ season.season_number }}</h2>
      <ul class="episodes">
        <li v-for="ep in season.episodes" :key="ep.id" @click="playEpisode(ep.id)">
          <img v-if="ep.still_url" :src="ep.still_url" :alt="ep.title" />
          <div>
            <strong>{{ ep.episode_number }}. {{ ep.title }}</strong>
            <p>{{ ep.overview }}</p>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  gap: 2rem;
  margin-bottom: 2rem;
}
.poster {
  width: 200px;
  border-radius: 6px;
}
.season {
  margin-bottom: 2rem;
}
.episodes {
  list-style: none;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.episodes li {
  display: flex;
  gap: 1rem;
  cursor: pointer;
  padding: 0.5rem;
  border-radius: 6px;
}
.episodes li:hover {
  background: rgba(127, 127, 127, 0.15);
}
.episodes img {
  width: 160px;
  border-radius: 4px;
  height: fit-content;
}
</style>
