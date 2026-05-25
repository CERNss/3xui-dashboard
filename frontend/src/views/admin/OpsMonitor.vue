<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { inboundsApi, type FleetResult } from '@/api/admin/inbounds'
import { nodesApi, type Node, type NodeMetricPoint } from '@/api/admin/nodes'
import Skeleton from '@/components/common/Skeleton.vue'
import { formatError } from '@/utils/format'

const { t } = useI18n()

interface TrendPoint {
  time: number
  cpu: number
  mem: number
}

interface MonitorCard {
  label: string
  value: string
  hint: string
  tone: 'live' | 'warn' | 'muted'
}

interface AnalysisPanel {
  title: string
  subtitle: string
  kind: 'bars' | 'line' | 'stack' | 'dots'
}

const nodes = ref<Node[]>([])
const fleet = ref<FleetResult>({ inbounds: [] })
const metricSeries = ref<Record<number, NodeMetricPoint[]>>({})
const loading = ref(true)
const error = ref<string | null>(null)
const metricError = ref<string | null>(null)
const lastRefresh = ref<Date | null>(null)

function safeInbounds(value: FleetResult | null | undefined): FleetResult {
  const base = value ?? { inbounds: [] }
  return {
    ...base,
    inbounds: Array.isArray(value?.inbounds) ? value.inbounds : [],
    node_errors: value?.node_errors ?? {},
  }
}

async function loadMetrics(rows: Node[]) {
  const targets = rows.filter((n) => n.enabled)
  metricSeries.value = {}
  metricError.value = null
  if (targets.length === 0) return

  const to = Math.floor(Date.now() / 1000)
  const from = to - 3 * 60 * 60
  const results = await Promise.allSettled(
    targets.map((n) => nodesApi.metrics(n.id, { from, to, bucket: '10m' })),
  )
  const next: Record<number, NodeMetricPoint[]> = {}
  let failures = 0
  results.forEach((result, index) => {
    const nodeID = targets[index].id
    if (result.status === 'fulfilled') {
      next[nodeID] = Array.isArray(result.value?.points) ? result.value.points : []
      return
    }
    failures += 1
    next[nodeID] = []
  })
  metricSeries.value = next
  if (failures > 0) {
    metricError.value = t('admin.opsMonitor.metricsPartialFailed', { n: failures })
  }
}

async function reload() {
  loading.value = true
  error.value = null
  metricError.value = null
  try {
    const [nodeRows, fleetRows] = await Promise.all([nodesApi.list(), inboundsApi.fleet()])
    nodes.value = Array.isArray(nodeRows) ? nodeRows : []
    fleet.value = safeInbounds(fleetRows)
    lastRefresh.value = new Date()
    await loadMetrics(nodes.value)
  } catch (e: any) {
    error.value = formatError(e, t('admin.opsMonitor.loadFailed'))
  } finally {
    loading.value = false
  }
}

const enabledNodes = computed(() => nodes.value.filter((n) => n.enabled))
const onlineNodes = computed(() => enabledNodes.value.filter((n) => n.status === 'online'))
const offlineNodes = computed(() => enabledNodes.value.filter((n) => n.status === 'offline'))
const unknownNodes = computed(() =>
  enabledNodes.value.filter((n) => n.status !== 'online' && n.status !== 'offline'),
)
const disabledNodes = computed(() => nodes.value.filter((n) => !n.enabled))
const attentionCount = computed(
  () => offlineNodes.value.length + unknownNodes.value.length + disabledNodes.value.length,
)
const healthScore = computed(() => {
  if (enabledNodes.value.length === 0) return 0
  return Math.round((onlineNodes.value.length / enabledNodes.value.length) * 100)
})
const healthTone = computed<'live' | 'warn' | 'muted'>(() => {
  if (enabledNodes.value.length === 0) return 'muted'
  return attentionCount.value > 0 ? 'warn' : 'live'
})
const healthLabel = computed(() => {
  if (enabledNodes.value.length === 0) return t('admin.opsMonitor.health.empty')
  return attentionCount.value > 0
    ? t('admin.opsMonitor.health.warning')
    : t('admin.opsMonitor.health.healthy')
})

const inbounds = computed(() => fleet.value.inbounds.map((row) => row.inbound))
const activeInbounds = computed(() => inbounds.value.filter((row) => row.enable).length)
const clientCount = computed(() =>
  inbounds.value.reduce((sum, row) => sum + (row.clientStats?.length ?? 0), 0),
)
const enabledClientCount = computed(() =>
  inbounds.value.reduce(
    (sum, row) => sum + (row.clientStats?.filter((client) => client.enable !== false).length ?? 0),
    0,
  ),
)
const nodeErrorCount = computed(() => Object.keys(fleet.value.node_errors ?? {}).length)

