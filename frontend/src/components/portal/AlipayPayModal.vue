<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import QRCode from 'qrcode'

import { portalBillingApi, type Order } from '@/api/portal/billing'

interface Props {
  open: boolean
  /** The order created by purchaseViaPayment('alipay', ...). Contains
   *  the qr_url to render and the id we poll for status. */
  order: Order | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:open', v: boolean): void
  /** Fired when the order flips to completed — parent should refresh
   *  balance / route to /portal/orders. */
  (e: 'success', order: Order): void
}>()

// State machine: 'waiting' (QR shown, polling) → 'success' | 'failed'
// | 'expired'. Modal stays open in terminal states so user can see
// what happened; the parent shows the final message + close button.
type State = 'waiting' | 'success' | 'failed' | 'expired'
const state = ref<State>('waiting')
const qrDataURL = ref<string>('')
const remainingSec = ref(0)

let pollTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null

const expiresAtMs = computed(() => {
  if (!props.order?.payment_expires_at) return 0
  return new Date(props.order.payment_expires_at).getTime()
})

async function renderQR(text: string) {
  try {
    qrDataURL.value = await QRCode.toDataURL(text, {
      width: 240,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#0c0e12', light: '#ffffff' },
    })
  } catch {
    qrDataURL.value = ''
  }
}

async function pollOnce() {
  if (!props.order) return
  try {
    const fresh = await portalBillingApi.getOrder(props.order.id)
    if (fresh.status === 'completed') {
      state.value = 'success'
      stopTimers()
      emit('success', fresh)
      return
    }
    if (fresh.status === 'payment_failed' || fresh.status === 'refunded') {
      state.value = 'failed'
      stopTimers()
      return
    }
    if (fresh.status === 'payment_expired') {
      state.value = 'expired'
      stopTimers()
      return
    }
  } catch {
    // Transient — keep polling
  }
}

function startTimers() {
  state.value = 'waiting'
  stopTimers()
  // Poll every 3s — alipay's notify usually fires within 1-2s of
  // user confirmation, so we'll catch most successes on the first
  // poll after the user finishes.
  pollTimer = setInterval(pollOnce, 3000)
  // Countdown until QR expiry (driven by payment_expires_at)
  countdownTimer = setInterval(() => {
    if (!expiresAtMs.value) return
    const diff = Math.floor((expiresAtMs.value - Date.now()) / 1000)
    remainingSec.value = diff > 0 ? diff : 0
    if (diff <= 0 && state.value === 'waiting') {
      state.value = 'expired'
      stopTimers()
    }
  }, 1000)
}

function stopTimers() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
}

function close() {
  stopTimers()
  emit('update:open', false)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

onMounted(() => {
  if (props.open) document.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  document.removeEventListener('keydown', onKey)
  stopTimers()
})

// Re-render QR + restart timers whenever the order changes (e.g.
// "重试" creates a new order with a new qr_url).
watch(
  () => props.order,
  (o) => {
    if (o?.payment_target_url) {
      renderQR(o.payment_target_url)
      startTimers()
    }
  },
  { immediate: true },
)

watch(
  () => props.open,
  (v) => {
    if (v) {
      document.addEventListener('keydown', onKey)
      if (props.order?.payment_target_url) startTimers()
    } else {
      document.removeEventListener('keydown', onKey)
      stopTimers()
    }
  },
)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="close"
    >
      <div class="w-full max-w-sm animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <div class="px-6 pt-6">
          <div class="flex items-start justify-between">
            <div>
              <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">支付宝支付</h2>
              <p v-if="order" class="mt-1 text-xs text-surface-500">订单 #{{ order.id }} · ¥{{ (order.price_cents / 100).toFixed(2) }}</p>
            </div>
            <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="close">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
            </button>
          </div>
        </div>

        <!-- Waiting state — QR + countdown -->
        <div v-if="state === 'waiting'" class="px-6 py-5">
          <div class="flex justify-center">
            <div class="rounded-2xl border border-surface-100 bg-surface-50 p-3 dark:border-surface-800 dark:bg-surface-800">
              <img v-if="qrDataURL" :src="qrDataURL" alt="alipay qr" class="h-60 w-60 rounded-lg" />
              <div v-else class="flex h-60 w-60 items-center justify-center text-xs text-surface-400">二维码生成中…</div>
            </div>
          </div>
          <p class="mt-4 text-center text-xs text-surface-500">使用支付宝 App 扫码完成支付</p>
          <p v-if="remainingSec > 0" class="mt-1 text-center text-2xs text-surface-400 tabular-nums">
            二维码 {{ Math.floor(remainingSec / 60) }}:{{ String(remainingSec % 60).padStart(2, '0') }} 后失效
          </p>
          <a
            v-if="order?.payment_target_url"
            :href="order.payment_target_url"
            class="mt-4 inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-xl bg-[#1677ff] px-4 text-sm font-medium text-white shadow-card transition-all hover:opacity-90 active:scale-[0.98]"
          >
            打开支付宝 APP
            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </a>
        </div>

        <!-- Success state -->
        <div v-else-if="state === 'success'" class="px-6 py-8 text-center">
          <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300">
            <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
          </div>
          <h3 class="mt-3 text-sm font-semibold text-ink-900 dark:text-surface-50">支付成功</h3>
          <p class="mt-1 text-xs text-surface-500">客户端已开通，正在跳转订单列表…</p>
        </div>

        <!-- Failed / expired state -->
        <div v-else class="px-6 py-8 text-center">
          <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-300">
            <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" /><path d="M12 9v4M12 17h.01" /></svg>
          </div>
          <h3 class="mt-3 text-sm font-semibold text-ink-900 dark:text-surface-50">
            {{ state === 'failed' ? '支付失败' : '二维码已过期' }}
          </h3>
          <p class="mt-1 text-xs text-surface-500">
            {{ state === 'failed' ? '支付未完成，请重新下单' : '请关闭此窗口后重新购买' }}
          </p>
          <button
            type="button"
            class="mt-4 inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
            @click="close"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
