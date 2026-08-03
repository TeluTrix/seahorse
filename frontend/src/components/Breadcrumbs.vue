<script setup lang="ts">
import { useRouter } from 'vue-router'

const props = defineProps<{
  // The trail excluding the current page, e.g. [{ label: 'Movies', to: '/movies' }].
  // The current page's own title is rendered last, non-clickable.
  trail: { label: string; to: string }[]
  current: string
  // Where "Back" goes if there's no in-app history to go back to (e.g. a
  // direct link opened in a new tab) — normally the overview page.
  fallback: string
}>()

const router = useRouter()

function goBack() {
  // history-based rather than always the fallback: the user could have
  // arrived here from Home, Search, or the overview page, and "back" should
  // return wherever they actually came from.
  if (window.history.state?.back) {
    router.back()
  } else {
    router.push(props.fallback)
  }
}
</script>

<template>
  <nav class="mb-4 flex items-center gap-4 text-sm">
    <button
      class="shrink-0 rounded-full border border-border bg-transparent px-3.5 py-1.5 text-[0.85rem] font-semibold text-text hover:border-white/30"
      @click="goBack"
    >
      ‹ Back
    </button>
    <span class="overflow-hidden text-ellipsis whitespace-nowrap text-text-dim">
      <template v-for="item in trail" :key="item.to">
        <RouterLink :to="item.to" class="text-text-dim no-underline hover:text-text hover:underline">{{
          item.label
        }}</RouterLink>
        <span class="mx-1.5">/</span>
      </template>
      <span class="font-bold text-text">{{ current }}</span>
    </span>
  </nav>
</template>
