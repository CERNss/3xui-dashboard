<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import AccountMenu, { type AccountMenuItem } from '@/components/common/AccountMenu.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { useBrandingStore } from '@/stores/branding'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { useThemeStore } from '@/stores/theme'
import { formatError } from '@/utils/format'

const router = useRouter()
const { t } = useI18n()
const auth = usePortalAuthStore()
const branding = useBrandingStore()
const theme = useThemeStore()
const profile = ref<UserProfile | null>(null)
const profileError = ref<string | null>(null)
const REPO_URL = 'https://github.com/cern/3xui-dashboard'

const navItems = computed(() => [
  { to: '/portal/subscription', label: 'nav.subscription', icon: 'M4 5h16v14H4zM9 9h6v6H9z' },
  { to: '/portal/usage', label: 'nav.usage', icon: 'M4 19V5M4 19h16M8 15v-4M12 15V8M16 15v-7' },
  { to: '/portal/plans', label: 'nav.plans', icon: 'M4 7h16M6 7V5h12v2M6 7l1 12h10l1-12M9 11h6' },
  { to: '/portal/orders', label: 'nav.orders', icon: 'M6 3h12v18l-3-2-3 2-3-2-3 2V3zM9 8h6M9 12h6' },
  { to: '/portal/profile', label: 'nav.profile', icon: 'M20 21a8 8 0 0 0-16 0M12 13a5 5 0 1 0 0-10 5 5 0 0 0 0 10z' },
])

const displayEmail = computed(() => profile.value?.email || auth.user?.email || '')
const accountName = computed(() => {
  const email = displayEmail.value
  if (!email) return 'AI'
  return email.split('@')[0] || email
})
const accountMenuItems = computed<AccountMenuItem[]>(() => [
  {
    label: t('account.profile'),
    to: '/portal/profile',
    icon: 'M20 21a8 8 0 0 0-16 0M12 13a5 5 0 1 0 0-10 5 5 0 0 0 0 10z',
  },
  {
    label: t('nav.subscription'),
    to: '/portal/subscription',
    icon: 'M4 5h16v3H4zM4 11h16v3H4zM4 17h10v3H4z',
  },
  {
    label: t('account.github'),
    href: REPO_URL,
    icon: 'M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22',
  },
  {
    label: t('account.guide'),
    href: `${REPO_URL}#readme`,
    icon: 'M12 20h9M12 4h9M4 19.5A2.5 2.5 0 0 1 6.5 17H21M4 4.5A2.5 2.5 0 0 1 6.5 2H21v15H6.5A2.5 2.5 0 0 0 4 19.5v-15z',
  },
])

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

async function loadProfile() {
  profileError.value = null
  try {
    profile.value = await portalProfileApi.get()
  } catch (e) {
    profileError.value = formatError(e, '')
  }
}

function logout() {
  auth.clear()
  router.push({ name: 'login', query: { next: '/portal' } })
}

onMounted(loadProfile)
</script>

<template>
  <div class="app-dark-bg flex h-full flex-col bg-surface-50 dark:bg-surface-950">
    <header
      class="app-dark-header flex min-h-14 items-center justify-between gap-3 border-b border-surface-100 bg-surface-0 px-4 py-2 dark:border-surface-700/80 sm:px-6"
    >
      <div class="min-w-0 shrink-0">
        <div class="flex items-center gap-2 text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">
          <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-accent-500 text-white">
            <img v-if="branding.iconUrl" :src="branding.iconUrl" alt="" class="h-5 w-5 rounded-md object-cover" @error="branding.clearIcon()" />
            <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor" stroke="none">
              <path d="M13 2 3 14h7l-1 8 11-13h-7l0-7z" />
            </svg>
          </span>
          <span>{{ branding.title || $t('app.title') }}</span>
        </div>
        <div v-if="displayEmail" class="mt-0.5 hidden max-w-[220px] truncate text-2xs text-surface-500 sm:block">{{ displayEmail }}</div>
      </div>
      <div class="ml-auto flex min-w-0 items-center justify-end gap-2">
        <div v-if="profile" class="hidden items-center gap-2 rounded-xl border border-surface-200 bg-surface-50 px-3 py-1.5 dark:border-surface-700 dark:bg-surface-800 md:flex">
          <div>
            <div class="text-2xs text-surface-500">{{ $t('portal.shell.balance') }}</div>
            <div class="font-mono text-sm font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ formatYuan(profile.balance_cents) }}</div>
          </div>
          <router-link
            to="/portal/plans"
            class="inline-flex h-8 items-center rounded-lg bg-ink-900 px-3 text-xs font-semibold text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
          >
            {{ $t('portal.shell.renew') }}
          </router-link>
        </div>
        <div v-else-if="profileError" class="hidden max-w-[220px] truncate text-2xs text-red-500 md:block">{{ profileError }}</div>
        <nav class="hidden min-w-0 flex-1 items-center justify-end gap-1 overflow-x-auto text-sm lg:flex">
        <router-link
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="shrink-0 rounded-lg px-2.5 py-1.5 font-medium text-surface-600 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50 sm:px-3"
          active-class="!text-accent-700 dark:!text-accent-300"
        >
          {{ $t(item.label) }}
        </router-link>
        <span class="mx-1 hidden h-5 w-px shrink-0 bg-surface-200 dark:bg-surface-700 sm:block"></span>
        </nav>
        <LocaleSwitcher variant="toolbar" />
        <button
          :title="theme.theme === 'dark' ? $t('theme.toggleLight') : $t('theme.toggleDark')"
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50"
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
        <!-- Phone: text label hidden, only icon visible; sm+: text -->
        <AccountMenu
          :name="accountName"
          :subtitle="displayEmail"
          :role-label="$t('account.userRole')"
          :avatar-text="accountName"
          :open-label="$t('account.openMenu')"
          :logout-label="$t('nav.logout')"
          :items="accountMenuItems"
          @logout="logout"
        />
      </div>
    </header>
    <section class="min-h-0 flex-1 overflow-y-auto p-4 pb-24 sm:p-6 lg:pb-6">
      <router-view />
    </section>
    <nav class="fixed inset-x-0 bottom-0 z-40 border-t border-surface-100 bg-surface-0/95 px-2 py-2 shadow-elevated backdrop-blur dark:border-surface-800 dark:bg-surface-900/95 lg:hidden">
      <div class="mx-auto grid max-w-md grid-cols-5 gap-1">
        <router-link
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="flex min-w-0 flex-col items-center gap-1 rounded-xl px-1.5 py-1.5 text-2xs font-medium text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-50"
          active-class="!bg-accent-50 !text-accent-700 dark:!bg-accent-950/40 dark:!text-accent-300"
        >
          <svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
            <path :d="item.icon" />
          </svg>
          <span class="max-w-full truncate">{{ $t(item.label) }}</span>
        </router-link>
      </div>
    </nav>
  </div>
</template>
