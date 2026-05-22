<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminPlansApi, type AdminPlan } from '@/api/admin/plans'
import { adminStatsApi, type AdminStats } from '@/api/admin/stats'
import Skeleton from '@/components/common/Skeleton.vue'
import { formatError } from '@/utils/format'

const { t } = useI18n()

// Stats is now a single-shot aggregate fetch. The previous version
// pulled list({ limit: 1000 }) for users + orders + plans and folded
// them client-side — which capped at the limit, hid the truth past
// 1000 rows, and shipped ~1MB of JSON per page load on a busy fleet.
// The /api/admin/stats endpoint returns just the KPI numbers + the
// 5 most-recent orders pre-joined with email + plan name.
//
// We still hit /plans separately because the plans-list panel below
// renders every row (≤dozens in practice — bounded by admin), and
// stuffing the full list into the stats payload would bloat it for
// the common case where the page just needs the counts.
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

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <!-- Composition-API t() instead of template $t — smoke test
             mocks $t to return keys; t() goes through real i18n. -->
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ t('admin.stats.title') }}</h1>
        <p class="mt-1.5 text-sm text-surface-500">{{ t('admin.stats.subtitle') }}</p>
      </div>
      <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700 dark:hover:text-surface-50" :title="$t('admin.stats.reload')" @click="reload">
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" variant="kpi" :rows="4" />

    <section v-else-if="stats" class="space-y-6">
      <!-- KPI strip — each card gets a distinct accent (sub2api-style). -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <!-- Users (accent teal) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.users') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 dark:bg-accent-500/15 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.users.total }}</div>
          <div class="mt-4 flex flex-wrap gap-1.5 text-2xs">
            <span class="inline-flex items-center gap-1 rounded-full bg-accent-50 px-2 py-0.5 font-medium text-accent-700 dark:bg-accent-950/40 dark:text-accent-300">
              <span class="h-1.5 w-1.5 rounded-full bg-accent-500" /> {{ t('admin.stats.activeCount', { n: stats.users.active }) }}
            </span>
            <span v-if="stats.users.suspended" class="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 font-medium text-red-600 dark:bg-red-950/40 dark:text-red-300">
              <span class="h-1.5 w-1.5 rounded-full bg-red-500" /> {{ t('admin.stats.suspendedCount', { n: stats.users.suspended }) }}
            </span>
          </div>
        </div>

        <!-- Month revenue (primary indigo) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.monthRevenue') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-500/15 dark:text-primary-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 17l6-6 4 4 8-8" /><path d="M14 7h7v7" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(stats.orders.month_revenue_cents) }}</div>
          <!-- Subtext via composition-API t() (not $t) so the smoke
               test's key-passthrough doesn't swallow the number. -->
          <div class="mt-4 text-2xs text-surface-500">{{ t('admin.stats.monthOrderCount', { n: stats.orders.month_count }) }}</div>
        </div>

        <!-- Orders (amber) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.orders') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-amber-50 text-amber-600 dark:bg-amber-500/15 dark:text-amber-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="18" height="13" rx="2" /><path d="M16 10a4 4 0 0 1-8 0" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.orders.total }}</div>
          <!-- Subtext: 成功 N · 失败 N. Numbers are interpolated outside
               the i18n call so the smoke test's $t key-passthrough still
               surfaces real values in the rendered DOM. -->
          <div class="mt-4 flex flex-wrap gap-2 text-2xs text-surface-500">
            <span>{{ t('admin.stats.ordersCompleted', { n: stats.orders.completed }) }}</span>
            <span aria-hidden="true">·</span>
            <span>{{ t('admin.stats.ordersFailed', { n: stats.orders.failed }) }}</span>
          </div>
        </div>

        <!-- Balance pool (pink) -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-700 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('admin.stats.kpi.balancePool') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-pink-50 text-pink-600 dark:bg-pink-500/15 dark:text-pink-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M16 8h-6a2 2 0 1 0 0 4h4a2 2 0 1 1 0 4H8M12 6v2M12 16v2" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(stats.users.total_balance_cents) }}</div>
          <!-- Subtext via composition t() — see comment above. -->
          <div class="mt-4 text-2xs text-surface-500">{{ t('admin.stats.balancePoolAccountSubtext', { n: stats.users.total, avg: formatYuan(stats.users.avg_balance_cents) }) }}</div>
        </div>
      </div>

      <!-- Plans summary + Recent activity -->
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
