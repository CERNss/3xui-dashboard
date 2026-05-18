<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAdminAuthStore } from '@/stores/adminAuth'

const router = useRouter()
const auth = useAdminAuthStore()

function logout() {
  auth.clear()
  router.push({ name: 'admin.login' })
}
</script>

<template>
  <div class="flex h-full">
    <aside class="w-60 border-r border-surface-200 bg-surface-0 px-4 py-6 dark:border-surface-800 dark:bg-surface-900">
      <div class="mb-8 text-lg font-semibold text-primary-700 dark:text-primary-300">
        {{ $t('app.title') }}
      </div>
      <nav class="space-y-1 text-sm">
        <router-link
          to="/admin/dashboard"
          class="block rounded px-3 py-2 hover:bg-surface-100 dark:hover:bg-surface-800"
          active-class="bg-primary-50 text-primary-700 dark:bg-surface-800 dark:text-primary-300"
        >
          {{ $t('nav.dashboard') }}
        </router-link>
      </nav>
    </aside>

    <main class="flex flex-1 flex-col">
      <header
        class="flex h-14 items-center justify-end border-b border-surface-200 bg-surface-0 px-6 dark:border-surface-800 dark:bg-surface-900"
      >
        <button
          class="text-sm text-surface-600 hover:text-surface-900 dark:text-surface-300 dark:hover:text-white"
          @click="logout"
        >
          {{ $t('nav.logout') }}
        </button>
      </header>
      <section class="flex-1 overflow-y-auto p-6">
        <router-view />
      </section>
    </main>
  </div>
</template>
