<script setup lang="ts">
import type { CastMember } from '../types'

defineProps<{
  cast?: CastMember[]
}>()
</script>

<template>
  <section v-if="cast?.length" class="cast-section">
    <h2>Cast</h2>
    <div class="cast-row">
      <div v-for="member in cast" :key="member.name + member.character" class="cast-member">
        <div class="headshot-wrap">
          <img v-if="member.profile_url" :src="member.profile_url" :alt="member.name" />
          <div v-else class="headshot-placeholder">{{ member.name.charAt(0) }}</div>
        </div>
        <div class="name">{{ member.name }}</div>
        <div v-if="member.character" class="character">{{ member.character }}</div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.cast-section {
  margin-top: 2rem;
}
.cast-row {
  display: flex;
  gap: 1.25rem;
  overflow-x: auto;
  padding-bottom: 0.5rem;
}
.cast-member {
  flex: 0 0 auto;
  width: 110px;
  text-align: center;
}
.headshot-wrap {
  width: 110px;
  height: 110px;
  border-radius: 50%;
  overflow: hidden;
  background: var(--bg-alt);
  margin-bottom: 0.5rem;
}
.headshot-wrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.headshot-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2rem;
  font-weight: 600;
  opacity: 0.5;
}
.name {
  font-size: 0.85rem;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.character {
  font-size: 0.8rem;
  opacity: 0.65;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
