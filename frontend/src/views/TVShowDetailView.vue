<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import CastList from '../components/CastList.vue'
import { useConfigStore } from '../stores/config'
import type { Episode, TVShow } from '../types'
import { formatRuntime, formatTime } from '../utils/format'

const config = useConfigStore()
const route = useRoute()
const router = useRouter()
const show = ref<TVShow | null>(null)

const posterUrl = computed(() => {
  if (!show.value) return ''
  return show.value.has_local_cover ? coverURL('tvshows', show.value.id) : show.value.poster_url
})

interface FlatEpisode {
  episode: Episode
  seasonNumber: number
}

// The episode to feature as "Continue Watching": whichever episode has the
// most recently updated progress record. If that episode is already
// completed, there's nothing to resume, so advance to the next episode in
// the show instead (or hide the section if the show is fully watched).
const continueWatching = computed<FlatEpisode | null>(() => {
  if (!show.value?.seasons) return null

  const flat: FlatEpisode[] = []
  for (const season of show.value.seasons) {
    for (const episode of season.episodes) {
      flat.push({ episode, seasonNumber: season.season_number })
    }
  }

  let latestIndex = -1
  let latestTime = -Infinity
  flat.forEach((item, idx) => {
    if (!item.episode.progress) return
    const t = new Date(item.episode.progress.updated_at).getTime()
    if (t > latestTime) {
      latestTime = t
      latestIndex = idx
    }
  })
  if (latestIndex === -1) return null

  const latest = flat[latestIndex]
  if (!latest.episode.progress!.completed) {
    return latest
  }
  return flat[latestIndex + 1] ?? null
})

function playLabel(ep: Episode): string {
  if (ep.progress && !ep.progress.completed && ep.progress.position_seconds > config.resumeThresholdSeconds) {
    return `Resume ${formatTime(ep.progress.position_seconds)}`
  }
  return 'Play'
}

onMounted(async () => {
  show.value = await api.getTVShow(route.params.id as string)
})

function playEpisode(id: string, restart: boolean) {
  router.push({ name: 'watch-episode', params: { id }, query: restart ? { restart: '1' } : {} })
}
</script>

<template>
  <div v-if="show" class="show-detail">
    <Breadcrumbs :trail="[{ label: 'TV Shows', to: '/tvshows' }]" :current="show.title" fallback="/tvshows" />
    <div
      v-if="continueWatching"
      class="continue-watching"
      @click="playEpisode(continueWatching.episode.id, false)"
    >
      <img
        v-if="continueWatching.episode.still_url"
        :src="continueWatching.episode.still_url"
        :alt="continueWatching.episode.title"
      />
      <div class="cw-info">
        <span class="cw-label">Continue Watching</span>
        <strong>
          S{{ continueWatching.seasonNumber }}E{{ continueWatching.episode.episode_number }} ·
          {{ continueWatching.episode.title }}
        </strong>
        <div
          v-if="continueWatching.episode.progress && !continueWatching.episode.progress.completed"
          class="cw-progress-track"
        >
          <div
            class="cw-progress-fill"
            :style="{
              width:
                (continueWatching.episode.progress.position_seconds /
                  continueWatching.episode.progress.duration_seconds) *
                  100 +
                '%',
            }"
          />
        </div>
        <button @click.stop="playEpisode(continueWatching.episode.id, false)">
          ▶ {{ playLabel(continueWatching.episode) }}
        </button>
      </div>
    </div>

    <div class="header">
      <img v-if="posterUrl" :src="posterUrl" :alt="show.title" class="poster" />
      <div>
        <h1>{{ show.title }}</h1>
        <p class="meta">{{ show.first_air_date }} · ⭐ {{ show.vote_average.toFixed(1) }} · {{ show.genres }}</p>
        <p v-if="show.creators" class="creators">Created by {{ show.creators }}</p>
        <p>{{ show.overview }}</p>
      </div>
    </div>

    <CastList :cast="show.cast" />

    <div v-for="season in show.seasons" :key="season.id" class="season">
      <h2>Season {{ season.season_number }}</h2>
      <ul class="episodes">
        <li v-for="ep in season.episodes" :key="ep.id" :class="{ watched: ep.progress?.completed }">
          <div class="thumb-wrap" @click="playEpisode(ep.id, !!ep.progress?.completed)">
            <img v-if="ep.still_url" :src="ep.still_url" :alt="ep.title" />
            <div v-if="ep.progress?.completed" class="watched-badge" title="Watched">✓</div>
          </div>
          <div class="episode-info" @click="playEpisode(ep.id, !!ep.progress?.completed)">
            <strong>{{ ep.episode_number }}. {{ ep.title }}</strong>
            <span v-if="ep.runtime_minutes" class="runtime">{{ formatRuntime(ep.runtime_minutes) }}</span>
            <p>{{ ep.overview }}</p>
          </div>
          <div class="episode-actions">
            <button @click.stop="playEpisode(ep.id, false)">▶ {{ playLabel(ep) }}</button>
            <button v-if="ep.progress" class="secondary" @click.stop="playEpisode(ep.id, true)">Start Over</button>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.continue-watching {
  display: flex;
  gap: 1.25rem;
  align-items: center;
  padding: 1rem;
  margin-bottom: 2rem;
  background: var(--bg-alt);
  border: 1px solid var(--border);
  border-radius: 8px;
  cursor: pointer;
}
.continue-watching img {
  width: 180px;
  border-radius: 6px;
  flex-shrink: 0;
}
.cw-info {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  flex: 1;
}
.cw-label {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--accent);
  font-weight: 600;
}
.cw-progress-track {
  width: 100%;
  max-width: 320px;
  height: 4px;
  background: var(--border);
  border-radius: 2px;
  overflow: hidden;
}
.cw-progress-fill {
  height: 100%;
  background: var(--accent);
}
.continue-watching button {
  align-self: flex-start;
  margin-top: 0.25rem;
}
.header {
  display: flex;
  gap: 2rem;
  margin-bottom: 2rem;
}
.creators {
  opacity: 0.8;
  font-size: 0.9rem;
  margin-bottom: 0.5rem;
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
.episodes li.watched {
  opacity: 0.6;
}
.thumb-wrap {
  position: relative;
  cursor: pointer;
  flex-shrink: 0;
}
.episodes img {
  width: 160px;
  border-radius: 4px;
  height: fit-content;
  display: block;
}
.watched-badge {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--accent);
  color: var(--accent-text);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.85rem;
  font-weight: 700;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
}
.episode-info {
  flex: 1;
  cursor: pointer;
}
.runtime {
  color: var(--text-dim);
  font-size: 0.85rem;
  margin-left: 0.5rem;
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
