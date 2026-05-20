<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { formatError, nodeStatusLabel } from '@/utils/format'

import { nodesApi, type Node } from '@/api/admin/nodes'
import { inboundsApi, type FleetResult } from '@/api/admin/inbounds'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { useRouter } from 'vue-router'
const router = useRouter()

const nodes = ref<Node[]>([])
const fleet = ref<FleetResult>({ inbounds: [] })
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [n, f] = await Promise.all([nodesApi.list(), inboundsApi.fleet()])
    nodes.value = n
    fleet.value = f
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
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

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">系统状态</h1>
        <p class="mt-1.5 text-sm text-surface-500">fleet 总览：节点健康 · 入站数 · 客户端 · 流量</p>
      </div>
      <button
        class="inline-flex h-9 items-center gap-1.5 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800"
        @click="reload"
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        刷新
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</p>

    <div v-if="loading" class="space-y-6">
      <Skeleton variant="kpi" :rows="4" />
      <Skeleton :rows="3" />
    </div>

    <section v-else class="space-y-6">
      <!-- KPI strip — Xboard-style: tiny label + icon top, big number, delta subtitle.
           Single accent across all 4 cards; semantics live in the icon, not the bg. -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <!-- Nodes -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">节点</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.nodes }}</span>
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-1.5 text-2xs">
            <span v-if="stats.online" class="inline-flex items-center gap-1 rounded-full bg-accent-50 px-2 py-0.5 font-medium text-accent-700 dark:bg-accent-950/40 dark:text-accent-300">
              <span class="h-1.5 w-1.5 rounded-full bg-accent-500" /> {{ stats.online }} 在线
            </span>
            <span v-if="stats.offline" class="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 font-medium text-red-600 dark:bg-red-950/40 dark:text-red-300">
              <span class="h-1.5 w-1.5 rounded-full bg-red-500" /> {{ stats.offline }} 离线
            </span>
            <span v-if="stats.unknown" class="inline-flex items-center gap-1 rounded-full bg-surface-100 px-2 py-0.5 font-medium text-surface-500 dark:bg-surface-800 dark:text-surface-400">
              <span class="h-1.5 w-1.5 rounded-full bg-surface-400" /> {{ stats.unknown }} 未知
            </span>
          </div>
        </div>

        <!-- Inbounds -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">入站</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h16M4 12h16M4 18h16" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.inbounds }}</span>
          </div>
          <div class="mt-4 text-2xs text-surface-500">across {{ stats.nodes }} 节点</div>
        </div>

        <!-- Clients -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">客户端</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.clients }}</span>
          </div>
          <div class="mt-4 text-2xs text-surface-500">已 provisioned</div>
        </div>

        <!-- Traffic -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">总流量</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 17l6-6 4 4 8-8" /><path d="M14 7h7v7" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-2xl font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(totalNow) }}</span>
          </div>
          <div class="mt-4 flex items-center gap-3 text-2xs text-surface-500">
            <span class="inline-flex items-center gap-1">
              <svg class="h-3 w-3 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5M5 12l7-7 7 7" /></svg>
              {{ formatBytes(stats.up) }}
            </span>
            <span class="inline-flex items-center gap-1">
              <svg class="h-3 w-3 text-primary-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12l7 7 7-7" /></svg>
              {{ formatBytes(stats.down) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Node health table -->
      <div class="overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-4 dark:border-surface-800">
          <div>
            <h2 class="text-body-md font-semibold tracking-tight text-ink-900 dark:text-surface-50">节点健康</h2>
            <p class="mt-0.5 text-xs text-surface-500">每 30 秒后台探测一次</p>
          </div>
          <router-link to="/admin/nodes" class="inline-flex items-center gap-1 text-xs font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-300">
            管理
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </router-link>
        </header>
        <table class="min-w-full text-sm">
          <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
            <tr class="border-b border-surface-100 dark:border-surface-800">
              <th class="px-6 py-3 font-medium">名称</th>
              <th class="px-6 py-3 font-medium">状态</th>
              <th class="px-6 py-3 font-medium">CPU / Mem</th>
              <th class="px-6 py-3 font-medium">Xray</th>
              <th class="px-6 py-3 font-medium">Last Seen</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
            <tr v-for="n in nodes" :key="n.id" :class="n.enabled ? '' : 'opacity-60'" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
              <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ n.name }}</td>
              <td class="px-6 py-3.5">
                <span
                  class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                  :class="{
                    'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800': n.status === 'online',
                    'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800': n.status === 'offline',
                    'bg-surface-100 text-surface-500 ring-surface-200 dark:bg-surface-800 dark:text-surface-400 dark:ring-surface-700': n.status === 'unknown',
                  }"
                >
                  <span class="h-1.5 w-1.5 rounded-full" :class="{
                    'bg-accent-500': n.status === 'online',
                    'bg-red-500': n.status === 'offline',
                    'bg-surface-400': n.status === 'unknown',
                  }" />
                  {{ nodeStatusLabel(n.status) }}
                </span>
              </td>
              <td class="px-6 py-3.5 tabular-nums text-surface-600 dark:text-surface-300">{{ n.cpu_pct.toFixed(1) }}% · {{ n.mem_pct.toFixed(1) }}%</td>
              <td class="px-6 py-3.5 font-mono text-xs text-surface-500">{{ n.xray_version || '—' }}</td>
              <td class="px-6 py-3.5 text-xs text-surface-500">{{ n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : '—' }}</td>
            </tr>
            <tr v-if="nodes.length === 0">
              <td colspan="5" class="p-0">
                <EmptyState
                  icon="M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01"
                  title="还没有节点"
                  description="dashboard 是 multi-node 聚合层，至少要接一台上游 3x-ui 面板才能工作。"
                  action-label="去添加节点"
                  @action="router.push('/admin/nodes')"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="text-xs text-surface-400">所有时间总使用量 · {{ formatBytes(stats.allTime) }}</p>
    </section>
  </div>
</template>
