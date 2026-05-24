<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminPlansApi, type AdminPlan } from '@/api/admin/plans'
import { adminStatsApi, type AdminStats, type TrafficRanking } from '@/api/admin/stats'
import Skeleton from '@/components/common/Skeleton.vue'
import { formatError } from '@/utils/format'

const { t } = useI18n()

// Stats is a single-shot aggregate fetch. /api/admin/stats now also
// returns traffic totals, top-node / top-user rankings, and audit
// severity counts — see backend/internal/handler/admin/stats.go for
// the wire shape.
const stats = ref<AdminStats | null>(null)
const plans = ref<AdminPlan[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [s, p] = await Promise.all([
      adminStatsApi.get(),
      adminPlansApi.list(),
    ])
    stats.value = s
    plans.value = p
  } catch (e: any) {
    error.value = formatError(e, t('admin.stats.loadFailed'))
  } finally {
    loading.value = false
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

// formatBytes renders bytes as a human-readable size. Matches the
// helper in admin/Status.vue — kept inline rather than extracted so
// this view stays compile-isolated (no cross-view import).
function formatBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = Math.abs(n)
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return (i === 0 ? v.toFixed(0) : v.toFixed(2)) + ' ' + units[i]
}

// monthDelta returns the signed percent change of new users this
// month vs last month. Returns null when both windows are zero (no
// meaningful comparison) so the template can hide the chip.
const monthDelta = computed<{ sign: string; percent: number; positive: boolean } | null>(() => {
  if (!stats.value) return null
  const cur = stats.value.users.month_new
  const prev = stats.value.users.prev_month_new
  if (cur === 0 && prev === 0) return null
  if (prev === 0) return { sign: '+', percent: 100, positive: true }
  const change = ((cur - prev) / prev) * 100
  const rounded = Math.round(change * 10) / 10
  return {
    sign: rounded > 0 ? '+' : rounded < 0 ? '-' : '',
    percent: Math.abs(rounded),
    positive: rounded >= 0,
  }
})

// rankingMax is the largest bytes value in a ranking list — drives
// the per-row progress bar width.
function rankingMax(rows: TrafficRanking[]): number {
  if (!rows.length) return 0
  return rows.reduce((m, r) => (r.bytes > m ? r.bytes : m), 0)
}

function rankingShare(row: TrafficRanking, max: number): number {
  if (max <= 0) return 0
  return Math.max(2, Math.round((row.bytes / max) * 100))
}

