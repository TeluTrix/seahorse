<script setup lang="ts">
defineProps<{
  status?: 'pending' | 'active'
}>()
</script>

<template>
  <span
    v-if="status"
    class="inline-flex items-center gap-1.5 rounded-full border border-danger bg-danger/10 px-2.5 py-1 text-[0.8rem] font-semibold whitespace-nowrap text-danger"
  >
    <span v-if="status === 'active'" class="h-1.5 w-1.5 animate-pulse rounded-full bg-danger" />
    {{ status === 'active' ? 'Audio fix in progress' : 'Audio fix queued' }}
  </span>
</template>
