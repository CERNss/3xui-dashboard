<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { portalBillingApi, type Plan, type PaymentMethod, type Order } from '@/api/portal/billing'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import AlipayPayModal from '@/components/portal/AlipayPayModal.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { t } = useI18n()
const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const router = useRouter()

const plans = ref<Plan[]>([])
const profile = ref<UserProfile | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const buying = ref<number | null>(null) // plan id currently being purchased
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

// Available payment methods from the backend. Always includes "balance";
// "alipay" only appears when the operator configured ALIPAY_APP_ID etc.
const paymentMethods = ref<PaymentMethod[]>(['balance'])
const selectedMethod = ref<PaymentMethod>('balance')

// Alipay QR modal state. order is the payment_pending order we just
// created — the modal polls /orders/:id and flips to "支付成功" when
// notify advances it to completed.
const alipayModal = ref<{ open: boolean; order: Order | null }>({
  open: false,
  order: null,
})

async function load() {
  loading.value = true
  error.value = null
  try {
    const [p, prof, methods] = await Promise.all([
      portalBillingApi.listPlans(),
      portalProfileApi.get(),
      portalBillingApi.paymentMethods(),
    ])
    plans.value = p
    profile.value = prof
    paymentMethods.value = methods.length > 0 ? methods : ['balance']
    // Default to whichever method the user picked last; otherwise the
    // first method the backend returned (balance, then alipay).
    if (!paymentMethods.value.includes(selectedMethod.value)) {
      selectedMethod.value = paymentMethods.value[0]
    }
  } catch (e: any) {
    error.value = formatError(e, t('portal.plans.loadFailed'))
  } finally {
    loading.value = false
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

function formatTraffic(bytes: number): string {
  // Backend stores limit in bytes. Plans are usually whole GB so round
  // sensibly: ≥1024 GB → TB, else GB. 0 is handled by the caller.
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1024) return (gb / 1024).toFixed(1) + ' TB'
  if (gb >= 1)    return Math.round(gb) + ' GB'
  return Math.round(bytes / (1024 * 1024)) + ' MB'
}

function canAfford(plan: Plan): boolean {
  if (!profile.value) return false
  return profile.value.balance_cents >= plan.price_cents
}

function canBuy(plan: Plan): boolean {
  return selectedMethod.value !== 'balance' || canAfford(plan)
}

function uuid(): string {
  // Lightweight RFC4122 v4 — good enough for client-side idempotency keys.
  if (crypto?.randomUUID) return crypto.randomUUID()
  const a = new Uint8Array(16)
  crypto.getRandomValues(a)
  a[6] = (a[6] & 0x0f) | 0x40
  a[8] = (a[8] & 0x3f) | 0x80
  const h = [...a].map(b => b.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

async function buy(plan: Plan) {
  const methodLabel =
    selectedMethod.value === 'alipay' ? t('portal.plans.method.alipay')
    : selectedMethod.value === 'stripe' ? t('portal.plans.method.stripe')
    : t('portal.plans.method.balance')
  const messageBody =
    selectedMethod.value === 'balance'
      ? t('portal.plans.confirmBalanceMsg', { amount: formatYuan(plan.price_cents) })
      : t('portal.plans.confirmPayMsg', { method: methodLabel, amount: formatYuan(plan.price_cents) })
  const ok = await askConfirm({
    title: t('portal.plans.confirmTitle', { name: plan.name }),
    message: messageBody,
    confirmLabel: t('portal.plans.confirmPayBtn', { method: methodLabel, amount: formatYuan(plan.price_cents) }),
  })
  if (!ok) return
  buying.value = plan.id
  flash.value = null
  try {
    const input = {
      plan_id: plan.id,
      idempotency_key: uuid(),
    }
    if (selectedMethod.value === 'alipay') {
      const order = await portalBillingApi.purchaseViaPayment('alipay', input)
      alipayModal.value = { open: true, order }
      return
    }
    if (selectedMethod.value === 'stripe') {
      const order = await portalBillingApi.purchaseViaPayment('stripe', input)
      // Stripe Checkout is a hosted page — payment_target_url is the
      // redirect target, not a QR source. We leave the page; the
      // success/cancel URLs configured server-side bring the user
      // back to /portal/orders or /portal/plans.
      if (order.payment_target_url) {
        window.location.href = order.payment_target_url
        return
      }
      flash.value = { kind: 'err', text: t('portal.plans.stripeNoUrl') }
      return
    }
    const order = await portalBillingApi.purchase(input)
    flash.value = { kind: 'ok', text: t('portal.plans.orderCreated', { id: order.id }) }
    await load() // refresh balance
    setTimeout(() => router.push('/portal/orders'), 800)
  } catch (e) {
    flash.value = { kind: 'err', text: formatError(e, t('portal.plans.purchaseFailed')) }
  } finally {
    buying.value = null
  }
}

function onAlipaySuccess(order: Order) {
  flash.value = { kind: 'ok', text: t('portal.plans.orderPaid', { id: order.id }) }
  // Close the modal after a brief success indicator, then refresh
  // balance + route to /portal/orders. The modal itself shows the
  // success state for ~800ms before we close it.
  setTimeout(() => {
    alipayModal.value = { open: false, order: null }
    void load()
    router.push('/portal/orders')
  }, 1000)
}

const sortedPlans = computed(() =>
  [...plans.value].filter(p => p.enabled).sort((a, b) => a.price_cents - b.price_cents),
)

onMounted(load)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.plans.title') }}</h1>
        <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">{{ $t('portal.plans.subtitle') }}</p>
      </div>
      <div v-if="profile" class="text-right">
        <div class="text-xs text-surface-500">{{ $t('portal.plans.currentBalance') }}</div>
        <div class="text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatYuan(profile.balance_cents) }}</div>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
    </p>

    <Transition name="fade">
      <p
        v-if="flash"
        class="mb-4 rounded-xl px-4 py-3 text-sm ring-1 ring-inset"
        :class="flash.kind === 'ok'
          ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
          : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'"
      >
        {{ flash.text }}
      </p>
    </Transition>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <!-- Payment method picker — hidden when only "balance" is configured -->
    <div
      v-if="!loading && paymentMethods.length > 1"
      class="mb-5 rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900"
    >
      <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.plans.methodPickerTitle') }}</h2>
      <p class="mt-1 text-xs text-surface-500">{{ $t('portal.plans.methodPickerHint') }}</p>
      <div class="mt-4 grid grid-cols-2 gap-2 md:grid-cols-3">
        <label
          v-for="m in paymentMethods"
          :key="m"
          class="flex cursor-pointer items-center gap-2.5 rounded-xl border p-3 transition-all duration-150 ease-brand"
          :class="selectedMethod === m
            ? 'border-accent-300 bg-accent-50 dark:border-accent-700 dark:bg-accent-950/40'
            : 'border-surface-200 bg-surface-0 hover:border-surface-300 hover:bg-surface-50 dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800'"
        >
          <input
            type="radio"
            :value="m"
            v-model="selectedMethod"
            class="h-4 w-4 border-surface-300 text-accent-600 focus:ring-accent-500/30"
          />
          <span class="text-sm font-medium" :class="selectedMethod === m ? 'text-accent-700 dark:text-accent-300' : 'text-ink-900 dark:text-surface-50'">
            <template v-if="m === 'alipay'">{{ $t('portal.plans.method.alipay') }}</template>
            <template v-else-if="m === 'balance'">{{ $t('portal.plans.method.balance') }}</template>
            <template v-else-if="m === 'stripe'">{{ $t('portal.plans.method.stripe') }}</template>
            <template v-else>{{ m }}</template>
          </span>
        </label>
      </div>
    </div>

    <section v-if="!loading && sortedPlans.length > 0" class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
      <div
        v-for="plan in sortedPlans"
        :key="plan.id"
        class="group relative flex flex-col rounded-2xl border border-surface-100 bg-surface-0 p-6 transition-all duration-200 ease-brand hover:-translate-y-0.5 hover:border-accent-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900 dark:hover:border-accent-700"
      >
        <div class="flex items-start justify-between">
          <h3 class="text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ plan.name }}</h3>
        </div>
        <div class="mt-4 flex items-baseline gap-1">
          <span class="text-display-sm font-semibold leading-none tracking-tight text-ink-900 dark:text-surface-50">{{ formatYuan(plan.price_cents) }}</span>
        </div>

        <ul class="mt-5 space-y-2.5 text-sm text-surface-600 dark:text-surface-300">
          <li class="flex items-start gap-2">
            <svg class="mt-0.5 h-4 w-4 shrink-0 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
            <span><b class="font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ plan.traffic_limit_bytes === 0 ? "∞" : formatTraffic(plan.traffic_limit_bytes) }}</b> {{ plan.traffic_limit_bytes === 0 ? $t('portal.plans.unlimitedTraffic') : $t('portal.plans.trafficLabel') }}</span>
          </li>
          <li class="flex items-start gap-2">
            <svg class="mt-0.5 h-4 w-4 shrink-0 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
            <span>{{ $t('portal.plans.planDays', { days: plan.duration_days }) }}</span>
          </li>
          <li v-if="plan.ip_limit" class="flex items-start gap-2">
            <svg class="mt-0.5 h-4 w-4 shrink-0 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
            <span>{{ $t('portal.plans.ipLimitText', { n: plan.ip_limit }) }}</span>
          </li>
        </ul>

        <button
          type="button"
          :disabled="buying === plan.id || !canBuy(plan)"
          class="mt-6 inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-xl bg-ink-900 px-4 text-sm font-semibold text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 dark:bg-accent-600 dark:hover:bg-accent-500"
          @click="buy(plan)"
        >
          <svg v-if="buying === plan.id" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
          <template v-else>
            <span v-if="selectedMethod === 'balance' && !canAfford(plan)">{{ $t('portal.plans.balanceInsufficient') }}</span>
            <span v-else>{{ $t('portal.plans.buyNow') }}</span>
          </template>
        </button>
      </div>
    </section>

    <div v-else-if="!loading" class="rounded-2xl border border-surface-100 bg-surface-0 px-6 py-16 text-center dark:border-surface-800 dark:bg-surface-900">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
        <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M9 11l3 3L22 4M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></svg>
      </div>
      <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">{{ $t('portal.plans.empty') }}</h3>
      <p class="mt-1 text-xs text-surface-500">{{ $t('portal.plans.emptyDescription') }}</p>
    </div>

    <ConfirmModal
      v-if="confirmState"
      :open="confirmState.open"
      :title="confirmState.title"
      :message="confirmState.message"
      :variant="confirmState.variant"
      :confirm-label="confirmState.confirmLabel"
      :cancel-label="confirmState.cancelLabel"
      :busy="confirmState.busy"
      @confirm="settleConfirm(true)"
      @cancel="settleConfirm(false)"
    />

    <AlipayPayModal
      :open="alipayModal.open"
      :order="alipayModal.order"
      @update:open="(v: boolean) => (alipayModal.open = v)"
      @success="onAlipaySuccess"
    />
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
