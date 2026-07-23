<script setup lang="ts">
defineProps<{
  status?: 'pending' | 'active'
}>()
</script>

<template>
  <span v-if="status" class="remux-badge" :class="status">
    <span v-if="status === 'active'" class="spinner-dot" />
    {{ status === 'active' ? 'Audio fix in progress' : 'Audio fix queued' }}
  </span>
</template>

<style scoped>
.remux-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  font-weight: 600;
  padding: 0.25rem 0.6rem;
  border-radius: 999px;
  background: rgba(248, 113, 113, 0.12);
  color: var(--danger);
  border: 1px solid var(--danger);
  white-space: nowrap;
}
.spinner-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--danger);
  animation: pulse 1s ease-in-out infinite;
}
@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.3;
  }
}
</style>
