<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import type { TVShow } from '../types'
import { formatTime } from '../utils/format'

const route = useRoute()
const router = useRouter()
const show = ref<TVShow | null>(null)

const posterUrl = computed(() => {
  if (!show.value) return ''
  return show.value.has_local_cover ? coverURL('tvshows', show.value.id) : show.value.poster_url
})

onMounted(async () => {
  show.value = await api.getTVShow(route.params.id as string)
})

function playEpisode(id: string, restart: boolean) {
  router.push({ name: 'watch-episode', params: { id }, query: restart ? { restart: '1' } : {} })
}
</script>

<template>
  <div v-if="show" class="show-detail">
    <div class="header">
      <img v-if="posterUrl" :src="posterUrl" :alt="show.title" class="poster" />
      <div>
        <h1>{{ show.title }}</h1>
        <p class="meta">{{ show.first_air_date }} · ⭐ {{ show.vote_average.toFixed(1) }} · {{ show.genres }}</p>
        <p>{{ show.overview }}</p>
      </div>
    </div>

    <div v-for="season in show.seasons" :key="season.id" class="season">
      <h2>Season {{ season.season_number }}</h2>
      <ul class="episodes">
        <li v-for="ep in season.episodes" :key="ep.id">
          <img
            v-if="ep.still_url"
            :src="ep.still_url"
            :alt="ep.title"
            @click="playEpisode(ep.id, !!ep.progress?.completed)"
          />
          <div class="episode-info" @click="playEpisode(ep.id, !!ep.progress?.completed)">
            <strong>{{ ep.episode_number }}. {{ ep.title }} <span v-if="ep.progress?.completed" class="watched">✓ Watched</span></strong>
            <p>{{ ep.overview }}</p>
          </div>
          <div class="episode-actions">
            <button
              v-if="ep.progress && !ep.progress.completed && ep.progress.position_seconds > 5"
              @click.stop="playEpisode(ep.id, false)"
            >
              ▶ Resume {{ formatTime(ep.progress.position_seconds) }}
            </button>
            <button v-else @click.stop="playEpisode(ep.id, false)">▶ Play</button>
            <button
              v-if="ep.progress"
              class="secondary"
              @click.stop="playEpisode(ep.id, true)"
            >
              Start Over
            </button>
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
  align-items: center;
  gap: 1rem;
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
  cursor: pointer;
}
.episode-info {
  flex: 1;
  cursor: pointer;
}
.watched {
  font-size: 0.8rem;
  color: var(--accent);
  font-weight: 600;
}
.episode-actions {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  align-items: flex-end;
}
.episode-actions button {
  white-space: nowrap;
  font-size: 0.85rem;
  padding: 0.4rem 0.7rem;
}
button.secondary {
  background: transparent;
  color: inherit;
  border: 1px solid var(--border);
}
</style>
