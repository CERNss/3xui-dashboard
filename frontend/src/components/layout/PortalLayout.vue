<script setup lang="ts">
import { useRouter } from 'vue-router'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { useThemeStore } from '@/stores/theme'

const router = useRouter()
const auth = usePortalAuthStore()
const theme = useThemeStore()

function logout() {
  auth.clear()
  router.push({ name: 'login', query: { hint: 'portal' } })
}
</script>

<template>
  <div class="flex h-full flex-col bg-surface-50 dark:bg-surface-950">
    <header
      class="flex h-14 items-center justify-between border-b border-surface-100 bg-surface-0 px-6 dark:border-surface-800 dark:bg-surface-900"
    >
      <div class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">
        {{ $t('app.title') }}
      </div>
      <nav class="flex items-center gap-2 text-sm">
        <router-link
          to="/portal/dashboard"
          class="rounded-lg px-3 py-1.5 font-medium text-surface-600 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
          active-class="!text-accent-700 dark:!text-accent-300"
        >
          {{ $t('nav.dashboard') }}
        </router-link>
        <button
          :title="theme.theme === 'dark' ? '切换浅色' : '切换深色'"
          class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50"
          @click="theme.toggle()"
        >
          <svg v-if="theme.theme === 'dark'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
          </svg>
          <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
          </svg>
        </button>
        <button
          class="rounded-lg px-3 py-1.5 font-medium text-surface-600 transition-colors hover:bg-red-50 hover:text-red-600 dark:text-surface-300 dark:hover:bg-red-950/40 dark:hover:text-red-400"
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
