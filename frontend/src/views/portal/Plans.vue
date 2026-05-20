<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { portalBillingApi, type Plan } from '@/api/portal/billing'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import { formatError } from '@/utils/format'

const router = useRouter()

const plans = ref<Plan[]>([])
const profile = ref<UserProfile | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const buying = ref<number | null>(null) // plan id currently being purchased
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    const [p, prof] = await Promise.all([portalBillingApi.listPlans(), portalProfileApi.get()])
    plans.value = p
    profile.value = prof
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
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
  if (!confirm(`确认购买「${plan.name}」？\n将从余额扣除 ${formatYuan(plan.price_cents)}。`)) return
  buying.value = plan.id
  flash.value = null
  try {
    const order = await portalBillingApi.purchase({ plan_id: plan.id, idempotency_key: uuid() })
    flash.value = { kind: 'ok', text: `订单 #${order.id} 已创建` }
    await load() // refresh balance
    setTimeout(() => router.push('/portal/orders'), 800)
  } catch (e) {
    flash.value = { kind: 'err', text: formatError(e, '购买失败') }
  } finally {
    buying.value = null
  }
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
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">套餐</h1>
        <p class="mt-1.5 text-sm text-surface-500">从余额购买 · 立即开通</p>
      </div>
      <div v-if="profile" class="text-right">
        <div class="text-xs text-surface-500">当前余额</div>
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

    <section v-else-if="sortedPlans.length > 0" class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
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
            <span><b class="font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ plan.traffic_limit_bytes === 0 ? "∞" : formatTraffic(plan.traffic_limit_bytes) }}</b> {{ plan.traffic_limit_bytes === 0 ? "不限流量" : "流量" }}</span>
          </li>
          <li class="flex items-start gap-2">
            <svg class="mt-0.5 h-4 w-4 shrink-0 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
            <span><b class="font-semibold tabular-nums text-ink-900 dark:text-surface-50">{{ plan.duration_days }}</b> 天有效期</span>
          </li>
          <li v-if="plan.ip_limit" class="flex items-start gap-2">
            <svg class="mt-0.5 h-4 w-4 shrink-0 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
            <span>最多 <b class="font-semibold text-ink-900 dark:text-surface-50">{{ plan.ip_limit }}</b> 个 IP</span>
          </li>
        </ul>

        <button
          type="button"
          :disabled="buying === plan.id || !canAfford(plan)"
          class="mt-6 inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-xl bg-ink-900 px-4 text-sm font-semibold text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 dark:bg-accent-600 dark:hover:bg-accent-500"
          @click="buy(plan)"
        >
          <svg v-if="buying === plan.id" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
          <template v-else>
            <span v-if="!canAfford(plan)">余额不足</span>
            <span v-else>立即购买</span>
          </template>
        </button>
      </div>
    </section>

    <div v-else class="rounded-2xl border border-surface-100 bg-surface-0 px-6 py-16 text-center dark:border-surface-800 dark:bg-surface-900">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
        <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M9 11l3 3L22 4M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></svg>
      </div>
      <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">还没有上架套餐</h3>
      <p class="mt-1 text-xs text-surface-500">管理员还没创建任何套餐，稍候再来看看</p>
    </div>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
