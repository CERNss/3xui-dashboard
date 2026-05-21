<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { adminUsersApi, type AdminUser } from '@/api/admin/users'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const users = ref<AdminUser[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const query = ref('')

// Balance-adjust modal
const balanceModal = ref<{
  open: boolean
  user: AdminUser | null
  delta: number
  reason: string
  busy: boolean
  err: string | null
}>({
  open: false,
  user: null,
  delta: 0,
  reason: '',
  busy: false,
  err: null,
})

async function reload() {
  loading.value = true
  error.value = null
  try {
    const r = await adminUsersApi.list({ limit: 200 })
    users.value = r.users
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

// Auto-renew toggle — inline, no confirmation modal. Flipping
// this is reversible and admin-driven, not destructive.
const autoRenewBusy = ref<number | null>(null)

async function toggleAutoRenew(u: AdminUser) {
  autoRenewBusy.value = u.id
  try {
    const updated = await adminUsersApi.update(u.id, { auto_renew: !u.auto_renew })
    const i = users.value.findIndex(x => x.id === u.id)
    if (i >= 0) users.value.splice(i, 1, updated)
  } catch (e: any) {
    error.value = formatError(e, '切换自动续费失败')
  } finally {
    autoRenewBusy.value = null
  }
}

async function toggleSuspend(u: AdminUser) {
  const verb = u.status === 'suspended' ? '解封' : '封停'
  const ok = await askConfirm({
    title: `${verb}用户`,
    message: `用户「${u.email || '#' + u.id}」将被${verb}。`,
    variant: u.status === 'suspended' ? 'default' : 'danger',
    confirmLabel: verb,
  })
  if (!ok) return
  try {
    if (u.status === 'suspended') {
      await adminUsersApi.unsuspend(u.id)
    } else {
      await adminUsersApi.suspend(u.id)
    }
    await reload()
  } catch (e: any) {
    error.value = formatError(e, `${verb}失败`)
  }
}

async function destroy(u: AdminUser) {
  const ok = await askConfirm({
    title: '删除用户',
    message: `用户「${u.email || '#' + u.id}」及其关联的 client_ownerships 会被级联删除，无法恢复。`,
    variant: 'danger',
    confirmLabel: '删除',
  })
  if (!ok) return
  try {
    await adminUsersApi.remove(u.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, '删除失败')
  }
}

function openBalance(u: AdminUser) {
  balanceModal.value = {
    open: true,
    user: u,
    delta: 0,
    reason: '',
    busy: false,
    err: null,
  }
}

async function submitBalance() {
  const m = balanceModal.value
  if (!m.user) return
  if (m.delta === 0) {
    m.err = '调整金额不能为 0'
    return
  }
  if (!m.reason.trim()) {
    m.err = '请填写理由（balance_log 审计需要）'
    return
  }
  m.busy = true
  m.err = null
  try {
    // delta is bound to a number input as integer cents; Math.round
    // guards against fractional input (paste / arrow-key float math).
    await adminUsersApi.adjustBalance(m.user.id, Math.round(m.delta), m.reason.trim())
    m.open = false
    await reload()
  } catch (e: any) {
    m.err = formatError(e, '调整失败')
  } finally {
    m.busy = false
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter((u) =>
    (u.email || '').toLowerCase().includes(q) ||
    String(u.id).includes(q) ||
    u.sub_id.toLowerCase().includes(q),
  )
})

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">用户管理</h1>
        <p class="mt-1.5 text-sm text-surface-500">查看 · 封停 · 调余额 · 删除</p>
      </div>
      <button
        class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800"
        title="刷新"
        @click="reload"
      >
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <!-- Search bar -->
    <div class="mb-4 flex items-center gap-2">
      <div class="relative">
        <svg class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
        <input
          v-model="query"
          type="text"
          placeholder="搜索 邮箱 / ID / sub_id"
          class="h-9 w-80 rounded-xl border border-surface-200 bg-surface-0 pl-9 pr-3 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
        />
      </div>
      <span class="text-xs text-surface-500">{{ filtered.length }} / {{ users.length }}</span>
    </div>

    <Skeleton v-if="loading" :rows="6" />

    <div
      v-else-if="filtered.length > 0"
      class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
    >
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">ID</th>
            <th class="px-6 py-3 font-medium">邮箱</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 text-right font-medium">余额</th>
            <th class="px-6 py-3 font-medium">自动续费</th>
            <th class="px-6 py-3 font-medium">注册</th>
            <th class="px-6 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="u in filtered" :key="u.id" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40" :class="u.status === 'suspended' ? 'opacity-60' : ''">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-400 tabular-nums">#{{ u.id }}</td>
            <td class="px-6 py-3.5">
              <div class="font-medium text-ink-900 dark:text-surface-50">{{ u.email || '—' }}</div>
              <div class="mt-0.5 flex items-center gap-1.5">
                <span v-if="u.email" class="inline-flex items-center rounded-full px-1.5 py-0.5 text-2xs font-medium ring-1 ring-inset"
                  :class="u.email_verified
                    ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                    : 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800'">
                  {{ u.email_verified ? '已验证' : '未验证' }}
                </span>
                <span class="font-mono text-2xs text-surface-400">{{ u.sub_id.slice(0, 8) }}…</span>
              </div>
            </td>
            <td class="px-6 py-3.5">
              <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                :class="u.status === 'active'
                  ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                  : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'">
                <span class="h-1.5 w-1.5 rounded-full" :class="u.status === 'active' ? 'bg-accent-500' : 'bg-red-500'" />
                {{ u.status === 'active' ? '正常' : '已停用' }}
              </span>
            </td>
            <td class="px-6 py-3.5 text-right tabular-nums font-medium text-ink-900 dark:text-surface-50">{{ formatYuan(u.balance_cents) }}</td>
            <td class="px-6 py-3.5">
              <button
                type="button"
                class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset transition-colors"
                :class="u.auto_renew
                  ? 'bg-accent-50 text-accent-700 ring-accent-100 hover:bg-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                  : 'bg-surface-100 text-surface-500 ring-surface-200 hover:bg-surface-200 dark:bg-surface-800 dark:text-surface-400 dark:ring-surface-700'"
                :disabled="autoRenewBusy === u.id"
                @click="toggleAutoRenew(u)"
                :title="u.auto_renew ? '关闭自动续费' : '开启自动续费'"
              >
                <span class="h-1.5 w-1.5 rounded-full" :class="u.auto_renew ? 'bg-accent-500' : 'bg-surface-400'" />
                {{ autoRenewBusy === u.id ? '更新中…' : (u.auto_renew ? '开' : '关') }}
              </button>
            </td>
            <td class="px-6 py-3.5 text-xs text-surface-500">{{ new Date(u.created_at).toLocaleDateString() }}</td>
            <td class="px-6 py-3.5">
              <div class="flex items-center justify-end gap-0.5">
                <button title="调余额" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-accent-50 hover:text-accent-700 dark:hover:bg-accent-950/40 dark:hover:text-accent-300" @click="openBalance(u)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M16 8h-6a2 2 0 1 0 0 4h4a2 2 0 1 1 0 4H8M12 6v2M12 16v2" /></svg>
                </button>
                <button :title="u.status === 'suspended' ? '解封' : '封停'" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="toggleSuspend(u)">
                  <svg v-if="u.status === 'suspended'" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3" /></svg>
                  <svg v-else class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="4" width="4" height="16" /><rect x="14" y="4" width="4" height="16" /></svg>
                </button>
                <button title="删除" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="destroy(u)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else
      icon="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0"
      title="没有用户"
      description="还没有用户注册，或者搜索条件没匹配到任何账户。"
    />

    <!-- Balance adjust modal -->
    <div
      v-if="balanceModal.open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="balanceModal.open = false"
    >
      <div class="w-full max-w-md animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-800">
          <div>
            <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">调整余额</h2>
            <p class="mt-0.5 text-xs text-surface-500">当前 {{ balanceModal.user ? formatYuan(balanceModal.user.balance_cents) : '' }}</p>
          </div>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="balanceModal.open = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-4 px-6 py-5" @submit.prevent="submitBalance">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">变动金额（分）</label>
            <input
              v-model.number="balanceModal.delta"
              type="number"
              required
              placeholder="正数 = 充值，负数 = 扣款"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
            />
            <p class="mt-1 text-2xs text-surface-400">{{ balanceModal.delta > 0 ? '+' : '' }}{{ formatYuan(balanceModal.delta) }}（{{ balanceModal.delta > 0 ? '充值' : balanceModal.delta < 0 ? '扣款' : '—' }}）</p>
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">理由</label>
            <input
              v-model="balanceModal.reason"
              type="text"
              required
              placeholder="例如：退款 / 手动充值 / 邀请奖励"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
            />
            <p class="mt-1 text-2xs text-surface-400">会写进 balance_logs 审计表</p>
          </div>
          <p v-if="balanceModal.err" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ balanceModal.err }}</p>
          <footer class="flex justify-end gap-2 border-t border-surface-100 -mx-6 -mb-5 px-6 py-4 dark:border-surface-800">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="balanceModal.open = false">取消</button>
            <button type="submit" :disabled="balanceModal.busy" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">{{ balanceModal.busy ? '提交中…' : '确认' }}</button>
          </footer>
        </form>
      </div>
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
  </div>
</template>
