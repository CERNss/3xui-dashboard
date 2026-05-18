<script setup lang="ts">
import { useRouter } from 'vue-router'
import { usePortalAuthStore } from '@/stores/portalAuth'

const router = useRouter()
const auth = usePortalAuthStore()

function logout() {
  auth.clear()
  router.push({ name: 'portal.login' })
}
</script>

<template>
  <div class="flex h-full flex-col">
    <header
      class="flex h-14 items-center justify-between border-b border-surface-200 bg-surface-0 px-6 dark:border-surface-800 dark:bg-surface-900"
    >
      <div class="text-base font-semibold text-primary-700 dark:text-primary-300">
        {{ $t('app.title') }}
      </div>
      <nav class="flex items-center gap-4 text-sm">
        <router-link
          to="/portal/dashboard"
          class="text-surface-600 hover:text-surface-900 dark:text-surface-300 dark:hover:text-white"
          active-class="text-primary-700 dark:text-primary-300"
        >
          {{ $t('nav.dashboard') }}
        </router-link>
        <button
          class="text-surface-600 hover:text-surface-900 dark:text-surface-300 dark:hover:text-white"
          @click="logout"
        >
          {{ $t('nav.logout') }}
        </button>
      </nav>
    </header>
    <section class="flex-1 overflow-y-auto p-6">
      <router-view />
    </section>
  </div>
</template>
