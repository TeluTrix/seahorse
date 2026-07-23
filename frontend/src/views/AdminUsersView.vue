<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api/client'
import type { PublicUser } from '../types'

const users = ref<PublicUser[]>([])
const loading = ref(true)
const editingId = ref<string | null>(null)
const newPassword = ref('')
const errors = reactive<Record<string, string>>({})
const successId = ref<string | null>(null)

const showCreateForm = ref(false)
const newUserEmail = ref('')
const newUserPassword = ref('')
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
    await api.createUser(newUserEmail.value, newUserPassword.value)
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
  <div class="admin">
    <h1>Users</h1>

    <div v-if="!showCreateForm">
      <button @click="startCreate">+ Create user</button>
    </div>
    <form v-else class="create-form" @submit.prevent="createUser">
      <input v-model="newUserEmail" type="email" placeholder="Email" required autocomplete="off" />
      <input v-model="newUserPassword" type="password" placeholder="Password (min. 8 characters)" required />
      <button type="submit" :disabled="creating">{{ creating ? 'Creating…' : 'Create' }}</button>
      <button type="button" class="secondary" @click="cancelCreate">Cancel</button>
    </form>
    <p v-if="createError" class="error-message">{{ createError }}</p>

    <div v-if="loading" class="spinner" />
    <table v-else class="users-table">
      <thead>
        <tr>
          <th>Email</th>
          <th>Role</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in users" :key="u.user_id">
          <td>{{ u.user_email }}</td>
          <td>{{ u.user_role }}</td>
          <td class="edit-cell">
            <template v-if="editingId === u.user_id">
              <input v-model="newPassword" type="password" placeholder="New password" />
              <button @click="savePassword(u.user_id)">Save</button>
              <button class="secondary" @click="cancelEdit">Cancel</button>
            </template>
            <template v-else>
              <span v-if="successId === u.user_id" class="success">Password updated</span>
              <button class="secondary" @click="startEdit(u.user_id)">Change password</button>
            </template>
            <p v-if="errors[u.user_id]" class="error-message">{{ errors[u.user_id] }}</p>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.admin {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.create-form {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.create-form input {
  width: 220px;
}
.users-table {
  width: 100%;
  border-collapse: collapse;
}
.users-table th,
.users-table td {
  text-align: left;
  padding: 0.6rem 0.75rem;
  border-bottom: 1px solid var(--border);
}
.edit-cell {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.edit-cell input {
  width: 180px;
}
.success {
  color: var(--accent);
  font-size: 0.85rem;
}
</style>
