<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { adminOrdersApi, type AdminOrder } from '@/api/admin/orders'
import { adminUsersApi, type AdminUser } from '@/api/admin/users'
import { adminPlansApi, type AdminPlan } from '@/api/admin/plans'
import Skeleton from '@/components/common/Skeleton.vue'
import { formatError } from '@/utils/format'

const orders = ref<AdminOrder[]>([])
const users = ref<AdminUser[]>([])
const plans = ref<AdminPlan[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [o, u, p] = await Promise.all([
      adminOrdersApi.list({ limit: 1000 }),
      adminUsersApi.list({ limit: 1000 }),
      adminPlansApi.list(),
    ])
    orders.value = o.orders
    users.value = u.users
    plans.value = p
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

// KPI computations — done client-side from the bulk lists.
// Backend doesn't ship a dedicated /stats endpoint yet (#future).

const userStats = computed(() => {
  const active = users.value.filter(u => u.status === 'active').length
  const suspended = users.value.filter(u => u.status === 'suspended').length
  return { total: users.value.length, active, suspended }
})

const planStats = computed(() => {
  const enabled = plans.value.filter(p => p.enabled).length
  return { total: plans.value.length, enabled, disabled: plans.value.length - enabled }
})

const orderStats = computed(() => {
  const completed = orders.value.filter(o => o.status === 'completed' || o.status === 'paid')
  const failed = orders.value.filter(o => o.status === 'failed').length
  const refunded = orders.value.filter(o => o.status === 'refunded').length
  const revenue = completed.reduce((s, o) => s + o.price_cents, 0)

  // This month's completed orders + revenue
  const now = new Date()
  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1).getTime()
  const monthCompleted = completed.filter(o => new Date(o.created_at).getTime() >= monthStart)
  const monthRevenue = monthCompleted.reduce((s, o) => s + o.price_cents, 0)

  return {
    total: orders.value.length,
    completed: completed.length,
    failed,
    refunded,
    revenue,
    monthCount: monthCompleted.length,
    monthRevenue,
  }
})

const balanceStats = computed(() => {
  const totalBalance = users.value.reduce((s, u) => s + u.balance_cents, 0)
  const avg = users.value.length > 0 ? totalBalance / users.value.length : 0
  return { total: totalBalance, avg }
})

// Recent activity — last 5 orders for the activity feed
const recentOrders = computed(() =>
  [...orders.value]
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .slice(0, 5),
)
const planNameById = computed(() => new Map(plans.value.map(p => [p.id, p.name])))
const userEmailById = computed(() => new Map(users.value.map(u => [u.id, u.email ?? `User #${u.id}`])))

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">统计</h1>
        <p class="mt-1.5 text-sm text-surface-500">运营概览 · 用户 / 订单 / 收入</p>
      </div>
      <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800" title="刷新" @click="reload">
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" variant="kpi" :rows="4" />

    <section v-else class="space-y-6">
      <!-- KPI strip -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">注册用户</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ userStats.total }}</div>
          <div class="mt-4 flex flex-wrap gap-1.5 text-2xs">
            <span class="inline-flex items-center gap-1 rounded-full bg-accent-50 px-2 py-0.5 font-medium text-accent-700 dark:bg-accent-950/40 dark:text-accent-300">
              <span class="h-1.5 w-1.5 rounded-full bg-accent-500" /> {{ userStats.active }} 正常
            </span>
            <span v-if="userStats.suspended" class="inline-flex items-center gap-1 rounded-full bg-red-50 px-2 py-0.5 font-medium text-red-600 dark:bg-red-950/40 dark:text-red-300">
              <span class="h-1.5 w-1.5 rounded-full bg-red-500" /> {{ userStats.suspended }} 封停
            </span>
          </div>
        </div>

        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">本月收入</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 17l6-6 4 4 8-8" /><path d="M14 7h7v7" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(orderStats.monthRevenue) }}</div>
          <div class="mt-4 text-2xs text-surface-500">{{ orderStats.monthCount }} 笔已完成 · 累计 {{ formatYuan(orderStats.revenue) }}</div>
        </div>

        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">订单总数</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="18" height="13" rx="2" /><path d="M16 10a4 4 0 0 1-8 0" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ orderStats.total }}</div>
          <div class="mt-4 flex flex-wrap gap-1.5 text-2xs text-surface-500">
            <span>✓ {{ orderStats.completed }}</span>
            <span v-if="orderStats.failed">⚠ {{ orderStats.failed }}</span>
            <span v-if="orderStats.refunded">↺ {{ orderStats.refunded }}</span>
          </div>
        </div>

        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">用户余额池</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M16 8h-6a2 2 0 1 0 0 4h4a2 2 0 1 1 0 4H8M12 6v2M12 16v2" /></svg>
            </div>
          </div>
          <div class="mt-3 text-display-sm font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(balanceStats.total) }}</div>
          <div class="mt-4 text-2xs text-surface-500">平均 {{ formatYuan(balanceStats.avg) }} / 用户</div>
        </div>
      </div>

      <!-- Plans summary + Recent activity -->
      <div class="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">套餐</h2>
          <p class="mt-1 text-xs text-surface-500">{{ planStats.enabled }} 启用 · {{ planStats.disabled }} 禁用</p>
          <ul class="mt-4 space-y-2">
            <li v-for="p in plans" :key="p.id" class="flex items-center justify-between gap-3 rounded-lg border border-surface-100 px-3 py-2 dark:border-surface-800" :class="!p.enabled ? 'opacity-50' : ''">
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium text-ink-900 dark:text-surface-50">{{ p.name }}</div>
                <div class="mt-0.5 text-2xs text-surface-500">{{ p.duration_days }} 天 · {{ p.traffic_limit_bytes === 0 ? '不限流量' : Math.round(p.traffic_limit_bytes / 1024 / 1024 / 1024) + ' GB' }}</div>
              </div>
              <div class="text-sm font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ formatYuan(p.price_cents) }}</div>
            </li>
            <li v-if="plans.length === 0" class="text-xs text-surface-500">还没有套餐</li>
          </ul>
        </div>

        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 lg:col-span-2 dark:border-surface-800 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">近期订单</h2>
          <p class="mt-1 text-xs text-surface-500">最近 5 单</p>
          <ul class="mt-4 space-y-2">
            <li v-for="o in recentOrders" :key="o.id" class="flex items-center justify-between gap-3 rounded-lg border border-surface-100 px-3 py-2 text-sm dark:border-surface-800">
              <div class="min-w-0 flex-1">
                <div class="truncate text-ink-900 dark:text-surface-50">{{ userEmailById.get(o.user_id) }} → {{ planNameById.get(o.plan_id) ?? `Plan #${o.plan_id}` }}</div>
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
            <li v-if="recentOrders.length === 0" class="text-xs text-surface-500">还没有订单</li>
          </ul>
        </div>
      </div>
    </section>
  </div>
</template>
