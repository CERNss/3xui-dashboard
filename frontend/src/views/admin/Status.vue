<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatError } from '@/utils/format'

import { nodesApi, type Node } from '@/api/admin/nodes'
import { inboundsApi, type FleetResult } from '@/api/admin/inbounds'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { useRouter } from 'vue-router'
const router = useRouter()
const { t } = useI18n()

const nodes = ref<Node[]>([])
const fleet = ref<FleetResult>({ inbounds: [] })
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [n, f] = await Promise.all([nodesApi.list(), inboundsApi.fleet()])
    nodes.value = Array.isArray(n) ? n : []
    fleet.value = { ...f, inbounds: Array.isArray(f?.inbounds) ? f.inbounds : [] }
  } catch (e: any) {
    error.value = formatError(e, t('admin.status.loadFailed'))
  } finally {
    loading.value = false
  }
}

function formatBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = Math.abs(n)
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return (i === 0 ? v.toFixed(0) : v.toFixed(2)) + ' ' + units[i]
}

const stats = computed(() => {
  const inbounds = fleet.value.inbounds.map((f) => f.inbound)
  const online = nodes.value.filter((n) => n.status === 'online').length
  const offline = nodes.value.filter((n) => n.status === 'offline').length
  const unknown = nodes.value.filter((n) => n.status === 'unknown').length
  return {
    nodes: nodes.value.length,
    online,
    offline,
    unknown,
    inbounds: inbounds.length,
    clients: inbounds.reduce((s, i) => s + (i.clientStats?.length ?? 0), 0),
    up: inbounds.reduce((s, i) => s + (i.up || 0), 0),
    down: inbounds.reduce((s, i) => s + (i.down || 0), 0),
    allTime: inbounds.reduce((s, i) => s + (i.allTime || 0), 0),
  }
})

const totalNow = computed(() => stats.value.up + stats.value.down)

function nodeStatusText(status: string | undefined | null): string {
  if (status === 'online' || status === 'offline' || status === 'unknown') {
    return t(`admin.nodes.status.${status}`)
  }
  return status || '—'
}

function formatDateTime(value: string | null | undefined): string {
  return value ? new Date(value).toLocaleString() : '—'
}

type NodeStateTone = 'online' | 'offline' | 'unknown' | 'disabled'

function nodeStateTone(n: Node): NodeStateTone {
  if (!n.enabled) return 'disabled'
  if (n.status === 'online' || n.status === 'offline' || n.status === 'unknown') return n.status
  return 'unknown'
}

function hasProbeData(n: Node): boolean {
  return Boolean(n.last_seen_at || n.xray_version || n.cpu_pct > 0 || n.mem_pct > 0)
}

function nodeStateLabel(n: Node): string {
  const tone = nodeStateTone(n)
  if (tone === 'disabled') return t('admin.status.nodeState.disabled')
  if (tone === 'unknown') return t('admin.status.nodeState.unknown')
  return nodeStatusText(tone)
}

function nodeStateDetail(n: Node): string {
  const tone = nodeStateTone(n)
  if (tone === 'online') return t('admin.status.nodeState.onlineHint')
  if (tone === 'disabled') return t('admin.status.nodeState.disabledHint')
  if (tone === 'unknown') return t('admin.status.nodeState.unknownHint')
  if (n.last_seen_at) {
    return t('admin.status.nodeState.offlineLastSeen', { time: formatDateTime(n.last_seen_at) })
  }
  return t('admin.status.nodeState.offlineNeverSeen')
}

function nodeStateBadgeClass(n: Node): string {
  switch (nodeStateTone(n)) {
    case 'online':
      return 'bg-accent-50 text-accent-700 ring-accent-200 dark:bg-accent-500/15 dark:text-accent-200 dark:ring-accent-400/30'
    case 'offline':
      return 'bg-red-50 text-red-700 ring-red-200 dark:bg-red-500/15 dark:text-red-200 dark:ring-red-400/30'
    case 'disabled':
      return 'bg-surface-100 text-surface-700 ring-surface-200 dark:bg-surface-700/55 dark:text-surface-100 dark:ring-surface-500/50'
    default:
      return 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-500/15 dark:text-amber-200 dark:ring-amber-400/30'
  }
}

