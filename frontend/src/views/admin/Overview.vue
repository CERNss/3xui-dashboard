<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import Status from './Status.vue'
import Stats from './Stats.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

type Tab = 'status' | 'stats'
const tabs: Tab[] = ['status', 'stats']

// Tab <-> route path is 1:1. Driving the active tab from route.path
// keeps the sidebar's active-link highlight in sync without extra wiring.
const tabPath: Record<Tab, string> = {
  status: '/admin/status',
  stats: '/admin/stats',
}

function tabFromPath(path: string): Tab {
  return path === '/admin/stats' ? 'stats' : 'status'
}

const active = ref<Tab>(tabFromPath(route.path))

// Lazy-mount panels on first activation, then keep them around with
// v-show. Switching tabs becomes a CSS toggle (no re-fetch), but the
// inactive panel's API call doesn't fire until the user actually
// opens it.
const mounted = ref<Set<Tab>>(new Set([active.value]))

const statusRef = ref<InstanceType<typeof Status> | null>(null)
const statsRef = ref<InstanceType<typeof Stats> | null>(null)

watch(
  () => route.path,
  (path) => {
    const next = tabFromPath(path)
    if (next !== active.value) {
      active.value = next
      mounted.value.add(next)
    }
  },
)

function selectTab(target: Tab) {
  if (target === active.value) return
  void router.replace(tabPath[target])
}

const headerTitle = computed(() =>
  active.value === 'status' ? t('admin.status.title') : t('admin.stats.title'),
)
const headerSubtitle = computed(() =>
  active.value === 'status' ? t('admin.status.subtitle') : t('admin.stats.subtitle'),
)

function reloadActive() {
  if (active.value === 'status') statusRef.value?.reload()
  else statsRef.value?.reload()
}

function tabLabel(target: Tab): string {
  return target === 'status' ? t('nav.status') : t('nav.stats')
}
</script>

<template>
  <div>
    <header class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ headerTitle }}</h1>
        <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">{{ headerSubtitle }}</p>
      </div>
      <button
        type="button"
        class="inline-flex h-9 items-center gap-1.5 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700 dark:hover:text-surface-50"
        @click="reloadActive"
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        {{ $t('admin.status.reload') }}
      </button>
    </header>

    <nav class="mb-5 inline-flex max-w-full flex-wrap gap-1 rounded-xl border border-surface-100 bg-surface-0 p-1 shadow-card dark:border-surface-800 dark:bg-surface-900" role="tablist" aria-label="Overview surfaces">
      <button
        v-for="tab in tabs"
        :key="tab"
        type="button"
        role="tab"
        :aria-selected="active === tab"
        :class="[
          'inline-flex h-9 shrink-0 items-center rounded-lg px-3.5 text-sm font-semibold transition-all ease-brand active:scale-[0.98]',
          active === tab
            ? 'bg-ink-900 text-white shadow-card dark:bg-surface-50 dark:text-ink-900'
            : 'text-surface-500 hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-50',
        ]"
        @click="selectTab(tab)"
      >
        {{ tabLabel(tab) }}
      </button>
    </nav>

    <Status v-if="mounted.has('status')" v-show="active === 'status'" ref="statusRef" />
    <Stats v-if="mounted.has('stats')" v-show="active === 'stats'" ref="statsRef" />
  </div>
</template>
