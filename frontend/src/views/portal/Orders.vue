<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { portalBillingApi, type Order, type Plan } from '@/api/portal/billing'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import AlipayPayModal from '@/components/portal/AlipayPayModal.vue'
import { formatError } from '@/utils/format'

const { t } = useI18n()

const orders = ref<Order[]>([])
const plansById = ref<Map<number, Plan>>(new Map())
const profile = ref<UserProfile | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
const refreshingOrderId = ref<number | null>(null)
const alipayModal = ref<{ open: boolean; order: Order | null }>({
  open: false,
  order: null,
})

async function load() {
  loading.value = true
  error.value = null
  try {
    const [o, p, pl] = await Promise.all([
      portalBillingApi.listOrders(),
      portalProfileApi.get(),
      portalBillingApi.listPlans(),
    ])
    orders.value = o
    profile.value = p
    // Backend's Order doesn't denormalize plan_name — look it up.
    plansById.value = new Map(pl.map(plan => [plan.id, plan]))
  } catch (e: any) {
    error.value = formatError(e, t('portal.orders.loadFailed'))
  } finally {
    loading.value = false
  }
}

function planName(planId: number): string {
  return plansById.value.get(planId)?.name ?? t('portal.orders.unknownPlan', { id: planId })
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

function methodLabel(method: Order['payment_method']): string {
  switch (method) {
    case 'alipay':
      return t('portal.orders.method.alipay')
    case 'stripe':
      return t('portal.orders.method.stripe')
    case 'balance':
      return t('portal.orders.method.balance')
    default:
      return method
  }
}

function statusPill(status: string): { cls: string; label: string } {
  switch (status) {
    case 'completed':
      return { cls: 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800', label: t('portal.orders.status.completed') }
    case 'paid':
      return { cls: 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800', label: t('portal.orders.status.paid') }
    case 'pending':
      return { cls: 'bg-primary-50 text-primary-700 ring-primary-100 dark:bg-primary-950/40 dark:text-primary-300 dark:ring-primary-800', label: t('portal.orders.status.pending') }
    case 'payment_pending':
      return { cls: 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800', label: t('portal.orders.status.paymentPending') }
    case 'payment_failed':
      return { cls: 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800', label: t('portal.orders.status.paymentFailed') }
    case 'payment_expired':
      return { cls: 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700', label: t('portal.orders.status.paymentExpired') }
    case 'failed':
      return { cls: 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800', label: t('portal.orders.status.failed') }
    case 'refunded':
      return { cls: 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800', label: t('portal.orders.status.refunded') }
    default:
      return { cls: 'bg-surface-100 text-surface-500', label: status }
  }
}

const sortedOrders = computed(() =>
  [...orders.value].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()),
)

function replaceOrder(order: Order) {
  const idx = orders.value.findIndex((o) => o.id === order.id)
  if (idx >= 0) orders.value.splice(idx, 1, order)
}

async function refreshOrder(order: Order): Promise<Order | null> {
  refreshingOrderId.value = order.id
  flash.value = null
  try {
    const fresh = await portalBillingApi.getOrder(order.id)
    replaceOrder(fresh)
    return fresh
  } catch (e) {
    flash.value = { kind: 'err', text: formatError(e, t('portal.orders.refreshFailed')) }
    return null
  } finally {
    refreshingOrderId.value = null
  }
}

async function continuePayment(order: Order) {
  const fresh = await refreshOrder(order)
  if (!fresh || fresh.status !== 'payment_pending') return
  if (!fresh.payment_target_url) {
    flash.value = { kind: 'err', text: t('portal.orders.paymentLinkMissing') }
    return
  }
  if (fresh.payment_method === 'stripe') {
    window.location.href = fresh.payment_target_url
    return
  }
  if (fresh.payment_method === 'alipay') {
    alipayModal.value = { open: true, order: fresh }
  }
}

function canContinuePayment(order: Order): boolean {
  return order.status === 'payment_pending' && (order.payment_method === 'alipay' || order.payment_method === 'stripe')
}

function onAlipaySuccess(order: Order) {
  replaceOrder(order)
  flash.value = { kind: 'ok', text: t('portal.orders.orderPaid', { id: order.id }) }
  setTimeout(() => {
    alipayModal.value = { open: false, order: null }
  }, 1000)
}

onMounted(load)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.orders.title') }}</h1>
        <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">{{ $t('portal.orders.subtitle') }}</p>
      </div>
      <div v-if="profile" class="text-right">
        <div class="text-xs text-surface-500">{{ $t('portal.orders.balance') }}</div>
        <div class="text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(profile.balance_cents) }}</div>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
    </p>

    <p
      v-if="flash"
      class="mb-4 rounded-xl px-4 py-3 text-sm ring-1 ring-inset"
      :class="flash.kind === 'ok'
        ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
        : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'"
    >
      {{ flash.text }}
    </p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <div
      v-else-if="sortedOrders.length > 0"
      class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
    >
      <table class="min-w-full text-sm">
        <thead class="text-left text-xs font-semibold uppercase tracking-wider text-surface-500 dark:text-surface-400">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">{{ $t('portal.orders.column.orderId') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('portal.orders.column.plan') }}</th>
            <th class="px-6 py-3 text-right font-medium">{{ $t('portal.orders.column.amount') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('portal.orders.column.method') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('portal.orders.column.status') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('portal.orders.column.createdAt') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('portal.orders.column.completedAt') }}</th>
            <th class="px-6 py-3 text-right font-medium">{{ $t('portal.orders.column.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="o in sortedOrders" :key="o.id" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-400 tabular-nums">#{{ o.id }}</td>
            <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ planName(o.plan_id) }}</td>
            <td class="px-6 py-3.5 text-right tabular-nums font-medium text-ink-900 dark:text-surface-50">{{ formatYuan(o.price_cents) }}</td>
            <td class="px-6 py-3.5 text-xs font-medium text-surface-600 dark:text-surface-300">{{ methodLabel(o.payment_method) }}</td>
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
            <td class="px-6 py-3.5 text-right">
              <button
                v-if="canContinuePayment(o)"
                type="button"
                :disabled="refreshingOrderId === o.id"
                class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-amber-200 px-2.5 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-amber-800 dark:text-amber-300 dark:hover:bg-amber-950/40"
                @click="continuePayment(o)"
              >
                <svg v-if="refreshingOrderId === o.id" class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round">
                  <path d="M21 12a9 9 0 1 1-6.2-8.55" />
                </svg>
                {{ $t('portal.orders.continuePayment') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="rounded-2xl border border-surface-100 bg-surface-0 px-6 py-14 text-center dark:border-surface-800 dark:bg-surface-900">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
        <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="18" height="13" rx="2" /><path d="M16 10a4 4 0 0 1-8 0" /></svg>
      </div>
      <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">{{ $t('portal.orders.empty') }}</h3>
      <p class="mt-1 text-xs text-surface-500">{{ $t('portal.orders.emptyDescription') }}</p>
      <RouterLink to="/portal/plans" class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500">
        {{ $t('portal.orders.seePlans') }}
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
      </RouterLink>
    </div>

    <AlipayPayModal
      :open="alipayModal.open"
      :order="alipayModal.order"
      @update:open="(v: boolean) => (alipayModal.open = v)"
      @success="onAlipaySuccess"
    />
  </div>
</template>
