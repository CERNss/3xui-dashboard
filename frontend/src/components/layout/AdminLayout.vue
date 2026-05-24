<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AccountMenu, { type AccountMenuItem } from '@/components/common/AccountMenu.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { useBrandingStore } from '@/stores/branding'
import { useThemeStore } from '@/stores/theme'

const SIDEBAR_COLLAPSED_KEY = 'dashboard.admin.sidebar.collapsed'

function readSidebarCollapsed(): boolean {
  if (typeof window === 'undefined') return false
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true'
  } catch {
    return false
  }
}

const router = useRouter()
const route = useRoute()
const auth = useAdminAuthStore()
const branding = useBrandingStore()
const theme = useThemeStore()
const { t } = useI18n()
const REPO_URL = 'https://github.com/cern/3xui-dashboard'

// Mobile drawer state. Sidebar is always-on at md+ (md: visible),
// off-canvas + toggleable below that. Close on route change so a
// nav click doesn't leave the drawer hanging open.
const drawerOpen = ref(false)
watch(() => route.fullPath, () => { drawerOpen.value = false })
const sidebarCollapsed = ref(readSidebarCollapsed())

function toggleSidebarCollapsed() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  try {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, sidebarCollapsed.value ? 'true' : 'false')
  } catch {
    // Ignore storage failures; the current session still updates.
  }
}

function logout() {
  auth.clear()
  router.push({ name: 'login', query: { next: '/admin' } })
}

interface NavItem {
  to: string
  label: string
  icon: string
}
interface NavSection {
  title: string
  items: NavItem[]
}

// Inline SVG path bodies — heroicons-style, single-line.
// Locale-aware: rebuild whenever t() changes so labels swap on locale toggle.
const sections = computed<NavSection[]>(() => [
  {
    title: t('section.overview'),
    items: [
      {
        to: '/admin/status',
        label: t('nav.status'),
        icon: 'M3 12a9 9 0 1 0 18 0 9 9 0 0 0-18 0zM12 7v5l3 2',
      },
      {
        to: '/admin/stats',
        label: t('nav.stats'),
        icon: 'M3 17l6-6 4 4 8-8M14 7h7v7',
      },
    ],
  },
  {
    title: t('section.nodes'),
    items: [
      {
        to: '/admin/nodes',
        label: t('nav.nodes'),
        icon: 'M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01',
      },
      {
        to: '/admin/inbounds',
        label: t('nav.inbounds'),
        icon: 'M4 6h16M4 12h16M4 18h16',
      },
    ],
  },
  {
    title: t('section.users'),
    items: [
      {
        to: '/admin/users',
        label: t('nav.users'),
        icon: 'M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0',
      },
      {
        to: '/admin/plans',
        label: t('nav.plansAdmin'),
        icon: 'M9 11l3 3L22 4M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11',
      },
      {
        to: '/admin/provisioning-pools',
        label: t('nav.provisioningPools'),
        icon: 'M4 7h16M7 7v10a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2V7M9 11h6M9 15h6',
      },
      {
        to: '/admin/orders',
        label: t('nav.ordersAdmin'),
        icon: 'M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6',
      },
    ],
  },
  {
    title: t('section.system'),
    items: [
      {
        to: '/admin/audit-log',
        label: t('nav.audit'),
        icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5.586a1 1 0 0 1 .707.293l5.414 5.414a1 1 0 0 1 .293.707V19a2 2 0 0 1-2 2z',
      },
      {
        to: '/admin/settings',
        label: t('nav.settings'),
        icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 0 0-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 0 0-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 0 0-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 0 0-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 0 0 1.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
      },
    ],
  },
])

