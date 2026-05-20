<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { portalBillingApi, type Order } from '@/api/portal/billing'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import { formatError } from '@/utils/format'

const orders = ref<Order[]>([])
const profile = ref<UserProfile | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    const [o, p] = await Promise.all([portalBillingApi.listOrders(), portalProfileApi.get()])
    orders.value = o
    profile.value = p
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

function statusPill(status: string): { cls: string; label: string } {
  switch (status) {
    case 'completed':
    case 'paid':
      return { cls: 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800', label: '已完成' }
    case 'failed':
      return { cls: 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800', label: '失败' }
    case 'refunded':
      return { cls: 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800', label: '已退款' }
    case 'created':
      return { cls: 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700', label: '处理中' }
    default:
      return { cls: 'bg-surface-100 text-surface-500', label: status }
  }
}

const sortedOrders = computed(() =>
  [...orders.value].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()),
)

onMounted(load)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">订单</h1>
        <p class="mt-1.5 text-sm text-surface-500">购买历史 · 余额变动</p>
      </div>
      <div v-if="profile" class="text-right">
        <div class="text-xs text-surface-500">余额</div>
        <div class="text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(profile.balance_cents) }}</div>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
    </p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <div
      v-else-if="sortedOrders.length > 0"
      class="overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
    >
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">订单号</th>
            <th class="px-6 py-3 font-medium">套餐</th>
            <th class="px-6 py-3 text-right font-medium">金额</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">下单时间</th>
            <th class="px-6 py-3 font-medium">完成时间</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="o in sortedOrders" :key="o.id" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-400 tabular-nums">#{{ o.id }}</td>
            <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ o.plan_name }}</td>
            <td class="px-6 py-3.5 text-right tabular-nums font-medium text-ink-900 dark:text-surface-50">{{ formatYuan(o.amount_cents) }}</td>
            <td class="px-6 py-3.5">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                :class="statusPill(o.status).cls"
              >
                {{ statusPill(o.status).label }}
              </span>
            </td>
            <td class="px-6 py-3.5 text-xs text-surface-500">{{ new Date(o.created_at).toLocaleString() }}</td>
            <td class="px-6 py-3.5 text-xs text-surface-500">{{ o.completed_at ? new Date(o.completed_at).toLocaleString() : '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="rounded-2xl border border-surface-100 bg-surface-0 px-6 py-14 text-center dark:border-surface-800 dark:bg-surface-900">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
        <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="18" height="13" rx="2" /><path d="M16 10a4 4 0 0 1-8 0" /></svg>
      </div>
      <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">还没有订单</h3>
      <p class="mt-1 text-xs text-surface-500">去套餐页选一个开通服务</p>
      <RouterLink to="/portal/plans" class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500">
        看套餐
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
      </RouterLink>
    </div>
  </div>
</template>
