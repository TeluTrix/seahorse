<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

const auth = useAuthStore()
const router = useRouter()

async function handleSubmit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    router.push({ name: 'home' })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'login failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-form">
    <h1>Log in</h1>
    <form @submit.prevent="handleSubmit">
      <label for="email">Email</label>
      <input id="email" v-model="email" type="email" autocomplete="email" required />
      <label for="password">Password</label>
      <input id="password" v-model="password" type="password" autocomplete="current-password" required />
      <p v-if="error" class="error-message">{{ error }}</p>
      <button type="submit" :disabled="loading">{{ loading ? 'Logging in…' : 'Log in' }}</button>
    </form>
    <p>No account yet? <RouterLink to="/register">Register</RouterLink></p>
  </div>
</template>

<style scoped>
.auth-form {
  max-width: 360px;
  margin: 4rem auto;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.auth-form form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
