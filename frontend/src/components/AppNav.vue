<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { APP_VERSION } from '../version'

const auth = useAuthStore()
const router = useRouter()
const searchQuery = ref('')

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
  <nav class="app-nav">
    <RouterLink to="/" class="brand">seahorse<span class="version">v{{ APP_VERSION }}</span></RouterLink>
    <input
      v-if="auth.isAuthenticated"
      v-model="searchQuery"
      type="search"
      placeholder="Search movies & tv shows..."
      class="nav-search"
      @keyup.enter="submitSearch"
    />
    <div class="spacer" />
    <template v-if="auth.isAuthenticated">
      <RouterLink v-if="auth.isAdmin" to="/admin">Admin</RouterLink>
      <span class="user-email">{{ auth.user?.user_email }}</span>
      <button class="secondary" @click="handleLogout">Logout</button>
    </template>
    <template v-else>
      <RouterLink to="/login">Login</RouterLink>
      <RouterLink to="/register">Register</RouterLink>
    </template>
  </nav>
</template>

<style scoped>
.app-nav {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  padding: 0.85rem 1.5rem;
  background: #1e1e2f;
  color: #fff;
}
.app-nav a {
  color: #fff;
  text-decoration: none;
  opacity: 0.85;
}
.app-nav a:hover {
  opacity: 1;
}
.spacer {
  flex: 1;
}
.brand {
  font-weight: 700;
  font-size: 1.15rem;
  opacity: 1 !important;
  display: inline-flex;
  align-items: baseline;
  gap: 0.4rem;
}
.version {
  font-weight: 400;
  font-size: 0.7rem;
  opacity: 0.55;
}
.nav-search {
  width: 260px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: #fff;
}
.nav-search::placeholder {
  color: rgba(255, 255, 255, 0.5);
}
.user-email {
  opacity: 0.7;
  font-size: 0.9rem;
}
button.secondary {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.3);
}
</style>
