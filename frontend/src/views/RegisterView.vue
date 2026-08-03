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
  <div
    class="mx-auto mt-16 flex max-w-[380px] flex-col gap-6 rounded-2xl border border-white/10 bg-bg-alt p-8 shadow-[0_20px_60px_rgb(0_0_0/0.4)]"
  >
    <h1 class="text-3xl font-black tracking-tight">Create account</h1>
    <form class="flex flex-col gap-3" @submit.prevent="handleSubmit">
      <label for="email" class="text-sm text-text-dim">Email</label>
      <input id="email" v-model="email" type="email" autocomplete="email" required class="field" />
      <label for="password" class="text-sm text-text-dim">Password (min. 8 characters)</label>
      <input
        id="password"
        v-model="password"
        type="password"
        autocomplete="new-password"
        required
        minlength="8"
        class="field"
      />
      <p v-if="error" class="rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
        {{ error }}
      </p>
      <button type="submit" :disabled="loading" class="btn-primary mt-1">
        {{ loading ? 'Registering…' : 'Register' }}
      </button>
    </form>
    <p class="text-sm text-text-dim">
      Already have an account? <RouterLink to="/login" class="font-semibold text-accent2 hover:underline">Log in</RouterLink>
    </p>
  </div>
</template>
