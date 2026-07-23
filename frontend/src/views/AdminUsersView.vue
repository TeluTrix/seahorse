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

async function load() {
  loading.value = true
  try {
    users.value = await api.listUsers()
  } finally {
    loading.value = false
  }
}

onMounted(load)

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
