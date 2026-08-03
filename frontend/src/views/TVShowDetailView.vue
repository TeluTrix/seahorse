<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, coverURL } from '../api/client'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import CastList from '../components/CastList.vue'
import RemuxStatusBadge from '../components/RemuxStatusBadge.vue'
import { useAuthStore } from '../stores/auth'
import { useConfigStore } from '../stores/config'
import type { Episode, TVShow } from '../types'
import { formatRuntime, formatTime } from '../utils/format'

const auth = useAuthStore()
const config = useConfigStore()
const route = useRoute()
const router = useRouter()
const show = ref<TVShow | null>(null)

const refreshing = ref(false)
const refreshError = ref('')
// The cover URL is stable (keyed by show id, not content), so the browser
// happily serves its cached image even after a refresh replaces the file on
// disk — this cache-busts it by appending a version the browser hasn't seen.
const coverCacheBust = ref(0)

async function refreshMetadata() {
  if (!show.value) return
  refreshing.value = true
  refreshError.value = ''
  try {
    show.value = await api.refreshTVShow(show.value.id)
    coverCacheBust.value = Date.now()
  } catch (e) {
    refreshError.value = e instanceof Error ? e.message : 'could not refresh metadata'
  } finally {
    refreshing.value = false
  }
}

const replacing = ref(false)
const replaceError = ref('')

async function replaceMetadata() {
  if (!show.value) return
  if (
    !confirm(
      "This deletes this show's seasons, episodes, cached cover, and metadata, then re-discovers everything from scratch via a new TMDB search (the only way to fix a wrong match or re-derive episode titles/overviews). Any watch progress for its episodes will be lost. Continue?",
    )
  ) {
    return
  }
  replacing.value = true
  replaceError.value = ''
  try {
    const updated = await api.replaceTVShow(show.value.id)
    // Unlike refreshMetadata, this assigns the show a brand new id (a full
    // rediscovery, not an in-place update) — move the URL to match so a
    // refresh or the back button doesn't land on the now-deleted old id.
    // App.vue keys <RouterView> on the full path, so this remounts the page
    // fresh rather than leaving stale local state around.
    show.value = updated
    await router.replace({ name: 'tvshow', params: { id: updated.id } })
  } catch (e) {
    replaceError.value = e instanceof Error ? e.message : 'could not replace metadata'
  } finally {
    replacing.value = false
  }
}

