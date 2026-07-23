<script setup lang="ts">
defineProps<{
  title: string
  posterUrl: string
  year?: string
}>()

defineEmits<{ click: [] }>()
</script>

<template>
  <div class="card" @click="$emit('click')">
    <img v-if="posterUrl" :src="posterUrl" :alt="title" />
    <div class="card-title">
      {{ title }}
      <span v-if="year" class="year">{{ year }}</span>
    </div>
  </div>
</template>

<style scoped>
.year {
  opacity: 0.6;
  font-weight: 400;
  font-size: 0.85em;
  margin-left: 0.25rem;
}
</style>
