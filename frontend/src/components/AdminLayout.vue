<script setup lang="ts"></script>

<template>
  <div class="admin-layout">
    <nav class="sidebar">
      <RouterLink to="/admin/library" class="sidebar-link">Library</RouterLink>
      <RouterLink to="/admin/users" class="sidebar-link">Users</RouterLink>
    </nav>
    <div class="admin-content">
      <RouterView />
    </div>
  </div>
</template>

<style scoped>
.admin-layout {
  display: flex;
  gap: 2rem;
  align-items: flex-start;
}
.sidebar {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  width: 160px;
  flex-shrink: 0;
}
.sidebar-link {
  padding: 0.6rem 0.9rem;
  border-radius: 6px;
  text-decoration: none;
  color: var(--text-dim);
}
.sidebar-link:hover {
  background: var(--bg-alt);
  color: var(--text);
}
.sidebar-link.router-link-active {
  background: var(--bg-alt);
  color: var(--text);
  font-weight: 600;
}
.admin-content {
  flex: 1;
  min-width: 0;
}
</style>