function nodeStateDotClass(n: Node): string {
  switch (nodeStateTone(n)) {
    case 'online':
      return 'bg-accent-500 dark:bg-accent-300'
    case 'offline':
      return 'bg-red-500 dark:bg-red-300'
    case 'disabled':
      return 'bg-surface-500 dark:bg-surface-300'
    default:
      return 'bg-amber-500 dark:bg-amber-300'
  }
}

function nodeMetricText(n: Node): string {
  if (!hasProbeData(n) && n.status !== 'online') return '—'
  return `${n.cpu_pct.toFixed(1)}% · ${n.mem_pct.toFixed(1)}%`
}

function nodeMetricHint(n: Node): string {
  const tone = nodeStateTone(n)
  if (tone === 'online') return t('admin.status.nodeState.liveMetrics')
  if (tone === 'disabled') return t('admin.status.nodeState.disabledMetrics')
  if (hasProbeData(n)) return t('admin.status.nodeState.staleMetrics')
  return t('admin.status.nodeState.noMetrics')
}

function xrayHint(n: Node): string {
  const tone = nodeStateTone(n)
  if (!n.xray_version) return t('admin.status.nodeState.noReport')
  if (tone === 'online') return t('admin.status.nodeState.liveReport')
  return t('admin.status.nodeState.staleReport')
}

onMounted(reload)

// Exposed so the parent Overview page can drive a shared
// refresh button without re-implementing the fetch logic here.
defineExpose({ reload })
</script>