const posterUrl = computed(() => {
  if (!show.value) return ''
  if (!show.value.has_local_cover) return show.value.poster_url
  const url = coverURL('tvshows', show.value.id)
  return coverCacheBust.value ? `${url}&v=${coverCacheBust.value}` : url
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
  <div v-if="show">
    <Breadcrumbs :trail="[{ label: 'TV Shows', to: '/tvshows' }]" :current="show.title" fallback="/tvshows" />
    <div
      v-if="continueWatching"
      class="mb-8 flex cursor-pointer items-center gap-5 rounded-xl border border-border bg-bg-alt p-4 transition-colors hover:bg-bg-elevated"
      @click="playEpisode(continueWatching.episode.id, false)"
    >
      <img
        v-if="continueWatching.episode.still_url"
        :src="continueWatching.episode.still_url"
        :alt="continueWatching.episode.title"
        class="w-32 shrink-0 rounded-lg sm:w-44"
      />
      <div class="flex flex-1 flex-col gap-1.5">
        <span class="text-xs font-bold tracking-wide text-accent uppercase">Continue Watching</span>
        <strong>
          S{{ continueWatching.seasonNumber }}E{{ continueWatching.episode.episode_number }} ·
          {{ continueWatching.episode.title }}
        </strong>
        <div
          v-if="continueWatching.episode.progress && !continueWatching.episode.progress.completed"
          class="h-1 w-full max-w-[320px] overflow-hidden rounded-full bg-border"
        >
          <div
            class="h-full bg-accent"
            :style="{
              width:
                (continueWatching.episode.progress.position_seconds /
                  continueWatching.episode.progress.duration_seconds) *
                  100 +
                '%',
            }"
          />
        </div>
        <button class="btn-primary mt-1 w-fit" @click.stop="playEpisode(continueWatching.episode.id, false)">
          ▶ {{ playLabel(continueWatching.episode) }}
        </button>
      </div>
    </div>

    <div class="mb-8 flex flex-col gap-6 sm:flex-row sm:gap-8">
      <img v-if="posterUrl" :src="posterUrl" :alt="show.title" class="w-36 shrink-0 self-start rounded-lg sm:w-52" />
      <div class="min-w-0">
        <h1 class="text-2xl font-black tracking-tight sm:text-4xl">{{ show.title }}</h1>
        <p class="mt-1.5 mb-1 text-text-dim">
          {{ show.first_air_date }} · ⭐ {{ show.vote_average.toFixed(1) }} · {{ show.genres }}
        </p>
        <p v-if="show.creators" class="mb-2 text-sm text-text-dim">Created by {{ show.creators }}</p>
        <p class="max-w-[70ch]">{{ show.overview }}</p>
        <div class="mt-3 flex flex-wrap gap-2.5">
          <button
            v-if="auth.isAdmin"
            class="btn-secondary"
            :disabled="refreshing || replacing"
            title="Re-fetch this show's metadata and cover from TMDB, keeping the same match"
            @click="refreshMetadata"
          >
            {{ refreshing ? 'Refreshing…' : '⟳ Refresh Metadata' }}
          </button>
          <button
            v-if="auth.isAdmin"
            class="btn-secondary"
            :disabled="refreshing || replacing"
            title="Delete and re-discover this show, its seasons, and episodes from scratch (a new TMDB search) — fixes a wrong match"
            @click="replaceMetadata"
          >
            {{ replacing ? 'Rescanning…' : '⟲ Full Rescan' }}
          </button>
        </div>
        <p v-if="refreshError" class="mt-3 rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
          {{ refreshError }}
        </p>
        <p v-if="replaceError" class="mt-3 rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
          {{ replaceError }}
        </p>
      </div>
    </div>

    <CastList :cast="show.cast" />

    <div v-for="season in show.seasons" :key="season.id" class="mt-8">
      <h2 class="mb-3 text-lg font-black tracking-tight">Season {{ season.season_number }}</h2>
      <ul class="m-0 flex list-none flex-col gap-3 p-0">
        <li
          v-for="ep in season.episodes"
          :key="ep.id"
          class="flex items-center gap-4 rounded-lg p-2 hover:bg-white/5"
          :class="{ 'opacity-60': ep.progress?.completed }"
        >
          <div class="relative shrink-0 cursor-pointer" @click="playEpisode(ep.id, !!ep.progress?.completed)">
            <img
              v-if="ep.still_url"
              :src="ep.still_url"
              :alt="ep.title"
              class="block h-fit w-36 rounded-md sm:w-40"
            />
            <div
              v-if="ep.progress?.completed"
              title="Watched"
              class="absolute top-1.5 right-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-accent text-[0.85rem] font-bold text-white shadow-[0_1px_4px_rgb(0_0_0/0.5)]"
            >
              ✓
            </div>
          </div>
          <div class="min-w-0 flex-1 cursor-pointer" @click="playEpisode(ep.id, !!ep.progress?.completed)">
            <strong>{{ ep.episode_number }}. {{ ep.title }}</strong>
            <span v-if="ep.runtime_minutes" class="ml-2 text-[0.85rem] text-text-dim">{{
              formatRuntime(ep.runtime_minutes)
            }}</span>
            <RemuxStatusBadge :status="ep.remux_status" class="ml-2" />
            <p class="text-text-dim">{{ ep.overview }}</p>
          </div>
          <div class="flex shrink-0 flex-col items-end gap-1.5">
            <button class="btn-primary px-3 py-1.5 text-[0.85rem] whitespace-nowrap" @click.stop="playEpisode(ep.id, false)">
              ▶ {{ playLabel(ep) }}
            </button>
            <button
              v-if="ep.progress"
              class="btn-secondary px-3 py-1.5 text-[0.85rem] whitespace-nowrap"
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
