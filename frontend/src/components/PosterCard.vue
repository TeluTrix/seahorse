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
  <div
    class="group cursor-pointer rounded-xl bg-bg-alt transition-transform duration-200 ease-out hover:scale-[1.04]"
    @click="$emit('click')"
  >
    <div
      class="relative aspect-[2/3] overflow-hidden rounded-xl bg-bg-elevated ring-1 ring-white/10 transition-shadow duration-200 group-hover:shadow-[0_0_0_2px_var(--color-accent),0_16px_32px_rgb(0_0_0/0.5)]"
    >
      <img
        v-if="posterUrl"
        :src="posterUrl"
        :alt="title"
        class="block h-full w-full object-cover"
      />
      <div
        v-if="watched"
        title="Watched"
        class="absolute top-1.5 right-1.5 flex h-6 w-6 items-center justify-center rounded-full bg-accent text-[0.85rem] font-bold text-white shadow-[0_1px_4px_rgb(0_0_0/0.5)]"
      >
        ✓
      </div>
    </div>
    <div class="mt-2.5 px-0.5 text-[0.9rem] font-bold">
      {{ title }}
      <span v-if="year" class="ml-1 text-[0.85em] font-semibold text-text-dim">{{ year }}</span>
    </div>
  </div>
</template>
