<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'
import type { PublicUser, Role } from '../types'

const users = ref<PublicUser[]>([])
const loading = ref(true)
const editingId = ref<string | null>(null)
const newPassword = ref('')
const errors = reactive<Record<string, string>>({})
const successId = ref<string | null>(null)

const showCreateForm = ref(false)
const newUserEmail = ref('')
const newUserPassword = ref('')
const newUserRole = ref<Role>('user')
const createError = ref('')
const creating = ref(false)

async function load() {
  loading.value = true
  try {
    users.value = await api.listUsers()
  } finally {
    loading.value = false
  }
}

onMounted(load)

function startCreate() {
  showCreateForm.value = true
  newUserEmail.value = ''
  newUserPassword.value = ''
  newUserRole.value = 'user'
  createError.value = ''
}

function cancelCreate() {
  showCreateForm.value = false
}

async function createUser() {
  createError.value = ''
  if (newUserPassword.value.length < 8) {
    createError.value = 'Password must be at least 8 characters'
    return
  }
  creating.value = true
  try {
    await api.createUser(newUserEmail.value, newUserPassword.value, newUserRole.value)
    showCreateForm.value = false
    await load()
  } catch (e) {
    createError.value = e instanceof Error ? e.message : 'could not create user'
  } finally {
    creating.value = false
  }
}

function startEdit(userId: string) {
  editingId.value = userId
  newPassword.value = ''
  errors[userId] = ''
  successId.value = null
}

function cancelEdit() {
  editingId.value = null
  newPassword.value = ''
}

async function savePassword(userId: string) {
  errors[userId] = ''
  if (newPassword.value.length < 8) {
    errors[userId] = 'Password must be at least 8 characters'
    return
  }
  try {
    await api.setUserPassword(userId, newPassword.value)
    successId.value = userId
    editingId.value = null
    newPassword.value = ''
  } catch (e) {
    errors[userId] = e instanceof Error ? e.message : 'could not update password'
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-3xl font-black tracking-tight">Users</h1>

    <div v-if="!showCreateForm">
      <button class="btn-primary" @click="startCreate">+ Create user</button>
    </div>
    <form v-else class="flex flex-wrap items-center gap-2" @submit.prevent="createUser">
      <input v-model="newUserEmail" type="email" placeholder="Email" required autocomplete="off" class="field w-[220px]" />
      <input v-model="newUserPassword" type="password" placeholder="Password (min. 8 characters)" required class="field w-auto" />
      <select v-model="newUserRole" class="field w-auto cursor-pointer">
        <option value="user">User</option>
        <option value="admin">Admin</option>
      </select>
      <button type="submit" :disabled="creating" class="btn-primary">{{ creating ? 'Creating…' : 'Create' }}</button>
      <button type="button" class="btn-secondary" @click="cancelCreate">Cancel</button>
    </form>
    <p v-if="createError" class="rounded-lg border border-danger bg-danger/10 px-3.5 py-2.5 text-sm text-danger">
      {{ createError }}
    </p>

    <div v-if="loading" class="spinner" />
    <table v-else class="w-full border-collapse">
      <thead>
        <tr>
          <th class="border-b border-border px-3 py-2.5 text-left">Email</th>
          <th class="border-b border-border px-3 py-2.5 text-left">Role</th>
          <th class="border-b border-border px-3 py-2.5 text-left"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in users" :key="u.user_id">
          <td class="border-b border-border px-3 py-2.5">{{ u.user_email }}</td>
          <td class="border-b border-border px-3 py-2.5">{{ u.user_role }}</td>
          <td class="border-b border-border px-3 py-2.5">
            <div class="flex items-center gap-2">
              <template v-if="editingId === u.user_id">
                <input v-model="newPassword" type="password" placeholder="New password" class="field w-[180px]" />
                <button class="btn-primary" @click="savePassword(u.user_id)">Save</button>
                <button class="btn-secondary" @click="cancelEdit">Cancel</button>
              </template>
              <template v-else>
                <span v-if="successId === u.user_id" class="text-[0.85rem] text-accent2">Password updated</span>
                <button class="btn-secondary" @click="startEdit(u.user_id)">Change password</button>
              </template>
            </div>
            <p v-if="errors[u.user_id]" class="mt-2 rounded-lg border border-danger bg-danger/10 px-3.5 py-2 text-sm text-danger">
              {{ errors[u.user_id] }}
            </p>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
