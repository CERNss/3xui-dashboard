<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { portalTrafficApi, type ClientUsage } from '@/api/portal/traffic'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import { formatError } from '@/utils/format'

const { t } = useI18n()

const clients = ref<ClientUsage[]>([])
const profile = ref<UserProfile | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [p, c] = await Promise.all([portalProfileApi.get(), portalTrafficApi.own()])
    profile.value = p
    clients.value = c
  } catch (e: any) {
    error.value = formatError(e, t('portal.dashboard.loadFailed'))
  } finally {
    loading.value = false
  }
}

function formatBytes(n: number | null | undefined): string {
  if (!n) return '0 B'
  if (n < 1024) return n + ' B'
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return v.toFixed(2) + ' ' + units[i]
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

// Days until the soonest expiry across the user's clients. Returns null when
// the user has no clients or no expiries set.
const daysToExpiry = computed(() => {
  const now = Date.now()
  let soonest = Infinity
  for (const c of clients.value) {
    if (!c.expires_at) continue
    const ms = new Date(c.expires_at).getTime() - now
    if (ms < soonest) soonest = ms
  }
  if (!Number.isFinite(soonest)) return null
  return Math.max(0, Math.floor(soonest / (24 * 3600 * 1000)))
})

const totalUsed = computed(() => clients.value.reduce((s, c) => s + (c.up || 0) + (c.down || 0), 0))
const totalLimit = computed(() => clients.value.reduce((s, c) => s + (c.limit || 0), 0))
const usagePct = computed(() => {
  if (totalLimit.value <= 0) return 0
  return Math.min(100, Math.round((totalUsed.value / totalLimit.value) * 100))
})

const subURL = computed(() => {
  if (!profile.value) return ''
  return location.origin + '/sub/' + profile.value.sub_id
})

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">
          {{ profile?.email ? $t('portal.dashboard.hi', { name: profile.email.split('@')[0] }) : $t('portal.dashboard.welcome') }}
        </h1>
        <p class="mt-1.5 text-sm text-surface-500">{{ $t('portal.dashboard.subtitle') }}</p>
      </div>
      <button
        class="inline-flex h-9 items-center gap-1.5 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800"
        @click="reload"
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        {{ $t('portal.dashboard.refresh') }}
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
    </p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <section v-else class="space-y-5">
      <!-- KPI strip -->
      <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
        <!-- 总用量 -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('portal.dashboard.usedTraffic') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 17l6-6 4 4 8-8" /><path d="M14 7h7v7" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-2xl font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(totalUsed) }}</span>
          </div>
          <div v-if="totalLimit > 0" class="mt-3 space-y-1">
            <div class="h-1.5 w-full overflow-hidden rounded-full bg-surface-100 dark:bg-surface-800">
              <div
                class="h-full rounded-full transition-all duration-500 ease-brand"
                :class="usagePct >= 85 ? 'bg-gradient-to-r from-red-400 to-red-500' : usagePct >= 60 ? 'bg-gradient-to-r from-amber-400 to-amber-500' : 'bg-gradient-to-r from-accent-400 to-accent-500'"
                :style="{ width: usagePct + '%' }"
              />
            </div>
            <div class="text-2xs text-surface-500">{{ $t('portal.dashboard.usagePct', { pct: usagePct, total: formatBytes(totalLimit) }) }}</div>
          </div>
          <div v-else class="mt-3 text-2xs text-surface-400">{{ $t('portal.dashboard.unlimited') }}</div>
        </div>

        <!-- 套餐到期 -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('portal.dashboard.planExpiry') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2" /><path d="M16 2v4M8 2v4M3 10h18" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span v-if="daysToExpiry === null" class="text-2xl font-semibold leading-none tracking-tight text-surface-400 dark:text-surface-500">—</span>
            <template v-else>
              <span class="text-2xl font-semibold leading-none tracking-tight tabular-nums" :class="daysToExpiry <= 3 ? 'text-red-600 dark:text-red-300' : daysToExpiry <= 7 ? 'text-amber-600 dark:text-amber-300' : 'text-ink-900 dark:text-surface-50'">{{ daysToExpiry }}</span>
              <span class="text-sm text-surface-500">{{ $t('portal.dashboard.days') }}</span>
            </template>
          </div>
          <div class="mt-3 text-2xs text-surface-500">
            <template v-if="daysToExpiry === null">{{ $t('portal.dashboard.noOrders') }}</template>
            <template v-else-if="daysToExpiry <= 3">{{ $t('portal.dashboard.expiringSoon') }}</template>
            <template v-else>{{ $t('portal.dashboard.planExpiryDays') }}</template>
          </div>
        </div>

        <!-- 账户余额 -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('portal.dashboard.balance') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M16 8h-6a2 2 0 1 0 0 4h4a2 2 0 1 1 0 4H8M12 6v2M12 16v2" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-2xl font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ profile ? formatYuan(profile.balance_cents) : '—' }}</span>
          </div>
          <div class="mt-3 text-2xs text-surface-500">{{ $t('portal.dashboard.balanceHint') }}</div>
        </div>

        <!-- 客户端数 -->
        <div class="group rounded-2xl border border-surface-100 bg-surface-0 p-5 transition-all duration-200 ease-brand hover:border-surface-200 hover:shadow-card-hover dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-start justify-between">
            <div class="text-xs font-medium text-surface-500">{{ $t('portal.dashboard.activeClients') }}</div>
            <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-accent-50 text-accent-600 transition-transform duration-200 ease-brand group-hover:scale-105 dark:bg-accent-950/40 dark:text-accent-300">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg>
            </div>
          </div>
          <div class="mt-3 flex items-baseline gap-2">
            <span class="text-2xl font-semibold leading-none tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ clients.length }}</span>
          </div>
          <div class="mt-3 text-2xs text-surface-500">{{ $t('portal.dashboard.acrossNodes', { n: new Set(clients.map(c => c.node_id)).size }) }}</div>
        </div>
      </div>

      <!-- Subscription URL preview card -->
      <div v-if="profile" class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0 flex-1">
            <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.dashboard.sub') }}</h2>
            <p class="mt-1 text-xs text-surface-500">{{ $t('portal.dashboard.subHint') }}</p>
            <p class="mt-3 break-all rounded-xl bg-surface-50 px-3 py-2 font-mono text-xs text-surface-600 dark:bg-surface-800 dark:text-surface-300">{{ subURL }}</p>
          </div>
          <RouterLink
            to="/portal/subscription"
            class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
          >
            {{ $t('portal.dashboard.viewQR') }}
            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </RouterLink>
        </div>
      </div>

      <!-- Recent clients table -->
      <div class="overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-4 dark:border-surface-800">
          <div>
            <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.dashboard.tableHeader') }}</h2>
            <p class="mt-0.5 text-xs text-surface-500">{{ $t('portal.dashboard.tableHeaderHint') }}</p>
          </div>
        </header>
        <!-- overflow-x-auto so narrow phones can horizontally scroll the
             wide columns (node / inbound / email / traffic / expires).
             A proper card view per breakpoint is a future-pass polish. -->
        <div v-if="clients.length > 0" class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
            <tr class="border-b border-surface-100 dark:border-surface-800">
              <th class="px-6 py-3 font-medium">{{ $t('portal.dashboard.column.node') }}</th>
              <th class="px-6 py-3 font-medium">{{ $t('portal.dashboard.column.inbound') }}</th>
              <th class="px-6 py-3 text-right font-medium">{{ $t('portal.dashboard.column.upload') }}</th>
              <th class="px-6 py-3 text-right font-medium">{{ $t('portal.dashboard.column.download') }}</th>
              <th class="px-6 py-3 text-right font-medium">{{ $t('portal.dashboard.column.usageLimit') }}</th>
              <th class="px-6 py-3 font-medium">{{ $t('portal.dashboard.column.expires') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
            <tr v-for="c in clients" :key="c.node_id + ':' + c.inbound_tag" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
              <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">#{{ c.node_id }}</td>
              <td class="px-6 py-3.5 font-mono text-xs text-surface-500">{{ c.inbound_tag }}</td>
              <td class="px-6 py-3.5 text-right tabular-nums">{{ formatBytes(c.up) }}</td>
              <td class="px-6 py-3.5 text-right tabular-nums">{{ formatBytes(c.down) }}</td>
              <td class="px-6 py-3.5 text-right tabular-nums">
                <span class="font-medium text-ink-900 dark:text-surface-50">{{ formatBytes(c.total) }}</span>
                <span v-if="c.limit && c.limit > 0" class="ml-1 text-2xs text-surface-400">/ {{ formatBytes(c.limit) }}</span>
              </td>
              <td class="px-6 py-3.5 text-xs text-surface-500">{{ c.expires_at ? new Date(c.expires_at).toLocaleString() : '∞' }}</td>
            </tr>
          </tbody>
        </table>
        </div>
        <div v-else class="px-6 py-14 text-center">
          <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
            <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0" /></svg>
          </div>
          <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">{{ $t('portal.dashboard.empty') }}</h3>
          <p class="mt-1 text-xs text-surface-500">{{ $t('portal.dashboard.emptyDescription') }}</p>
          <RouterLink to="/portal/plans" class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500">
            {{ $t('portal.dashboard.goToPlans') }}
            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </RouterLink>
        </div>
      </div>
    </section>
  </div>
</template>