<template>
  <div>
    <p v-if="error" class="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</p>

    <div v-if="loading" class="space-y-6">
      <Skeleton variant="kpi" :rows="4" />
      <Skeleton :rows="3" />
    </div>

    <section v-else class="space-y-6">
      <!-- KPI strip — Xboard-style: tiny label + icon top, big number, delta subtitle.
           Single accent across all 4 cards; semantics live in the icon, not the bg. -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <!-- Nodes (accent teal) -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-700 dark:bg-surface-900 dark:hover:border-surface-500">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.status.kpi.nodes') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-500/15 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.nodes }}</span>
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-1.5 text-2xs">
            <span v-if="stats.online" class="inline-flex items-center gap-1 rounded-full bg-accent-50 px-2 py-0.5 font-medium text-accent-700 ring-1 ring-inset ring-accent-100 dark:bg-accent-500/15 dark:text-accent-200 dark:ring-accent-400/30">
              <span class="h-1.5 w-1.5 rounded-full bg-accent-500 dark:bg-accent-300" /> {{ $t('admin.status.kpi.online', { n: stats.online }) }}
            </span>
            <span v-if="stats.offline" class="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 font-medium text-red-700 ring-1 ring-inset ring-red-100 dark:bg-red-500/15 dark:text-red-200 dark:ring-red-400/30">
              <span class="h-1.5 w-1.5 rounded-full bg-red-500 dark:bg-red-300" /> {{ $t('admin.status.kpi.offline', { n: stats.offline }) }}
            </span>
            <span v-if="stats.unknown" class="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 font-medium text-amber-700 ring-1 ring-inset ring-amber-100 dark:bg-amber-500/15 dark:text-amber-200 dark:ring-amber-400/30">
              <span class="h-1.5 w-1.5 rounded-full bg-amber-500 dark:bg-amber-300" /> {{ $t('admin.status.kpi.unknown', { n: stats.unknown }) }}
            </span>
          </div>
        </div>

        <!-- Inbounds (primary indigo) -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-700 dark:bg-surface-900 dark:hover:border-surface-500">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.status.kpi.inbounds') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-primary-50 text-primary-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-primary-500/15 dark:text-primary-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h16M4 12h16M4 18h16" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.inbounds }}</span>
          </div>
          <div class="mt-4 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.status.kpi.inboundsHint', { n: stats.nodes }) }}</div>
        </div>

        <!-- Clients (amber) -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-700 dark:bg-surface-900 dark:hover:border-surface-500">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.status.kpi.clients') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-amber-50 text-amber-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-amber-500/15 dark:text-amber-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.clients }}</span>
          </div>
          <div class="mt-4 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.status.kpi.clientsHint') }}</div>
        </div>

        <!-- Traffic (pink) -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-700 dark:bg-surface-900 dark:hover:border-surface-500">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.status.kpi.traffic') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-pink-50 text-pink-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-pink-500/15 dark:text-pink-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 17l6-6 4 4 8-8" /><path d="M14 7h7v7" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-2xl font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(totalNow) }}</span>
          </div>
          <div class="mt-4 flex items-center gap-3 text-2xs text-surface-600 dark:text-surface-200">
            <span class="inline-flex items-center gap-1">
              <svg class="h-3 w-3 text-accent-600 dark:text-accent-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5M5 12l7-7 7 7" /></svg>
              {{ formatBytes(stats.up) }}
            </span>
            <span class="inline-flex items-center gap-1">
              <svg class="h-3 w-3 text-primary-500 dark:text-primary-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12l7 7 7-7" /></svg>
              {{ formatBytes(stats.down) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Node health table -->
      <div class="overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-700 dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-4 dark:border-surface-700/70">
          <div>
            <h2 class="text-body-md font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.status.nodeHealth') }}</h2>
            <p class="mt-0.5 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.status.nodeHealthHint') }}</p>
          </div>
          <router-link to="/admin/nodes" class="inline-flex items-center gap-1 text-xs font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-300">
            {{ $t('admin.status.manage') }}
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </router-link>
        </header>
        <table class="min-w-full text-sm">
          <thead class="text-left text-xs font-semibold uppercase tracking-wider text-surface-600 dark:text-surface-300">
            <tr class="border-b border-surface-100 dark:border-surface-700/70">
              <th class="px-6 py-3 font-medium">{{ $t('admin.nodes.column.name') }}</th>
              <th class="px-6 py-3 font-medium">{{ $t('admin.nodes.column.status') }}</th>
              <th class="px-6 py-3 font-medium">{{ $t('admin.nodes.column.cpuMem') }}</th>
              <th class="px-6 py-3 font-medium">{{ $t('admin.nodes.column.xray') }}</th>
              <th class="px-6 py-3 font-medium">{{ $t('admin.nodes.column.lastSeen') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-100 dark:divide-surface-700/60">
            <tr
              v-for="n in nodes"
              :key="n.id"
              :class="{
                'bg-red-50/30 dark:bg-red-950/10': nodeStateTone(n) === 'offline',
                'bg-surface-50/70 dark:bg-surface-800/20': nodeStateTone(n) === 'disabled',
              }"
              class="transition-colors hover:bg-surface-50/70 dark:hover:bg-surface-800/45"
            >
              <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">
                <div>{{ n.name }}</div>
                <div v-if="!n.enabled" class="mt-0.5 text-2xs font-medium text-surface-500 dark:text-surface-300">{{ $t('admin.status.nodeState.disabled') }}</div>
              </td>
              <td class="px-6 py-3.5">
                <span
                  class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                  :class="nodeStateBadgeClass(n)"
                >
                  <span class="h-1.5 w-1.5 rounded-full" :class="nodeStateDotClass(n)" />
                  {{ nodeStateLabel(n) }}
                </span>
                <div class="mt-1.5 max-w-[18rem] text-2xs leading-4 text-surface-600 dark:text-surface-300">{{ nodeStateDetail(n) }}</div>
              </td>
              <td class="px-6 py-3.5 tabular-nums">
                <div class="text-surface-700 dark:text-surface-100">{{ nodeMetricText(n) }}</div>
                <div class="mt-0.5 text-2xs text-surface-500 dark:text-surface-300">{{ nodeMetricHint(n) }}</div>
              </td>
              <td class="px-6 py-3.5">
                <div class="font-mono text-xs text-surface-700 dark:text-surface-100">{{ n.xray_version || '—' }}</div>
                <div class="mt-0.5 text-2xs text-surface-500 dark:text-surface-300">{{ xrayHint(n) }}</div>
              </td>
              <td class="px-6 py-3.5">
                <div class="text-xs font-medium text-surface-700 dark:text-surface-100">{{ n.last_seen_at ? formatDateTime(n.last_seen_at) : $t('admin.status.nodeState.neverSeen') }}</div>
              </td>
            </tr>
            <tr v-if="nodes.length === 0">
              <td colspan="5" class="p-0">
                <EmptyState
                  icon="M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01"
                  :title="$t('admin.status.empty')"
                  :description="$t('admin.status.emptyDescription')"
                  :action-label="$t('admin.status.emptyAction')"
                  @action="router.push('/admin/nodes')"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.status.allTimeUsage', { value: formatBytes(stats.allTime) }) }}</p>
    </section>
  </div>
</template>