function hasProbeData(n: Node): boolean {
  return Boolean(n.status === 'online' || n.last_seen_at || n.cpu_pct > 0 || n.mem_pct > 0)
}

const resourceNodes = computed(() => enabledNodes.value.filter(hasProbeData))

function avg(values: number[]): number | null {
  if (values.length === 0) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

const cpuAvg = computed(() => avg(resourceNodes.value.map((n) => n.cpu_pct)))
const memAvg = computed(() => avg(resourceNodes.value.map((n) => n.mem_pct)))

function formatPercent(value: number | null): string {
  if (value === null || Number.isNaN(value)) return t('admin.opsMonitor.unavailable')
  return `${value.toFixed(1)}%`
}

function formatRatio(value: number, total: number): string {
  return `${value}/${total}`
}

function lastRefreshText(): string {
  if (!lastRefresh.value) return t('admin.opsMonitor.waitingRefresh')
  return t('admin.opsMonitor.lastRefresh', {
    time: lastRefresh.value.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
  })
}

const realtimeCards = computed<MonitorCard[]>(() => [
  {
    label: t('admin.opsMonitor.metric.qps'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.tps'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.requests'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.tokens'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.activeInbounds'),
    value: formatRatio(activeInbounds.value, inbounds.value.length),
    hint: t('admin.opsMonitor.metric.fromFleet'),
    tone: activeInbounds.value > 0 ? 'live' : 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.clients'),
    value: enabledClientCount.value.toLocaleString(),
    hint: t('admin.opsMonitor.metric.clientsHint', { total: clientCount.value }),
    tone: enabledClientCount.value > 0 ? 'live' : 'muted',
  },
])

const qualityCards = computed<MonitorCard[]>(() => [
  {
    label: t('admin.opsMonitor.metric.sla'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.requestErrors'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.latencyP99'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.ttftP99'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.businessUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.upstreamErrors'),
    value: nodeErrorCount.value.toLocaleString(),
    hint: t('admin.opsMonitor.metric.nodeErrorsHint'),
    tone: nodeErrorCount.value > 0 ? 'warn' : 'live',
  },
])

const infraCards = computed<MonitorCard[]>(() => [
  {
    label: t('admin.opsMonitor.metric.cpu'),
    value: formatPercent(cpuAvg.value),
    hint: t('admin.opsMonitor.metric.nodeProbeAverage'),
    tone: cpuAvg.value === null ? 'muted' : cpuAvg.value >= 85 ? 'warn' : 'live',
  },
  {
    label: t('admin.opsMonitor.metric.memory'),
    value: formatPercent(memAvg.value),
    hint: t('admin.opsMonitor.metric.nodeProbeAverage'),
    tone: memAvg.value === null ? 'muted' : memAvg.value >= 85 ? 'warn' : 'live',
  },
  {
    label: t('admin.opsMonitor.metric.db'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.infrastructureUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.redis'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.infrastructureUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.queue'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.infrastructureUnavailable'),
    tone: 'muted',
  },
  {
    label: t('admin.opsMonitor.metric.backgroundTasks'),
    value: t('admin.opsMonitor.unavailable'),
    hint: t('admin.opsMonitor.metric.infrastructureUnavailable'),
    tone: 'muted',
  },
])

const analysisPanels = computed<AnalysisPanel[]>(() => [
  {
    title: t('admin.opsMonitor.analysis.concurrencyQueue'),
    subtitle: t('admin.opsMonitor.metric.businessUnavailable'),
    kind: 'bars',
  },
  {
    title: t('admin.opsMonitor.analysis.accountSwitch'),
    subtitle: t('admin.opsMonitor.metric.businessUnavailable'),
    kind: 'line',
  },
  {
    title: t('admin.opsMonitor.analysis.throughput'),
    subtitle: t('admin.opsMonitor.metric.businessUnavailable'),
    kind: 'line',
  },
  {
    title: t('admin.opsMonitor.analysis.durationDistribution'),
    subtitle: t('admin.opsMonitor.metric.businessUnavailable'),
    kind: 'stack',
  },
  {
    title: t('admin.opsMonitor.analysis.errorDistribution'),
    subtitle: t('admin.opsMonitor.metric.businessUnavailable'),
    kind: 'dots',
  },
  {
    title: t('admin.opsMonitor.analysis.errorTrend'),
    subtitle: t('admin.opsMonitor.metric.businessUnavailable'),
    kind: 'line',
  },
])

const trendPoints = computed<TrendPoint[]>(() => {
  const buckets = new Map<number, { cpu: number; mem: number; count: number }>()
  Object.values(metricSeries.value).forEach((points) => {
    points.forEach((point) => {
      const time = new Date(point.time).getTime()
      if (!Number.isFinite(time) || !Number.isFinite(point.cpu) || !Number.isFinite(point.mem)) {
        return
      }
      const bucket = buckets.get(time) ?? { cpu: 0, mem: 0, count: 0 }
      bucket.cpu += point.cpu
      bucket.mem += point.mem
      bucket.count += 1
      buckets.set(time, bucket)
    })
  })
  return [...buckets.entries()]
    .sort(([a], [b]) => a - b)
    .slice(-24)
    .map(([time, point]) => ({
      time,
      cpu: point.cpu / point.count,
      mem: point.mem / point.count,
    }))
})

function sparkline(points: TrendPoint[], key: 'cpu' | 'mem'): string {
  if (points.length < 2) return ''
  return points
    .map((point, index) => {
      const x = (index / (points.length - 1)) * 100
      const y = 42 - Math.min(100, Math.max(0, point[key])) * 0.38
      return `${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
}

const cpuLine = computed(() => sparkline(trendPoints.value, 'cpu'))
const memLine = computed(() => sparkline(trendPoints.value, 'mem'))

const loadedNodes = computed(() =>
  [...resourceNodes.value]
    .sort((a, b) => Math.max(b.cpu_pct, b.mem_pct) - Math.max(a.cpu_pct, a.mem_pct))
    .slice(0, 6),
)

function cardClass(tone: MonitorCard['tone']): string {
  if (tone === 'warn') {
    return 'border-amber-200 bg-amber-50/80 dark:border-amber-400/40 dark:bg-amber-500/12'
  }
  if (tone === 'live') {
    return 'border-accent-200 bg-accent-50/70 dark:border-accent-400/30 dark:bg-accent-500/10'
  }
  return 'border-surface-200 bg-surface-50 dark:border-surface-700 dark:bg-surface-800/65'
}

function dotClass(tone: MonitorCard['tone'] | 'muted'): string {
  if (tone === 'warn') return 'bg-amber-500 dark:bg-amber-300'
  if (tone === 'live') return 'bg-accent-500 dark:bg-accent-300'
  return 'bg-surface-400 dark:bg-surface-300'
}

onMounted(reload)

defineExpose({ reload })
</script>

<template>
  <div>
    <p
      v-if="error"
      class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-200 dark:ring-red-400/30"
    >
      {{ error }}
    </p>

    <div v-if="loading" class="space-y-5">
      <Skeleton variant="kpi" :rows="4" />
      <Skeleton variant="card" :rows="4" height="h-5" />
      <Skeleton :rows="4" />
    </div>

    <section v-else class="space-y-6">
      <header class="flex min-h-10 flex-wrap items-center justify-end gap-2 text-xs">
          <span
            class="inline-flex h-10 items-center gap-1.5 rounded-full px-3 font-semibold ring-1 ring-inset"
            :class="healthTone === 'warn'
              ? 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-500/15 dark:text-amber-100 dark:ring-amber-400/30'
              : healthTone === 'live'
              ? 'bg-accent-50 text-accent-700 ring-accent-200 dark:bg-accent-500/15 dark:text-accent-100 dark:ring-accent-400/30'
              : 'bg-surface-100 text-surface-700 ring-surface-200 dark:bg-surface-800 dark:text-surface-100 dark:ring-surface-600'"
          >
            <span class="h-2 w-2 rounded-full" :class="dotClass(healthTone)" />
            {{ healthLabel }}
          </span>
          <span class="inline-flex h-10 items-center rounded-full bg-surface-0 px-3 font-medium text-surface-700 ring-1 ring-inset ring-surface-200 dark:bg-surface-900 dark:text-surface-100 dark:ring-surface-600">
            {{ lastRefreshText() }}
          </span>
      </header>

      <div class="grid grid-cols-1 gap-5 xl:grid-cols-12">
        <section class="min-h-[19rem] rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900 xl:col-span-4">
          <div class="flex h-full flex-col justify-between gap-5">
            <div class="flex items-center justify-between">
              <div>
                <div class="text-xs font-semibold text-surface-700 dark:text-surface-100">{{ $t('admin.opsMonitor.health.title') }}</div>
                <div class="mt-1 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.health.formula') }}</div>
              </div>
              <span class="rounded-full bg-surface-50 px-2.5 py-1 text-2xs font-semibold text-surface-700 ring-1 ring-inset ring-surface-100 dark:bg-surface-800 dark:text-surface-100 dark:ring-surface-700">
                {{ formatRatio(onlineNodes.length, enabledNodes.length) }}
              </span>
            </div>

            <div class="relative mx-auto h-44 w-44">
              <svg class="h-full w-full -rotate-90" viewBox="0 0 120 120">
                <circle cx="60" cy="60" r="50" fill="none" stroke="currentColor" stroke-width="10" class="text-surface-200 dark:text-surface-700" />
                <circle
                  cx="60"
                  cy="60"
                  r="50"
                  fill="none"
                  stroke="currentColor"
                  stroke-linecap="round"
                  stroke-width="10"
                  pathLength="100"
                  :stroke-dasharray="`${healthScore} 100`"
                  :class="healthTone === 'warn' ? 'text-amber-500 dark:text-amber-300' : healthTone === 'live' ? 'text-accent-500 dark:text-accent-300' : 'text-surface-400 dark:text-surface-300'"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center text-center">
                <div class="text-display-md font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ healthScore }}</div>
                <div class="mt-1 text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.health.score') }}</div>
              </div>
            </div>

            <div class="grid grid-cols-4 gap-2 text-center">
              <div class="border-r border-surface-100 pr-2 dark:border-surface-700">
                <div class="text-sm font-semibold tabular-nums text-accent-700 dark:text-accent-200">{{ onlineNodes.length }}</div>
                <div class="mt-0.5 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.node.online') }}</div>
              </div>
              <div class="border-r border-surface-100 pr-2 dark:border-surface-700">
                <div class="text-sm font-semibold tabular-nums text-red-600 dark:text-red-200">{{ offlineNodes.length }}</div>
                <div class="mt-0.5 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.node.offline') }}</div>
              </div>
              <div class="border-r border-surface-100 pr-2 dark:border-surface-700">
                <div class="text-sm font-semibold tabular-nums text-amber-600 dark:text-amber-200">{{ unknownNodes.length }}</div>
                <div class="mt-0.5 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.node.unknown') }}</div>
              </div>
              <div>
                <div class="text-sm font-semibold tabular-nums text-surface-700 dark:text-surface-100">{{ disabledNodes.length }}</div>
                <div class="mt-0.5 text-2xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.node.disabled') }}</div>
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900 xl:col-span-8">
          <div>
            <h3 class="text-sm font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.realtime') }}</h3>
            <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <div
                v-for="card in realtimeCards"
                :key="card.label"
                class="min-h-[7.25rem] rounded-xl border p-4"
                :class="cardClass(card.tone)"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="min-w-0 truncate text-xs font-semibold text-surface-700 dark:text-surface-100">{{ card.label }}</span>
                  <span class="h-2 w-2 shrink-0 rounded-full" :class="dotClass(card.tone)" />
                </div>
                <div class="mt-3 text-2xl font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ card.value }}</div>
                <div class="mt-3 text-2xs leading-4 text-surface-600 dark:text-surface-300">{{ card.hint }}</div>
              </div>
            </div>
          </div>

          <div class="mt-4">
            <h3 class="text-sm font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.quality') }}</h3>
            <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-5">
              <div
                v-for="card in qualityCards"
                :key="card.label"
                class="min-h-[6.75rem] rounded-xl border p-4"
                :class="cardClass(card.tone)"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="min-w-0 truncate text-xs font-semibold text-surface-700 dark:text-surface-100">{{ card.label }}</span>
                  <span class="h-2 w-2 shrink-0 rounded-full" :class="dotClass(card.tone)" />
                </div>
                <div class="mt-3 text-xl font-semibold leading-none text-ink-900 tabular-nums dark:text-surface-50">{{ card.value }}</div>
                <div class="mt-3 text-2xs leading-4 text-surface-600 dark:text-surface-300">{{ card.hint }}</div>
              </div>
            </div>
          </div>
        </section>
      </div>

      <p
        v-if="metricError"
        class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-700 ring-1 ring-inset ring-amber-100 dark:bg-amber-500/12 dark:text-amber-100 dark:ring-amber-400/30"
      >
        {{ metricError }}
      </p>

      <div class="grid grid-cols-1 gap-5 xl:grid-cols-12">
        <section class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900 xl:col-span-7">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.resourceTrend') }}</h3>
              <p class="mt-1 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.resourceTrendHint') }}</p>
            </div>
            <div class="flex items-center gap-3 text-2xs font-medium text-surface-600 dark:text-surface-300">
              <span class="inline-flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-accent-500 dark:bg-accent-300" />{{ $t('admin.opsMonitor.metric.cpu') }}</span>
              <span class="inline-flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-sky-500 dark:bg-sky-300" />{{ $t('admin.opsMonitor.metric.memory') }}</span>
            </div>
          </div>

          <div class="mt-5 min-h-[16rem] rounded-xl border border-surface-100 bg-surface-50 p-4 dark:border-surface-700 dark:bg-surface-800/55">
            <svg v-if="trendPoints.length >= 2" class="h-44 w-full overflow-visible" viewBox="0 0 100 48" preserveAspectRatio="none" role="img" :aria-label="$t('admin.opsMonitor.resourceTrend')">
              <line v-for="y in [10, 20, 30, 40]" :key="y" x1="0" x2="100" :y1="y" :y2="y" stroke="currentColor" stroke-width="0.2" class="text-surface-300 dark:text-surface-700" />
              <polyline :points="memLine" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" class="text-sky-500 dark:text-sky-300" vector-effect="non-scaling-stroke" />
              <polyline :points="cpuLine" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="text-accent-500 dark:text-accent-300" vector-effect="non-scaling-stroke" />
            </svg>
            <div v-else class="flex h-44 flex-col items-center justify-center text-center">
              <div class="text-sm font-semibold text-surface-700 dark:text-surface-100">{{ $t('admin.opsMonitor.noTrend') }}</div>
              <div class="mt-1 max-w-xs text-xs leading-5 text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.noTrendHint') }}</div>
            </div>

            <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div class="rounded-lg bg-surface-0 p-3 ring-1 ring-inset ring-surface-100 dark:bg-surface-900 dark:ring-surface-700">
                <div class="text-2xs font-semibold text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.metric.cpu') }}</div>
                <div class="mt-1 text-xl font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ formatPercent(cpuAvg) }}</div>
              </div>
              <div class="rounded-lg bg-surface-0 p-3 ring-1 ring-inset ring-surface-100 dark:bg-surface-900 dark:ring-surface-700">
                <div class="text-2xs font-semibold text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.metric.memory') }}</div>
                <div class="mt-1 text-xl font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ formatPercent(memAvg) }}</div>
              </div>
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900 xl:col-span-5">
          <h3 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.infrastructure') }}</h3>
          <p class="mt-1 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.infrastructureHint') }}</p>
          <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div
              v-for="card in infraCards"
              :key="card.label"
              class="min-h-[6.5rem] rounded-xl border p-4"
              :class="cardClass(card.tone)"
            >
              <div class="flex items-center justify-between gap-3">
                <span class="min-w-0 truncate text-xs font-semibold text-surface-700 dark:text-surface-100">{{ card.label }}</span>
                <span class="h-2 w-2 shrink-0 rounded-full" :class="dotClass(card.tone)" />
              </div>
              <div class="mt-3 text-xl font-semibold leading-none text-ink-900 tabular-nums dark:text-surface-50">{{ card.value }}</div>
              <div class="mt-3 text-2xs leading-4 text-surface-600 dark:text-surface-300">{{ card.hint }}</div>
            </div>
          </div>
        </section>
      </div>

      <div class="grid grid-cols-1 gap-5 xl:grid-cols-2">
        <section class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <h3 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.nodeLoad') }}</h3>
          <p class="mt-1 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.nodeLoadHint') }}</p>
          <div class="mt-4 space-y-3">
            <div v-for="node in loadedNodes" :key="node.id" class="rounded-xl border border-surface-100 p-3 dark:border-surface-700">
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="min-w-0 truncate font-semibold text-ink-900 dark:text-surface-50">{{ node.name }}</span>
                <span class="shrink-0 tabular-nums text-surface-700 dark:text-surface-100">{{ node.cpu_pct.toFixed(1) }}% / {{ node.mem_pct.toFixed(1) }}%</span>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-2">
                <div class="h-2 overflow-hidden rounded-full bg-surface-100 dark:bg-surface-800">
                  <div class="h-full rounded-full bg-accent-500 dark:bg-accent-300" :style="{ width: Math.min(100, Math.max(0, node.cpu_pct)) + '%' }" />
                </div>
                <div class="h-2 overflow-hidden rounded-full bg-surface-100 dark:bg-surface-800">
                  <div class="h-full rounded-full bg-sky-500 dark:bg-sky-300" :style="{ width: Math.min(100, Math.max(0, node.mem_pct)) + '%' }" />
                </div>
              </div>
            </div>
            <div v-if="loadedNodes.length === 0" class="rounded-xl border border-dashed border-surface-200 px-4 py-8 text-center text-sm text-surface-600 dark:border-surface-700 dark:text-surface-300">
              {{ $t('admin.opsMonitor.noNodeMetrics') }}
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <h3 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.nodeErrors') }}</h3>
          <p class="mt-1 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.nodeErrorsHint') }}</p>
          <div class="mt-4 space-y-3">
            <div
              v-for="(message, id) in (fleet.node_errors ?? {})"
              :key="id"
              class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-400/30 dark:bg-amber-500/12"
            >
              <div class="text-xs font-semibold text-amber-800 dark:text-amber-100">{{ $t('admin.opsMonitor.nodeId', { id }) }}</div>
              <div class="mt-1 break-words text-xs leading-5 text-amber-700 dark:text-amber-100">{{ message }}</div>
            </div>
            <div v-if="nodeErrorCount === 0" class="rounded-xl border border-surface-100 bg-surface-50 px-4 py-8 text-center dark:border-surface-700 dark:bg-surface-800/55">
              <div class="text-sm font-semibold text-surface-700 dark:text-surface-100">{{ $t('admin.opsMonitor.noNodeErrors') }}</div>
              <div class="mt-1 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.noNodeErrorsHint') }}</div>
            </div>
          </div>
        </section>
      </div>

      <section class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h3 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.opsMonitor.businessTelemetry') }}</h3>
            <p class="mt-1 text-xs text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.businessTelemetryHint') }}</p>
          </div>
          <span class="inline-flex w-fit items-center rounded-full bg-surface-100 px-2.5 py-1 text-2xs font-semibold text-surface-700 ring-1 ring-inset ring-surface-200 dark:bg-surface-800 dark:text-surface-100 dark:ring-surface-600">
            {{ $t('admin.opsMonitor.notConnected') }}
          </span>
        </div>
        <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          <div
            v-for="panel in analysisPanels"
            :key="panel.title"
            class="min-h-[10rem] rounded-xl border border-surface-200 bg-surface-50 p-4 dark:border-surface-700 dark:bg-surface-800/55"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <h4 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ panel.title }}</h4>
                <p class="mt-1 text-2xs leading-4 text-surface-600 dark:text-surface-300">{{ panel.subtitle }}</p>
              </div>
              <span class="h-2 w-2 rounded-full bg-surface-400 dark:bg-surface-300" />
            </div>

            <div class="mt-5 h-16">
              <div v-if="panel.kind === 'bars'" class="flex h-full items-end gap-2 opacity-60">
                <span v-for="height in [28, 44, 22, 52, 36]" :key="height" class="w-full rounded-t bg-surface-300 dark:bg-surface-600" :style="{ height: height + 'px' }" />
              </div>
              <svg v-else-if="panel.kind === 'line'" class="h-full w-full opacity-60" viewBox="0 0 120 48" preserveAspectRatio="none">
                <path d="M0 36 C 20 30, 28 30, 42 34 S 70 42, 86 28 S 108 16, 120 20" fill="none" stroke="currentColor" stroke-width="2" class="text-surface-400 dark:text-surface-500" />
              </svg>
              <div v-else-if="panel.kind === 'stack'" class="flex h-full items-center gap-1 opacity-60">
                <span class="h-4 flex-[3] rounded-full bg-surface-300 dark:bg-surface-600" />
                <span class="h-4 flex-[2] rounded-full bg-surface-300 dark:bg-surface-600" />
                <span class="h-4 flex-1 rounded-full bg-surface-300 dark:bg-surface-600" />
              </div>
              <div v-else class="grid h-full grid-cols-6 items-center gap-2 opacity-60">
                <span v-for="size in [12, 8, 16, 10, 14, 8]" :key="size" class="rounded-full bg-surface-300 dark:bg-surface-600" :style="{ width: size + 'px', height: size + 'px' }" />
              </div>
            </div>

            <div class="mt-3 text-2xs font-semibold text-surface-600 dark:text-surface-300">{{ $t('admin.opsMonitor.notConnected') }}</div>
          </div>
        </div>
      </section>
    </section>
  </div>
</template>
