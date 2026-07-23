<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { streamURL } from '../api/client'

const route = useRoute()
const src = computed(() =>
  streamURL(route.name === 'watch-movie' ? 'movies' : 'episodes', route.params.id as string),
)
</script>

<template>
  <div class="player">
    <video :src="src" controls autoplay class="video" />
  </div>
</template>

<style scoped>
.player {
  display: flex;
  justify-content: center;
}
.video {
  width: 100%;
  max-height: 80vh;
  background: #000;
}
</style>
