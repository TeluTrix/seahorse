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
    await auth.register(email.value, password.value)
    router.push({ name: 'home' })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'registration failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-form">
    <h1>Create account</h1>
    <form @submit.prevent="handleSubmit">
      <label for="email">Email</label>
      <input id="email" v-model="email" type="email" autocomplete="email" required />
      <label for="password">Password (min. 8 characters)</label>
      <input id="password" v-model="password" type="password" autocomplete="new-password" required minlength="8" />
      <p v-if="error" class="error-message">{{ error }}</p>
      <button type="submit" :disabled="loading">{{ loading ? 'Registering…' : 'Register' }}</button>
    </form>
    <p>Already have an account? <RouterLink to="/login">Log in</RouterLink></p>
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
