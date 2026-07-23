<script setup lang="ts">
defineProps<{
  title: string
  posterUrl: string
  year?: string
  watched?: boolean
}>()

defineEmits<{ click: [] }>()
</script>

<template>
  <div class="card" @click="$emit('click')">
    <div class="poster-wrap">
      <img v-if="posterUrl" :src="posterUrl" :alt="title" />
      <div v-if="watched" class="watched-badge" title="Watched">✓</div>
    </div>
    <div class="card-title">
      {{ title }}
      <span v-if="year" class="year">{{ year }}</span>
    </div>
  </div>
</template>

<style scoped>
.poster-wrap {
  position: relative;
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
.year {
  opacity: 0.6;
  font-weight: 400;
  font-size: 0.85em;
  margin-left: 0.25rem;
}
</style>
