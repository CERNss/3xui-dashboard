<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { useThemeStore } from '@/stores/theme'

const router = useRouter()
const route = useRoute()
const auth = useAdminAuthStore()
const theme = useThemeStore()

// Mobile drawer state. Sidebar is always-on at md+ (md: visible),
// off-canvas + toggleable below that. Close on route change so a
// nav click doesn't leave the drawer hanging open.
const drawerOpen = ref(false)
watch(() => route.fullPath, () => { drawerOpen.value = false })

function logout() {
  auth.clear()
  router.push({ name: 'admin.login' })
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
const sections: NavSection[] = [
  {
    title: '总览',
    items: [
      {
        to: '/admin/status',
        label: '系统状态',
        icon: 'M3 12a9 9 0 1 0 18 0 9 9 0 0 0-18 0zM12 7v5l3 2',
      },
      {
        to: '/admin/stats',
        label: '统计',
        icon: 'M3 17l6-6 4 4 8-8M14 7h7v7',
      },
    ],
  },
  {
    title: '节点运维',
    items: [
      {
        to: '/admin/nodes',
        label: '节点列表',
        icon: 'M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01',
      },
      {
        to: '/admin/inbounds',
        label: '入站列表',
        icon: 'M4 6h16M4 12h16M4 18h16',
      },
    ],
  },
  {
    title: '用户与计费',
    items: [
      {
        to: '/admin/users',
        label: '用户管理',
        icon: 'M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0',
      },
      {
        to: '/admin/plans',
        label: '套餐管理',
        icon: 'M9 11l3 3L22 4M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11',
      },
      {
        to: '/admin/orders',
        label: '订单管理',
        icon: 'M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6',
      },
    ],
  },
  {
    title: '系统',
    items: [
      {
        to: '/admin/webhooks',
        label: 'Webhooks',
        icon: 'M13 10V3L4 14h7v7l9-11h-7z',
      },
      {
        to: '/admin/audit-log',
        label: '审计日志',
        icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5.586a1 1 0 0 1 .707.293l5.414 5.414a1 1 0 0 1 .293.707V19a2 2 0 0 1-2 2z',
      },
      {
        to: '/admin/settings',
        label: '面板设置',
        icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 0 0-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 0 0-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 0 0-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 0 0-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 0 0 1.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
      },
    ],
  },
]

const initial = computed(() => (auth.username || 'A').slice(0, 1).toUpperCase())
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
        'fixed inset-y-0 left-0 z-40 shadow-elevated transition-transform duration-200 ease-brand',
        drawerOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
      ]"
    >
      <!-- Brand -->
      <div class="mb-7 flex items-center gap-3 px-2">
        <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-accent-500 to-accent-700 text-white shadow-card ring-1 ring-accent-700/30">
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z" />
          </svg>
        </div>
        <div class="leading-tight">
          <div class="text-body-md font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('app.title') }}</div>
          <div class="text-eyebrow uppercase tracking-eyebrow text-surface-400">central panel</div>
        </div>
      </div>

      <!-- Nav -->
      <nav class="flex-1 space-y-6 text-sm">
        <div v-for="(section, sIdx) in sections" :key="sIdx" class="space-y-1">
          <div class="px-3 pb-1 text-eyebrow font-semibold uppercase tracking-eyebrow text-surface-400 dark:text-surface-500">
            {{ section.title }}
          </div>
          <router-link
            v-for="item in section.items"
            :key="item.to"
            :to="item.to"
            class="group relative flex items-center gap-3 rounded-xl px-3 py-2 font-medium text-surface-600 transition-all duration-150 ease-brand hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
            active-class="!bg-accent-50 !text-accent-700 shadow-rail dark:!bg-accent-950/40 dark:!text-accent-300"
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
            <span>{{ item.label }}</span>
          </router-link>
        </div>
      </nav>

      <!-- Theme toggle — Sub2API pattern: labeled sidebar item, not a tiny icon button. -->
      <button
        type="button"
        class="group mt-3 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium text-surface-600 transition-all duration-150 ease-brand hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
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
        <span>{{ theme.theme === 'dark' ? '浅色模式' : '深色模式' }}</span>
      </button>

      <!-- Footer: user + logout -->
      <div class="mt-3 rounded-2xl border border-surface-100/80 bg-surface-50/60 p-2.5 dark:border-surface-800 dark:bg-surface-800/40">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-ink-900 text-xs font-semibold text-white dark:bg-ink-700">
            {{ initial }}
          </div>
          <div class="min-w-0 flex-1 leading-tight">
            <div class="truncate text-xs font-medium text-ink-900 dark:text-surface-50">{{ auth.username || 'admin' }}</div>
            <div class="text-eyebrow uppercase tracking-wider text-surface-400">signed in</div>
          </div>
          <button
            :title="$t('nav.logout')"
            class="flex h-8 w-8 items-center justify-center rounded-xl text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400"
            @click="logout"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M15 12H4M11 16l-4-4 4-4M20 4v16" />
            </svg>
          </button>
        </div>
      </div>
    </aside>

    <main class="flex flex-1 flex-col overflow-y-auto">
      <!-- Mobile top bar: hamburger + brand. md:hidden because the
           sidebar is always-visible at md+. -->
      <header class="flex h-14 items-center gap-3 border-b border-surface-100 bg-surface-0 px-4 dark:border-surface-800 dark:bg-surface-900 md:hidden">
        <button
          type="button"
          :aria-label="drawerOpen ? '关闭导航' : '打开导航'"
          class="flex h-9 w-9 items-center justify-center rounded-xl text-surface-600 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
          @click="drawerOpen = !drawerOpen"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <div class="text-sm font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('app.title') }}</div>
      </header>

      <section class="mx-auto w-full max-w-page px-4 py-5 sm:px-6 sm:py-7 lg:px-8 lg:py-9">
        <router-view />
      </section>
    </main>
  </div>
</template>
