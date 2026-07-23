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
  <nav class="breadcrumbs">
    <button class="back-button" @click="goBack">‹ Back</button>
    <span class="trail">
      <template v-for="item in trail" :key="item.to">
        <RouterLink :to="item.to">{{ item.label }}</RouterLink>
        <span class="sep">/</span>
      </template>
      <span class="current">{{ current }}</span>
    </span>
  </nav>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
  font-size: 0.9rem;
}
.back-button {
  background: transparent;
  color: var(--text);
  border: 1px solid var(--border);
  padding: 0.35rem 0.75rem;
  font-size: 0.85rem;
  font-weight: 500;
}
.trail {
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.trail a {
  color: var(--text-dim);
  text-decoration: none;
}
.trail a:hover {
  color: var(--text);
  text-decoration: underline;
}
.sep {
  margin: 0 0.4rem;
}
.current {
  color: var(--text);
  font-weight: 600;
}
</style>