const accountName = computed(() => auth.username || 'admin')
const accountMenuItems = computed<AccountMenuItem[]>(() => [
  {
    label: t('account.profile'),
    to: '/admin/settings?tab=securityAuth',
    icon: 'M20 21a8 8 0 0 0-16 0M12 13a5 5 0 1 0 0-10 5 5 0 0 0 0 10z',
  },
  {
    label: t('nav.nodes'),
    to: '/admin/nodes',
    icon: 'M4 7l8-4 8 4M4 7v10l8 4 8-4V7M4 7l8 4 8-4M12 11v10',
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
</script>

<template>
  <div class="flex h-full bg-surface-50 dark:bg-surface-950">
    <!-- Mobile backdrop — covers content when drawer is open. -->
    <div
      v-if="drawerOpen"
      class="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm md:hidden"
      @click="drawerOpen = false"
    />

    <aside
      :class="[
        'flex w-64 flex-col border-r border-surface-100/80 bg-surface-0 px-4 pb-5 pt-6 dark:border-surface-800 dark:bg-surface-900',
        'md:relative md:translate-x-0 md:shadow-none',
        'fixed inset-y-0 left-0 z-40 shadow-elevated transition-[width,transform,padding] duration-200 ease-brand',
        sidebarCollapsed ? 'md:w-[76px] md:px-3' : 'md:w-64',
        drawerOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
      ]"
    >
      <!-- Brand -->
      <div
        :class="sidebarCollapsed ? 'md:justify-center md:px-0' : 'px-2'"
        class="mb-7 flex items-center gap-3"
      >
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-accent-500 to-accent-700 text-white shadow-card ring-1 ring-accent-700/30">
          <img v-if="branding.iconUrl" :src="branding.iconUrl" alt="" class="h-6 w-6 rounded-lg object-cover" @error="branding.clearIcon()" />
          <svg v-else class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z" />
          </svg>
        </div>
        <div :class="sidebarCollapsed ? 'md:hidden' : ''" class="leading-tight">
          <div class="text-body-md font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ branding.title || $t('app.title') }}</div>
          <div class="text-eyebrow uppercase tracking-eyebrow text-surface-400">{{ branding.subtitle || $t('brand.centralPanel') }}</div>
        </div>
      </div>

      <!-- Nav -->
      <nav class="flex-1 pt-1 text-sm">
        <div
          v-for="(section, sIdx) in sections"
          :key="sIdx"
          :class="sIdx === 0 ? '' : 'mt-6'"
          class="space-y-1 border-t border-surface-200/80 pt-3.5 dark:border-surface-700/70"
        >
          <div
            :class="sidebarCollapsed ? 'md:hidden' : ''"
            class="px-3 pb-1 text-eyebrow font-semibold uppercase tracking-eyebrow text-surface-500 dark:text-surface-400"
          >
            {{ section.title }}
          </div>
          <router-link
            v-for="item in section.items"
            :key="item.to"
            :to="item.to"
            :title="sidebarCollapsed ? item.label : undefined"
            :aria-label="item.label"
            :class="sidebarCollapsed ? 'md:justify-center md:gap-0 md:px-2 md:before:left-0' : 'gap-3 px-3'"
            class="group relative flex items-center rounded-xl py-2 font-medium text-surface-600 transition-all duration-150 ease-brand before:absolute before:inset-y-2 before:left-1 before:w-0.5 before:rounded-full before:bg-transparent hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
            active-class="!bg-surface-100 !text-accent-700 before:!bg-accent-500 dark:!bg-surface-800 dark:!text-accent-200 dark:before:!bg-accent-300"
            exact-active-class=""
          >
            <svg
              class="h-[18px] w-[18px] shrink-0 transition-transform duration-200 ease-brand group-hover:scale-105"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path :d="item.icon" />
            </svg>
            <span :class="sidebarCollapsed ? 'md:hidden' : ''">{{ item.label }}</span>
          </router-link>
        </div>
      </nav>

      <!-- Theme toggle — Sub2API pattern: labeled sidebar item, not a tiny icon button. -->
      <button
        type="button"
        :title="sidebarCollapsed ? (theme.theme === 'dark' ? $t('theme.light') : $t('theme.dark')) : undefined"
        :aria-label="theme.theme === 'dark' ? $t('theme.light') : $t('theme.dark')"
        :class="sidebarCollapsed ? 'md:justify-center md:gap-0 md:px-2' : 'gap-3 px-3'"
        class="group mt-3 flex w-full items-center rounded-xl py-2 text-sm font-medium text-surface-600 transition-all duration-150 ease-brand hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
        @click="theme.toggle()"
      >
        <!-- Show the icon for the mode you'd switch TO (Sub2API convention). -->
        <svg v-if="theme.theme === 'dark'" class="h-[18px] w-[18px] shrink-0 transition-transform duration-200 ease-brand group-hover:scale-105" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
        </svg>
        <svg v-else class="h-[18px] w-[18px] shrink-0 transition-transform duration-200 ease-brand group-hover:scale-105" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
        <span :class="sidebarCollapsed ? 'md:hidden' : ''">{{ theme.theme === 'dark' ? $t('theme.light') : $t('theme.dark') }}</span>
      </button>

      <button
        type="button"
        :title="sidebarCollapsed ? $t('nav.expandSidebar') : $t('nav.collapseSidebar')"
        :aria-label="sidebarCollapsed ? $t('nav.expandSidebar') : $t('nav.collapseSidebar')"
        :class="sidebarCollapsed ? 'md:justify-center md:gap-0 md:px-2' : 'gap-3 px-3'"
        class="group mt-3 flex w-full items-center rounded-xl border-t border-surface-100 pt-3 pb-2 text-sm font-medium text-surface-600 transition-colors hover:text-ink-900 dark:border-surface-800 dark:text-surface-300 dark:hover:text-surface-50"
        @click="toggleSidebarCollapsed"
      >
        <svg
          class="h-[18px] w-[18px] shrink-0 transition-transform duration-200 ease-brand"
          :class="sidebarCollapsed ? 'rotate-180' : ''"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="m11 17-5-5 5-5" />
          <path d="m18 17-5-5 5-5" />
        </svg>
        <span :class="sidebarCollapsed ? 'md:hidden' : ''">{{ $t('nav.collapseSidebar') }}</span>
      </button>

    </aside>

    <main class="flex flex-1 flex-col overflow-y-auto">
      <header class="hidden h-16 shrink-0 items-center justify-end border-b border-surface-100 bg-surface-0 px-6 dark:border-surface-800 dark:bg-surface-900 md:flex lg:px-8">
        <div class="flex items-center gap-2">
          <LocaleSwitcher variant="toolbar" />
          <AccountMenu
            :name="accountName"
            :subtitle="$t('account.adminSubtitle')"
            :role-label="$t('account.adminRole')"
            :avatar-text="accountName"
            :open-label="$t('account.openMenu')"
            :logout-label="$t('nav.logout')"
            :items="accountMenuItems"
            @logout="logout"
          />
        </div>
      </header>

      <!-- Mobile top bar: hamburger + brand. md:hidden because the
           sidebar is always-visible at md+. -->
      <header class="flex h-14 items-center gap-3 border-b border-surface-100 bg-surface-0 px-4 dark:border-surface-800 dark:bg-surface-900 md:hidden">
        <button
          type="button"
          :aria-label="drawerOpen ? $t('a11y.closeNav') : $t('a11y.openNav')"
          class="flex h-9 w-9 items-center justify-center rounded-xl text-surface-600 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
          @click="drawerOpen = !drawerOpen"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <div class="min-w-0 flex-1 truncate text-sm font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ branding.title || $t('app.title') }}</div>
        <LocaleSwitcher variant="toolbar" />
        <AccountMenu
          :name="accountName"
          :subtitle="$t('account.adminSubtitle')"
          :role-label="$t('account.adminRole')"
          :avatar-text="accountName"
          :open-label="$t('account.openMenu')"
          :logout-label="$t('nav.logout')"
          :items="accountMenuItems"
          @logout="logout"
        />
      </header>

      <section class="mx-auto w-full max-w-page px-4 py-5 sm:px-6 sm:py-7 lg:px-8 lg:py-9">
        <router-view />
      </section>
    </main>
  </div>
</template>
