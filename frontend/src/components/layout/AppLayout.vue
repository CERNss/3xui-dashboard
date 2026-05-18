<template>
  <div class="min-h-screen flex bg-gray-50">
    <AppSidebar />

    <!-- Main content: offset by sidebar width on large screens -->
    <div class="flex-1 flex flex-col min-w-0 lg:pl-64">
      <AppHeader />
      <main class="flex-1 p-4 lg:p-6 overflow-auto">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
</script>
