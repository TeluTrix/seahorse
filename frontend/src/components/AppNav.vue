<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useConfigStore } from '../stores/config'
import { APP_VERSION } from '../version'

const auth = useAuthStore()
const config = useConfigStore()
const router = useRouter()
const route = useRoute()
const searchQuery = ref('')
const drawerOpen = ref(false)

// Closing on every route change covers both link clicks inside the drawer
// and any programmatic navigation (e.g. submitting the search box).
watch(
  () => route.fullPath,
  () => {
    drawerOpen.value = false
  },
)

function handleLogout() {
  auth.logout()
  router.push({ name: 'login' })
}

function submitSearch() {
  const q = searchQuery.value.trim()
  if (!q) return
  router.push({ name: 'search', query: { q } })
}
</script>

<template>
  <nav
    class="flex items-center gap-7 border-b border-white/10 bg-bg px-4 py-3.5 text-white sm:px-8"
  >
    <RouterLink to="/" class="inline-flex shrink-0 items-center gap-2 no-underline">
      <img src="/logo.svg" alt="" class="h-7 w-auto" />
      <span
        class="bg-gradient-to-r from-white to-[#ffb199] bg-clip-text text-[1.25rem] font-black tracking-tight text-transparent"
        >seahorse</span
      >
      <span class="text-[0.7rem] font-normal text-white/40">v{{ APP_VERSION }}</span>
    </RouterLink>

    <template v-if="auth.isAuthenticated">
      <RouterLink
        to="/movies"
        class="hidden text-[0.95rem] font-bold text-white/75 no-underline hover:text-white md:inline [&.router-link-active]:text-accent2 [&.router-link-active]:opacity-100"
        >Movies</RouterLink
      >
      <RouterLink
        to="/tvshows"
        class="hidden text-[0.95rem] font-bold text-white/75 no-underline hover:text-white md:inline [&.router-link-active]:text-accent2 [&.router-link-active]:opacity-100"
        >TV Shows</RouterLink
      >
      <RouterLink
        to="/actors"
        class="hidden text-[0.95rem] font-bold text-white/75 no-underline hover:text-white md:inline [&.router-link-active]:text-accent2 [&.router-link-active]:opacity-100"
        >Actors</RouterLink
      >
    </template>

    <input
      v-if="auth.isAuthenticated"
      v-model="searchQuery"
      type="search"
      placeholder="Search movies & tv shows..."
      class="ml-2 hidden w-64 rounded-full border border-white/15 bg-white/8 px-4 py-2 text-sm text-white placeholder:text-white/50 md:block"
      @keyup.enter="submitSearch"
    />

    <div class="flex-1" />

    <template v-if="auth.isAuthenticated">
      <RouterLink
        v-if="auth.isAdmin"
        to="/admin"
        class="hidden text-[0.95rem] font-bold text-white/75 no-underline hover:text-white md:inline [&.router-link-active]:text-accent2 [&.router-link-active]:opacity-100"
        >Admin</RouterLink
      >
      <span class="hidden text-sm text-white/70 md:inline">{{ auth.user?.user_email }}</span>
      <button
        class="hidden rounded-lg border border-white/30 bg-transparent px-4 py-2 text-sm font-bold text-white md:inline-block"
        @click="handleLogout"
      >
        Logout
      </button>
    </template>
    <template v-else>
      <RouterLink
        to="/login"
        class="hidden text-[0.95rem] font-bold text-white/75 no-underline hover:text-white md:inline [&.router-link-active]:text-accent2 [&.router-link-active]:opacity-100"
        >Login</RouterLink
      >
      <RouterLink
        v-if="config.registrationEnabled"
        to="/register"
        class="hidden text-[0.95rem] font-bold text-white/75 no-underline hover:text-white md:inline [&.router-link-active]:text-accent2 [&.router-link-active]:opacity-100"
        >Register</RouterLink
      >
    </template>

    <button
      class="ml-1 flex h-9 w-9 items-center justify-center rounded-lg text-white md:hidden"
      aria-label="Menu"
      @click="drawerOpen = true"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" class="h-5 w-5">
        <path d="M3 6h18M3 12h18M3 18h18" />
      </svg>
    </button>
  </nav>

  <!-- Mobile slide-out drawer: backdrop + panel, both dismiss on click; the
       route-change watcher above additionally closes it on navigation. -->
  <Transition name="fade">
    <div
      v-if="drawerOpen"
      class="fixed inset-0 z-40 bg-black/60 md:hidden"
      @click="drawerOpen = false"
    />
  </Transition>
  <Transition name="slide">
    <div
      v-if="drawerOpen"
      class="fixed top-0 right-0 z-50 flex h-full w-72 max-w-[80vw] flex-col gap-1 bg-bg-alt p-5 shadow-2xl md:hidden"
    >
      <div class="mb-4 flex items-center justify-between">
        <span class="text-lg font-black tracking-tight">Menu</span>
        <button
          class="flex h-8 w-8 items-center justify-center rounded-lg text-white/70 hover:text-white"
          aria-label="Close menu"
          @click="drawerOpen = false"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" class="h-5 w-5">
            <path d="M6 6l12 12M18 6L6 18" />
          </svg>
        </button>
      </div>

      <input
        v-if="auth.isAuthenticated"
        v-model="searchQuery"
        type="search"
        placeholder="Search movies & tv shows..."
        class="mb-3 w-full rounded-full border border-white/15 bg-white/8 px-4 py-2.5 text-sm text-white placeholder:text-white/50"
        @keyup.enter="submitSearch"
      />

      <template v-if="auth.isAuthenticated">
        <RouterLink
          to="/movies"
          class="rounded-lg px-3 py-2.5 font-bold text-white/85 no-underline hover:bg-white/8 [&.router-link-active]:text-accent2"
          >Movies</RouterLink
        >
        <RouterLink
          to="/tvshows"
          class="rounded-lg px-3 py-2.5 font-bold text-white/85 no-underline hover:bg-white/8 [&.router-link-active]:text-accent2"
          >TV Shows</RouterLink
        >
        <RouterLink
          to="/actors"
          class="rounded-lg px-3 py-2.5 font-bold text-white/85 no-underline hover:bg-white/8 [&.router-link-active]:text-accent2"
          >Actors</RouterLink
        >
        <RouterLink
          v-if="auth.isAdmin"
          to="/admin"
          class="rounded-lg px-3 py-2.5 font-bold text-white/85 no-underline hover:bg-white/8 [&.router-link-active]:text-accent2"
          >Admin</RouterLink
        >
        <div class="mt-2 border-t border-white/10 pt-3">
          <div class="px-3 pb-3 text-sm text-white/60">{{ auth.user?.user_email }}</div>
          <button
            class="mx-3 rounded-lg border border-white/30 bg-transparent px-4 py-2 text-sm font-bold text-white"
            @click="handleLogout"
          >
            Logout
          </button>
        </div>
      </template>
      <template v-else>
        <RouterLink
          to="/login"
          class="rounded-lg px-3 py-2.5 font-bold text-white/85 no-underline hover:bg-white/8 [&.router-link-active]:text-accent2"
          >Login</RouterLink
        >
        <RouterLink
          v-if="config.registrationEnabled"
          to="/register"
          class="rounded-lg px-3 py-2.5 font-bold text-white/85 no-underline hover:bg-white/8 [&.router-link-active]:text-accent2"
          >Register</RouterLink
        >
      </template>
    </div>
  </Transition>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.25s ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
}
</style>
