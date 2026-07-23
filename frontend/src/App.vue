<script setup lang="ts">
import { useRoute } from 'vue-router'
import AppNav from './components/AppNav.vue'

const route = useRoute()
</script>

<template>
  <AppNav />
  <main class="container">
    <!-- Keyed on the full path so navigating between two routes that map to
         the same component (e.g. two different movies' player pages) always
         forces a fresh mount instead of Vue Router's default of reusing the
         instance — the player in particular relies on onMounted/onBeforeUnmount
         firing for every distinct video. -->
    <RouterView :key="route.fullPath" />
  </main>
</template>
