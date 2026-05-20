<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { adminOrdersApi, type AdminOrder } from '@/api/admin/orders'
import { adminPlansApi, type AdminPlan } from '@/api/admin/plans'
import { adminUsersApi, type AdminUser } from '@/api/admin/users'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const orders = ref<AdminOrder[]>([])
const plansById = ref<Map<number, AdminPlan>>(new Map())
const usersById = ref<Map<number, AdminUser>>(new Map())
const loading = ref(true)
const error = ref<string | null>(null)
const statusFilter = ref<'all' | AdminOrder['status']>('all')

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [o, p, u] = await Promise.all([
      adminOrdersApi.list({ limit: 200 }),
      adminPlansApi.list(),
      adminUsersApi.list({ limit: 500 }),
    ])
    orders.value = o.orders
    plansById.value = new Map(p.map(plan => [plan.id, plan]))
    usersById.value = new Map(u.users.map(user => [user.id, user]))
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

function planName(id: number): string {
  return plansById.value.get(id)?.name ?? `Plan #${id}`
}
function userEmail(id: number): string {
  return usersById.value.get(id)?.email ?? `User #${id}`
}
function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

function statusPill(s: string): { cls: string; label: string } {
  switch (s) {
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
      return { cls: 'bg-surface-100 text-surface-500', label: s }
  }
}

const filtered = computed(() => {
  const list = statusFilter.value === 'all'
    ? orders.value
    : orders.value.filter(o => o.status === statusFilter.value)
  return [...list].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
})

const stats = computed(() => {
  const total = orders.value.length
  const completed = orders.value.filter(o => o.status === 'completed' || o.status === 'paid').length
  const revenue = orders.value
    .filter(o => o.status === 'completed' || o.status === 'paid')
    .reduce((s, o) => s + o.price_cents, 0)
  return { total, completed, revenue }
})

// Refund: only completed / paid orders are eligible. Admin can
// provide an optional reason that gets recorded in the user's
// balance-ledger note for the +PriceCents credit.
const refundingId = ref<number | null>(null)
const refundErr = ref<string | null>(null)

async function refund(o: AdminOrder) {
  const ok = await askConfirm({
    title: '退款',
    message: `订单 #${o.id}（${planName(o.plan_id)}，${formatYuan(o.price_cents)}）将退款给 ${userEmail(o.user_id)}，余额自动 +${formatYuan(o.price_cents)}。客户端不会自动停用，需要的话去 inbounds 页手动处理。`,
    variant: 'danger',
    confirmLabel: '退款',
  })
  if (!ok) return
  refundingId.value = o.id
  refundErr.value = null
  try {
    const updated = await adminOrdersApi.refund(o.id, 'admin manual refund')
    // Optimistic replace so the user sees the chip flip without a full reload.
    const i = orders.value.findIndex(x => x.id === o.id)
    if (i >= 0) orders.value.splice(i, 1, updated)
  } catch (e: any) {
    refundErr.value = formatError(e, '退款失败')
  } finally {
    refundingId.value = null
  }
}

function refundable(s: string): boolean {
  return s === 'completed' || s === 'paid'
}

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">订单管理</h1>
        <p class="mt-1.5 text-sm text-surface-500">全部订单 · 跨用户</p>
      </div>
      <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800" title="刷新" @click="reload">
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <!-- Summary stats -->
    <div v-if="!loading && orders.length > 0" class="mb-5 grid grid-cols-3 gap-3">
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900">
        <div class="text-2xs font-medium text-surface-500">总订单</div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.total }}</div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900">
        <div class="text-2xs font-medium text-surface-500">已完成</div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.completed }}</div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900">
        <div class="text-2xs font-medium text-surface-500">完成订单总额</div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-accent-600 tabular-nums dark:text-accent-400">{{ formatYuan(stats.revenue) }}</div>
      </div>
    </div>

    <!-- Status filter chips -->
    <div v-if="!loading && orders.length > 0" class="mb-4 flex h-9 items-center gap-0.5 rounded-xl border border-surface-200 bg-surface-0 p-1 text-xs dark:border-surface-700 dark:bg-surface-900 w-fit">
      <button v-for="s in ['all','completed','failed','refunded','created'] as const" :key="s" type="button"
        class="rounded-lg px-3 py-1 font-medium transition-all duration-150 ease-brand"
        :class="statusFilter === s ? 'bg-ink-900 text-white shadow-card dark:bg-accent-600' : 'text-surface-500 hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-50'"
        @click="statusFilter = s">
        {{ s === 'all' ? '全部' : statusPill(s).label }}
      </button>
    </div>

    <Skeleton v-if="loading" :rows="6" />

    <div v-else-if="filtered.length > 0" class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">订单号</th>
            <th class="px-6 py-3 font-medium">用户</th>
            <th class="px-6 py-3 font-medium">套餐</th>
            <th class="px-6 py-3 text-right font-medium">金额</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">下单</th>
            <th class="px-6 py-3 font-medium">完成</th>
            <th class="px-6 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="o in filtered" :key="o.id" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-400 tabular-nums">#{{ o.id }}</td>
            <td class="px-6 py-3.5">
              <div class="text-sm text-ink-900 dark:text-surface-50">{{ userEmail(o.user_id) }}</div>
              <div class="mt-0.5 font-mono text-2xs text-surface-400">user #{{ o.user_id }}</div>
            </td>
            <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ planName(o.plan_id) }}</td>
            <td class="px-6 py-3.5 text-right tabular-nums font-medium text-ink-900 dark:text-surface-50">{{ formatYuan(o.price_cents) }}</td>
            <td class="px-6 py-3.5">
              <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset" :class="statusPill(o.status).cls">{{ statusPill(o.status).label }}</span>
              <div v-if="o.error_message" class="mt-1 text-2xs text-red-600">{{ o.error_message }}</div>
            </td>
            <td class="px-6 py-3.5 text-xs text-surface-500">{{ new Date(o.created_at).toLocaleString() }}</td>
            <td class="px-6 py-3.5 text-xs text-surface-500">{{ o.completed_at ? new Date(o.completed_at).toLocaleString() : '—' }}</td>
            <td class="px-6 py-3.5 text-right">
              <button
                v-if="refundable(o.status)"
                type="button"
                class="rounded-lg border border-amber-200 px-2.5 py-1 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-50 disabled:opacity-50 dark:border-amber-800 dark:text-amber-300 dark:hover:bg-amber-950/40"
                :disabled="refundingId === o.id"
                @click="refund(o)"
              >
                {{ refundingId === o.id ? '处理中…' : '退款' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else
      icon="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"
      title="没有订单"
      :description="orders.length === 0 ? '还没有用户购买套餐。' : '当前过滤条件下没有订单。'"
    />

    <p v-if="refundErr" class="mt-3 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800">{{ refundErr }}</p>
    <ConfirmModal
      v-if="confirmState"
      :open="confirmState.open"
      :title="confirmState.title"
      :message="confirmState.message"
      :confirm-label="confirmState.confirmLabel"
      :variant="confirmState.variant"
      @confirm="settleConfirm(true)"
      @cancel="settleConfirm(false)"
    />
  </div>
</template>
