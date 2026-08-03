<script setup lang="ts"></script>

<template>
  <div class="flex items-start gap-8">
    <nav class="flex w-40 shrink-0 flex-col gap-1">
      <RouterLink
        to="/admin/library"
        class="rounded-lg px-3.5 py-2.5 text-text-dim no-underline hover:bg-bg-alt hover:text-text [&.router-link-active]:bg-bg-alt [&.router-link-active]:font-semibold [&.router-link-active]:text-text"
        >Library</RouterLink
      >
      <RouterLink
        to="/admin/users"
        class="rounded-lg px-3.5 py-2.5 text-text-dim no-underline hover:bg-bg-alt hover:text-text [&.router-link-active]:bg-bg-alt [&.router-link-active]:font-semibold [&.router-link-active]:text-text"
        >Users</RouterLink
      >
    </nav>
    <div class="min-w-0 flex-1">
      <RouterView />
    </div>
  </div>
</template>