const auditTotal = computed(() => {
  if (!stats.value) return 0
  const a = stats.value.audit
  return a.info + a.warn + a.err
})

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ t('admin.stats.title') }}</h1>
        <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">{{ t('admin.stats.subtitle') }}</p>
      </div>
      <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700 dark:hover:text-surface-50" :title="$t('admin.stats.reload')" @click="reload">
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" variant="kpi" :rows="4" />

    <section v-else-if="stats" class="space-y-6">
      <!-- KPI strip: 4 cards matching screenshot row 2.
           今日收入 / 月收入 / 工单 / 佣金 dropped per product call —
           no income concept yet and no ticket/commission models. -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <!-- 月新增用户 (accent green for growth) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.monthNewUsers') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 dark:bg-accent-500/15 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM19 8v6M22 11h-6" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.users.month_new }}</div>
          <div v-if="monthDelta" class="mt-4 text-2xs">
            <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium ring-1 ring-inset"
              :class="monthDelta.positive
                ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'">
              <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path v-if="monthDelta.positive" d="M7 17l5-5 4 4 5-9" />
                <path v-else d="M7 7l5 5 4-4 5 9" />
              </svg>
              {{ t('admin.stats.kpiSubtitle.monthDelta', { sign: monthDelta.sign, percent: monthDelta.percent }) }}
            </span>
          </div>
        </div>

        <!-- 总用户 (accent indigo) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.totalUsers') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-500/15 dark:text-primary-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.users.total }}</div>
          <div class="mt-4 text-2xs text-surface-500">{{ t('admin.stats.kpiSubtitle.activeUsers', { n: stats.users.active }) }}</div>
        </div>

        <!-- 月上传 (sky) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.monthUpload') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-sky-50 text-sky-600 dark:bg-sky-500/15 dark:text-sky-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(stats.traffic.month_up_bytes) }}</div>
          <div class="mt-4 text-2xs text-surface-500">{{ t('admin.stats.kpiSubtitle.todayDelta', { value: formatBytes(stats.traffic.today_up_bytes) }) }}</div>
        </div>

        <!-- 月下载 (violet) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.monthDownload') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-violet-50 text-violet-600 dark:bg-violet-500/15 dark:text-violet-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(stats.traffic.month_down_bytes) }}</div>
          <div class="mt-4 text-2xs text-surface-500">{{ t('admin.stats.kpiSubtitle.todayDelta', { value: formatBytes(stats.traffic.today_down_bytes) }) }}</div>
        </div>
      </div>

      <!-- Traffic rankings: 2 panels side-by-side. Per-row progress
           bar is share-of-max within the panel (not of total), so the
           biggest consumer always reads 100% and the rest scale to it
           — matches the screenshot's bar lengths. -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">
                <svg class="h-4 w-4 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="5" r="3" /><circle cx="5" cy="19" r="3" /><circle cx="19" cy="19" r="3" /><path d="M12 8v3M12 11l-5 5M12 11l5 5" /></svg>
                {{ $t('admin.stats.nodeTrafficRanking') }}
              </h2>
              <p class="mt-1 text-xs text-surface-500">{{ $t('admin.stats.nodeTrafficRankingSubtitle') }}</p>
            </div>
            <span class="rounded-full bg-surface-100 px-2.5 py-0.5 text-2xs font-medium text-surface-600 dark:bg-surface-800 dark:text-surface-300">{{ $t('admin.stats.todayWindow') }}</span>
          </div>
          <ul class="mt-4 space-y-3">
            <li v-for="row in stats.top_nodes" :key="row.key" class="space-y-1.5">
              <div class="flex items-center justify-between text-sm">
                <span class="truncate font-medium text-ink-900 dark:text-surface-50">{{ row.key }}</span>
                <span class="ml-3 shrink-0 text-2xs tabular-nums text-surface-500">{{ formatBytes(row.bytes) }}</span>
              </div>
              <div class="h-1.5 rounded-full bg-surface-100 dark:bg-surface-800">
                <div class="h-1.5 rounded-full bg-ink-900 dark:bg-surface-50" :style="{ width: rankingShare(row, rankingMax(stats.top_nodes)) + '%' }" />
              </div>
            </li>
            <li v-if="stats.top_nodes.length === 0" class="text-xs text-surface-500">{{ $t('admin.stats.noTraffic') }}</li>
          </ul>
        </div>

        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">
                <svg class="h-4 w-4 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></svg>
                {{ $t('admin.stats.userTrafficRanking') }}
              </h2>
              <p class="mt-1 text-xs text-surface-500">{{ $t('admin.stats.userTrafficRankingSubtitle') }}</p>
            </div>
            <span class="rounded-full bg-surface-100 px-2.5 py-0.5 text-2xs font-medium text-surface-600 dark:bg-surface-800 dark:text-surface-300">{{ $t('admin.stats.todayWindow') }}</span>
          </div>
          <ul class="mt-4 space-y-3">
            <li v-for="row in stats.top_users" :key="row.key" class="space-y-1.5">
              <div class="flex items-center justify-between text-sm">
                <span class="truncate font-medium text-ink-900 dark:text-surface-50">{{ row.key }}</span>
                <span class="ml-3 shrink-0 text-2xs tabular-nums text-surface-500">{{ formatBytes(row.bytes) }}</span>
              </div>
              <div class="h-1.5 rounded-full bg-surface-100 dark:bg-surface-800">
                <div class="h-1.5 rounded-full bg-ink-900 dark:bg-surface-50" :style="{ width: rankingShare(row, rankingMax(stats.top_users)) + '%' }" />
              </div>
            </li>
            <li v-if="stats.top_users.length === 0" class="text-xs text-surface-500">{{ $t('admin.stats.noTraffic') }}</li>
          </ul>
        </div>
      </div>

      <!-- System log severity panel. Audit log already exists in this
           project; this strip just surfaces severity counts inline so
           the operator can spot incidents without leaving the page. -->
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h2 class="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">
              <svg class="h-4 w-4 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" /></svg>
              {{ $t('admin.stats.systemLog') }}
            </h2>
            <p class="mt-1 text-xs text-surface-500">{{ $t('admin.stats.systemLogSubtitle') }}</p>
          </div>
          <router-link to="/admin/audit" class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-surface-200 px-3 text-xs font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800">
            {{ $t('admin.stats.systemLogViewAll') }}
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14M13 5l7 7-7 7" /></svg>
          </router-link>
        </div>
        <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div class="rounded-xl bg-sky-50 px-4 py-3 dark:bg-sky-950/40">
            <div class="flex items-center gap-1.5 text-xs font-medium text-sky-700 dark:text-sky-300">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M12 16v-4M12 8h.01" /></svg>
              {{ $t('admin.stats.systemLogInfo') }}
            </div>
            <div class="mt-2 text-2xl font-semibold leading-none tabular-nums text-sky-700 dark:text-sky-200">{{ stats.audit.info }}</div>
          </div>
          <div class="rounded-xl bg-amber-50 px-4 py-3 dark:bg-amber-950/40">
            <div class="flex items-center gap-1.5 text-xs font-medium text-amber-700 dark:text-amber-300">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><path d="M12 9v4M12 17h.01" /></svg>
              {{ $t('admin.stats.systemLogWarn') }}
            </div>
            <div class="mt-2 text-2xl font-semibold leading-none tabular-nums text-amber-700 dark:text-amber-200">{{ stats.audit.warn }}</div>
          </div>
          <div class="rounded-xl bg-red-50 px-4 py-3 dark:bg-red-950/40">
            <div class="flex items-center gap-1.5 text-xs font-medium text-red-700 dark:text-red-300">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M15 9l-6 6M9 9l6 6" /></svg>
              {{ $t('admin.stats.systemLogErr') }}
            </div>
            <div class="mt-2 text-2xl font-semibold leading-none tabular-nums text-red-700 dark:text-red-200">{{ stats.audit.err }}</div>
          </div>
        </div>
        <div class="mt-3 text-2xs text-surface-500">{{ t('admin.stats.systemLogTotal', { n: auditTotal }) }}</div>
      </div>

      <!-- Plans summary + Recent activity — pre-existing, kept below
           the screenshot-aligned blocks because plans / orders are
           genuinely useful for an admin overview even though they
           don't appear in the reference design. -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.stats.plans') }}</h2>
          <p class="mt-1 text-xs text-surface-500">{{ $t('admin.stats.plansEnabledSummary', { enabled: stats.plans.enabled, disabled: stats.plans.disabled }) }}</p>
          <ul class="mt-4 space-y-2">
            <li v-for="p in plans" :key="p.id" class="flex items-center justify-between gap-3 rounded-lg border border-surface-100 px-3 py-2 dark:border-surface-800" :class="!p.enabled ? 'opacity-50' : ''">
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium text-ink-900 dark:text-surface-50">{{ p.name }}</div>
                <div class="mt-0.5 text-2xs text-surface-500">{{ $t('admin.stats.planTrafficLine', { days: p.duration_days, traffic: p.traffic_limit_bytes === 0 ? $t('admin.stats.unlimited') : Math.round(p.traffic_limit_bytes / 1024 / 1024 / 1024) + ' GB' }) }}</div>
              </div>
              <div class="text-sm font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ formatYuan(p.price_cents) }}</div>
            </li>
            <li v-if="plans.length === 0" class="text-xs text-surface-500">{{ $t('admin.stats.empty') }}</li>
          </ul>
        </div>

        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 lg:col-span-2 dark:border-surface-700 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ t('admin.stats.recentOrders') }}</h2>
          <p class="mt-1 text-xs text-surface-500">{{ $t('admin.stats.recentOrdersSubtitle') }}</p>
          <ul class="mt-4 space-y-2">
            <li v-for="o in stats.recent_orders" :key="o.id" class="flex items-center justify-between gap-3 rounded-lg border border-surface-100 px-3 py-2 text-sm dark:border-surface-800">
              <div class="min-w-0 flex-1">
                <div class="truncate text-ink-900 dark:text-surface-50">{{ o.user_email || `User #${o.user_id}` }} → {{ o.plan_name || `Plan #${o.plan_id}` }}</div>
                <div class="mt-0.5 text-2xs text-surface-500">{{ new Date(o.created_at).toLocaleString() }}</div>
              </div>
              <div class="flex items-center gap-2">
                <span class="font-medium tabular-nums text-ink-900 dark:text-surface-50">{{ formatYuan(o.price_cents) }}</span>
                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-medium ring-1 ring-inset"
                  :class="o.status === 'completed' || o.status === 'paid'
                    ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                    : o.status === 'failed'
                    ? 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'
                    : 'bg-surface-100 text-surface-500 ring-surface-200 dark:bg-surface-800 dark:text-surface-400 dark:ring-surface-700'">
                  {{ o.status }}
                </span>
              </div>
            </li>
            <li v-if="stats.recent_orders.length === 0" class="text-xs text-surface-500">{{ $t('admin.stats.emptyOrders') }}</li>
          </ul>
        </div>
      </div>
    </section>
  </div>
</template>
