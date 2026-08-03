<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useConfigStore } from '../stores/config'

const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

const auth = useAuthStore()
const config = useConfigStore()
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
  <div
    class="mx-auto mt-16 flex max-w-[380px] flex-col gap-6 rounded-2xl border border-white/10 bg-bg-alt p-8 shadow-[0_20px_60px_rgb(0_0_0/0.4)]"
  >
    <h1 class="text-3xl font-black tracking-tight">Log in</h1>
    <form class="flex flex-col gap-3" @submit.prevent="handleSubmit">
      <label for="email" class="text-sm text-text-dim">Email</label>
      <input id="email" v-model="email" type="email" autocomplete="email" required class="field" />
      <label for="password" class="text-sm text-text-dim">Password</label>
      <input
        id="password"
        v-model="password"
        type="password"
        autocomplete="current-password"
        required
        class="field"
      />
      <p v-if="error" class="rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
        {{ error }}
      </p>
      <button type="submit" :disabled="loading" class="btn-primary mt-1">
        {{ loading ? 'Logging in…' : 'Log in' }}
      </button>
    </form>
    <p v-if="config.registrationEnabled" class="text-sm text-text-dim">
      No account yet? <RouterLink to="/register" class="font-semibold text-accent2 hover:underline">Register</RouterLink>
    </p>
  </div>
</template>
