<script setup lang="ts">
import type { CastMember } from '../types'

defineProps<{
  cast?: CastMember[]
}>()
</script>

<template>
  <section v-if="cast?.length" class="mt-8">
    <h2 class="mb-3 text-lg font-black tracking-tight">Cast</h2>
    <div class="flex gap-5 overflow-x-auto pb-2">
      <div
        v-for="member in cast"
        :key="member.name + member.character"
        class="w-[110px] shrink-0 text-center"
      >
        <div class="mb-2 h-[110px] w-[110px] overflow-hidden rounded-full bg-bg-alt">
          <img
            v-if="member.profile_url"
            :src="member.profile_url"
            :alt="member.name"
            class="block h-full w-full object-cover"
          />
          <div v-else class="flex h-full w-full items-center justify-center text-3xl font-bold opacity-50">
            {{ member.name.charAt(0) }}
          </div>
        </div>
        <div class="overflow-hidden text-ellipsis whitespace-nowrap text-[0.85rem] font-semibold">
          {{ member.name }}
        </div>
        <div
          v-if="member.character"
          class="overflow-hidden text-ellipsis whitespace-nowrap text-[0.8rem] opacity-65"
        >
          {{ member.character }}
        </div>
      </div>
    </div>
  </section>
</template>
