<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminUsersApi, type AdminUser, type UserStatus } from '@/api/admin/users'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { t } = useI18n()

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const users = ref<AdminUser[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const query = ref('')
const statusFilter = ref<'' | UserStatus>('') // '' = all
// '' = all; 'verified' = email_verified true; 'unverified' = email_verified false.
const verifiedFilter = ref<'' | 'verified' | 'unverified'>('')
// '' = all; 'email' = traditional account (oidc_subject null/empty);
// 'oidc' = OIDC-only (oidc_subject present and non-empty).
const oidcFilter = ref<'' | 'email' | 'oidc'>('')
// Sort key + direction packed into one ref so dropdown + clickable
// headers stay trivially in sync. Format: '<col>:<asc|desc>'.
// Default — newest registrations first, matches sub2api default ordering.
type SortKey = 'created_at:desc' | 'created_at:asc' | 'balance:desc' | 'balance:asc' | 'id:desc' | 'email:asc' | 'email:desc'
const sort = ref<SortKey>('created_at:desc')
const page = ref(1)
const pageSize = 20

// Batch-select: Set of user ids that are currently checked. Using a Set
// (not an array) so add/has/delete are all O(1) on row-toggle.
const selected = ref<Set<number>>(new Set())

// Auto-refresh: when on, poll /users every AUTO_REFRESH_MS. Interval id
// lives outside the ref so we can clear it on unmount even if Vue's
// reactivity has already torn down the component instance.
const autoRefresh = ref(false)
const AUTO_REFRESH_MS = 15_000
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

// 更多操作 dropdown — anchored next to the auto-refresh toggle. Closes
// on outside click and on item click.
const moreMenuOpen = ref(false)

// Inline toast for success flashes (create / actions). Auto-dismisses
// after 2.4s — quick visual ack, no big nag bar lingering.
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
let flashTimer: ReturnType<typeof setTimeout> | null = null
function showFlash(kind: 'ok' | 'err', text: string) {
  if (flashTimer) clearTimeout(flashTimer)
  flash.value = { kind, text }
  flashTimer = setTimeout(() => {
    flash.value = null
  }, 2400)
}

// Balance-adjust modal (kept from previous Users.vue — AdjustBalance
// affordance is the most-used row action).
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

// Create-user modal — Task 1 addition. initialBalanceYuan is the
// human-facing input (元); we convert to cents on submit so the
// backend gets the integer it wants.
const createModal = ref<{
  open: boolean
  email: string
  password: string
  showPassword: boolean
  initialBalanceYuan: string
  busy: boolean
  err: string | null
}>({
  open: false,
  email: '',
  password: '',
  showPassword: false,
  initialBalanceYuan: '',
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
    error.value = formatError(e, t('admin.users.loadFailed'))
  } finally {
    loading.value = false
  }
}

const autoRenewBusy = ref<number | null>(null)

async function toggleAutoRenew(u: AdminUser) {
  autoRenewBusy.value = u.id
  try {
    const updated = await adminUsersApi.update(u.id, { auto_renew: !u.auto_renew })
    const i = users.value.findIndex(x => x.id === u.id)
    if (i >= 0) users.value.splice(i, 1, updated)
  } catch (e: any) {
    error.value = formatError(e, t('admin.users.autoRenewToggleFailed'))
  } finally {
    autoRenewBusy.value = null
  }
}

async function toggleSuspend(u: AdminUser) {
  const verb = u.status === 'suspended' ? t('admin.users.unsuspend') : t('admin.users.suspend')
  const ok = await askConfirm({
    title: t('admin.users.suspendTitle', { verb }),
    message: t('admin.users.suspendMsg', { email: u.email || '#' + u.id, verb }),
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
    error.value = formatError(e, t('admin.users.suspendFailed', { verb }))
  }
}

async function destroy(u: AdminUser) {
  const ok = await askConfirm({
    title: t('admin.users.confirmDelete'),
    message: t('admin.users.confirmDeleteMsg', { email: u.email || '#' + u.id }),
    variant: 'danger',
    confirmLabel: t('admin.users.delete'),
  })
  if (!ok) return
  try {
    await adminUsersApi.remove(u.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, t('admin.users.deleteFailed'))
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
    m.err = t('admin.users.balance.deltaMustNonZero')
    return
  }
  if (!m.reason.trim()) {
    m.err = t('admin.users.balance.reasonRequired')
    return
  }
  m.busy = true
  m.err = null
  try {
    await adminUsersApi.adjustBalance(m.user.id, Math.round(m.delta), m.reason.trim())
    m.open = false
    await reload()
  } catch (e: any) {
    m.err = formatError(e, t('admin.users.balance.failed'))
  } finally {
    m.busy = false
  }
}

function openCreate() {
  createModal.value = {
    open: true,
    email: '',
    password: '',
    showPassword: false,
    initialBalanceYuan: '',
    busy: false,
    err: null,
  }
}

async function submitCreate() {
  const m = createModal.value
  const email = m.email.trim()
  if (!email) {
    m.err = t('admin.users.create.emailRequired')
    return
  }
  if (!m.password) {
    m.err = t('admin.users.create.passwordRequired')
    return
  }
  if (m.password.length < 8) {
    m.err = t('admin.users.create.passwordMin')
    return
  }
  // Yuan → cents. Float math is safe here because the form is bounded
  // by step=0.01; round to integer cents to dodge 12.34 → 1233.9999.
  const yuan = String(m.initialBalanceYuan ?? '').trim()
  let initialCents: number | undefined
  if (yuan) {
    const parsed = Number(yuan)
    if (!Number.isFinite(parsed) || parsed < 0) {
      m.err = t('admin.users.create.failed')
      return
    }
    initialCents = Math.round(parsed * 100)
  }
  m.busy = true
  m.err = null
  try {
    const created = await adminUsersApi.create({
      email,
      password: m.password,
      initial_balance_cents: initialCents,
    })
    m.open = false
    showFlash('ok', t('admin.users.create.success', { email: created.email || `#${created.id}` }))
    await reload()
  } catch (e: any) {
    // 409 from backend = email exists; surface a friendly message
    // instead of the raw "request failed".
    const status = e?.response?.status
    if (status === 409) {
      m.err = t('admin.users.create.emailExists')
    } else {
      m.err = formatError(e, t('admin.users.create.failed'))
    }
  } finally {
    m.busy = false
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

// Initial = first character of email (uppercase). Falls back to "?"
// for OIDC-only accounts with no email yet, which is rare but real.
function avatarInitial(u: AdminUser): string {
  if (u.email && u.email.length > 0) return u.email[0].toUpperCase()
  return '?'
}

// Stable color per user id from a small palette, so the avatar circles
// don't all look the same. Uses id-based hash → 5 colors to match
// sub2api's "warm but varied" feel.
const avatarPalette = [
  'bg-accent-500/15 text-accent-600 dark:text-accent-300',
  'bg-primary-500/15 text-primary-600 dark:text-primary-300',
  'bg-amber-500/15 text-amber-700 dark:text-amber-300',
  'bg-pink-500/15 text-pink-600 dark:text-pink-300',
  'bg-violet-500/15 text-violet-600 dark:text-violet-300',
]
function avatarClass(u: AdminUser): string {
  return avatarPalette[u.id % avatarPalette.length]
}

// Relative-time formatter. The backend always returns ISO strings
// in UTC; we render the locale's "X days ago" form and stash the
// absolute timestamp in title= so hover gives the precise wall-clock.
function relativeTime(iso: string): string {
  const now = Date.now()
  const then = new Date(iso).getTime()
  if (!Number.isFinite(then)) return '—'
  const sec = Math.max(0, Math.floor((now - then) / 1000))
  if (sec < 60) return t('admin.users.relTime.justNow')
  const min = Math.floor(sec / 60)
  if (min < 60) return t('admin.users.relTime.minutes', { n: min })
  const hr = Math.floor(min / 60)
  if (hr < 24) return t('admin.users.relTime.hours', { n: hr })
  const day = Math.floor(hr / 24)
  if (day < 30) return t('admin.users.relTime.days', { n: day })
  const mo = Math.floor(day / 30)
  if (mo < 12) return t('admin.users.relTime.months', { n: mo })
  const yr = Math.floor(mo / 12)
  return t('admin.users.relTime.years', { n: yr })
}

function absoluteTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isFinite(d.getTime()) ? d.toLocaleString() : ''
}

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  const out = users.value.filter((u) => {
    if (statusFilter.value && u.status !== statusFilter.value) return false
    if (verifiedFilter.value === 'verified' && !u.email_verified) return false
    if (verifiedFilter.value === 'unverified' && u.email_verified) return false
    if (oidcFilter.value === 'oidc' && !(u.oidc_subject && u.oidc_subject.length > 0)) return false
    if (oidcFilter.value === 'email' && u.oidc_subject && u.oidc_subject.length > 0) return false
    if (!q) return true
    return (
      (u.email || '').toLowerCase().includes(q) ||
      String(u.id).includes(q) ||
      u.sub_id.toLowerCase().includes(q)
    )
  })
  return sortUsers(out, sort.value)
})

// Pure sort fn — produces a new array so the computed memoizes
// correctly. Comparators are tuned to keep stable ordering by id
// when the primary key ties (e.g. two users with balance=0).
function sortUsers(list: AdminUser[], key: SortKey): AdminUser[] {
  const arr = list.slice()
  switch (key) {
    case 'created_at:asc':
      arr.sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id - b.id)
      break
    case 'created_at:desc':
      arr.sort((a, b) => b.created_at.localeCompare(a.created_at) || b.id - a.id)
      break
    case 'balance:asc':
      arr.sort((a, b) => a.balance_cents - b.balance_cents || a.id - b.id)
      break
    case 'balance:desc':
      arr.sort((a, b) => b.balance_cents - a.balance_cents || b.id - a.id)
      break
    case 'id:desc':
      arr.sort((a, b) => b.id - a.id)
      break
    case 'email:asc':
      arr.sort((a, b) => (a.email || '').localeCompare(b.email || '') || a.id - b.id)
      break
    case 'email:desc':
      arr.sort((a, b) => (b.email || '').localeCompare(a.email || '') || b.id - a.id)
      break
  }
  return arr
}

// Header-click handler: toggles between asc/desc on the same column,
// otherwise switches to desc on the freshly clicked column. Keeps the
// dropdown ref in sync because they both read `sort`.
function sortBy(col: 'email' | 'balance' | 'created_at') {
  const [curCol, curDir] = sort.value.split(':')
  if (curCol === col) {
    sort.value = (curDir === 'asc' ? `${col}:desc` : `${col}:asc`) as SortKey
  } else {
    sort.value = `${col}:desc` as SortKey
  }
}

function sortIndicator(col: 'email' | 'balance' | 'created_at'): '' | '↑' | '↓' {
  const [curCol, curDir] = sort.value.split(':')
  if (curCol !== col) return ''
  return curDir === 'asc' ? '↑' : '↓'
}

const total = computed(() => filtered.value.length)
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

// Reset to page 1 whenever any filter changes — otherwise a query
// that yields 3 rows but page is stuck at 4 shows an empty table.
watch([query, statusFilter, verifiedFilter, oidcFilter, sort], () => {
  page.value = 1
})

const paged = computed(() => {
  const from = (page.value - 1) * pageSize
  return filtered.value.slice(from, from + pageSize)
})

const pageFrom = computed(() => (total.value === 0 ? 0 : (page.value - 1) * pageSize + 1))
const pageTo = computed(() => Math.min(page.value * pageSize, total.value))

// Show empty state only when there are zero users AND no active
// filter/search — otherwise "no matches for filter" reads weird.
const showEmptyState = computed(() =>
  !loading.value &&
  users.value.length === 0 &&
  !query.value.trim() &&
  !statusFilter.value &&
  !verifiedFilter.value &&
  !oidcFilter.value,
)

// Batch-select helpers. Header-checkbox state is tri-valued so the
// browser draws an indeterminate dash when *some* (but not all)
// visible rows are selected — that's the standard table affordance.
const selectedCount = computed(() => selected.value.size)
const allVisibleSelected = computed(() => {
  if (paged.value.length === 0) return false
  return paged.value.every((u) => selected.value.has(u.id))
})
const someVisibleSelected = computed(() => {
  return paged.value.some((u) => selected.value.has(u.id)) && !allVisibleSelected.value
})
function toggleAllVisible(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  if (checked) {
    paged.value.forEach((u) => selected.value.add(u.id))
  } else {
    paged.value.forEach((u) => selected.value.delete(u.id))
  }
  // Trigger reactivity — Set mutations aren't tracked deeply.
  selected.value = new Set(selected.value)
}
function toggleOne(id: number, e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  if (checked) selected.value.add(id)
  else selected.value.delete(id)
  selected.value = new Set(selected.value)
}
function clearSelection() {
  selected.value = new Set()
}

// Batch runners — Promise.all so 50 suspends fire in parallel, with
// settled-style counting so one 404 doesn't cancel the others. We
// always reload + clear after, regardless of partial failures.
async function batchSuspend() {
  if (selected.value.size === 0) return
  const ids = Array.from(selected.value)
  const results = await Promise.allSettled(ids.map((id) => adminUsersApi.suspend(id)))
  const ok = results.filter((r) => r.status === 'fulfilled').length
  const fail = results.length - ok
  showFlash(fail === 0 ? 'ok' : 'err', t('admin.users.batch.suspendResult', { ok, fail }))
  clearSelection()
  await reload()
}
async function batchUnsuspend() {
  if (selected.value.size === 0) return
  const ids = Array.from(selected.value)
  const results = await Promise.allSettled(ids.map((id) => adminUsersApi.unsuspend(id)))
  const ok = results.filter((r) => r.status === 'fulfilled').length
  const fail = results.length - ok
  showFlash(fail === 0 ? 'ok' : 'err', t('admin.users.batch.unsuspendResult', { ok, fail }))
  clearSelection()
  await reload()
}
async function batchDelete() {
  if (selected.value.size === 0) return
  const n = selected.value.size
  const ok = await askConfirm({
    title: t('admin.users.batch.deleteTitle'),
    message: t('admin.users.batch.deleteMsg', { n }),
    variant: 'danger',
    confirmLabel: t('admin.users.delete'),
  })
  if (!ok) return
  const ids = Array.from(selected.value)
  const results = await Promise.allSettled(ids.map((id) => adminUsersApi.remove(id)))
  const okCount = results.filter((r) => r.status === 'fulfilled').length
  const failCount = results.length - okCount
  showFlash(failCount === 0 ? 'ok' : 'err', t('admin.users.batch.deleteResult', { ok: okCount, fail: failCount }))
  clearSelection()
  await reload()
}

// Auto-refresh — interval polls /users on the same cadence as a
// human hammering F5. We don't fall back exponentially because the
// dataset is bounded (limit=200) and the load is trivial.
function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    autoRefreshTimer = setInterval(() => {
      // Quietly skip if a manual reload is already in flight — we
      // don't want overlapping requests during slow networks.
      if (!loading.value) reload()
    }, AUTO_REFRESH_MS)
  } else if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

// Click-outside teardown for the 更多操作 menu. Document listener
// is cheap because we only mount it conditionally on open.
function closeMoreMenu(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.closest('[data-more-menu]')) {
    moreMenuOpen.value = false
  }
}
watch(moreMenuOpen, (open) => {
  if (open) {
    // Defer so the click that *opened* the menu doesn't immediately
    // close it on the bubble phase.
    setTimeout(() => document.addEventListener('click', closeMoreMenu), 0)
  } else {
    document.removeEventListener('click', closeMoreMenu)
  }
})

// CSV export — client-side blob. Currently-filtered list (not just
// the current page) so users can export across pagination. Backend
// has no /users/export endpoint and we're not inventing one.
function exportCsv() {
  const rows = filtered.value
  const headers = ['id', 'email', 'status', 'balance_cents', 'email_verified', 'created_at']
  const esc = (v: unknown): string => {
    const s = v === null || v === undefined ? '' : String(v)
    // Wrap in quotes if the value contains comma / quote / newline,
    // doubling internal quotes — minimal RFC4180 escaping, no library.
    if (/[",\n\r]/.test(s)) return '"' + s.replace(/"/g, '""') + '"'
    return s
  }
  const lines = [headers.join(',')]
  for (const u of rows) {
    lines.push([
      u.id,
      esc(u.email || ''),
      u.status,
      u.balance_cents,
      u.email_verified ? 'true' : 'false',
      u.created_at,
    ].join(','))
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  a.href = url
  a.download = `users-${ts}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  moreMenuOpen.value = false
  showFlash('ok', t('admin.users.more.csvExported', { n: rows.length }))
}

// Placeholder — the backend has no endpoint for purging client
// caches across nodes. Surfacing this here advertises the future
// affordance without lying that it works.
function purgeClientCache() {
  moreMenuOpen.value = false
  showFlash('err', t('admin.users.more.notImplemented'))
}

onMounted(reload)
onUnmounted(() => {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
  document.removeEventListener('click', closeMoreMenu)
})
</script>

<template>
  <div>
    <p
      v-if="error"
      class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300"
    >{{ error }}</p>

    <Transition name="fade">
      <p
        v-if="flash"
        class="mb-4 rounded-xl px-4 py-3 text-sm ring-1 ring-inset"
        :class="flash.kind === 'ok'
          ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-500/10 dark:text-accent-300 dark:ring-accent-500/30'
          : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300'"
      >{{ flash.text }}</p>
    </Transition>

    <!-- Toolbar: two-layer layout. Search gets its own row; filters and
         actions sit on the second row so the control surface stays scannable. -->
    <section class="mb-4 rounded-2xl border border-surface-100 bg-surface-0 p-3 shadow-card dark:border-surface-800 dark:bg-surface-900">
      <div class="flex items-center">
        <div class="relative w-full max-w-2xl">
          <svg class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
          <input
            v-model="query"
            type="text"
            :placeholder="$t('admin.users.searchPlaceholder')"
            class="h-10 w-full rounded-xl border border-surface-200 bg-surface-50/60 pl-9 pr-3 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:bg-surface-0 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-950/30 dark:text-surface-100 dark:focus:bg-surface-950/50"
          />
        </div>
      </div>
      <div class="mt-3 flex flex-col gap-3 border-t border-surface-100 pt-3 dark:border-surface-800 xl:flex-row xl:items-center xl:justify-between">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <div class="relative">
            <select
              v-model="statusFilter"
              :aria-label="$t('admin.users.filterStatus')"
              class="h-9 appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200"
            >
              <option value="">{{ $t('admin.users.filterAll') }}</option>
              <option value="active">{{ $t('admin.users.status.active') }}</option>
              <option value="suspended">{{ $t('admin.users.status.suspended') }}</option>
            </select>
            <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
          </div>
          <div class="relative">
            <select
              v-model="verifiedFilter"
              :aria-label="$t('admin.users.filterVerified')"
              class="h-9 appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200"
            >
              <option value="">{{ $t('admin.users.filterVerifiedAll') }}</option>
              <option value="verified">{{ $t('admin.users.verified') }}</option>
              <option value="unverified">{{ $t('admin.users.unverified') }}</option>
            </select>
            <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
          </div>
          <div class="relative">
            <select
              v-model="oidcFilter"
              :aria-label="$t('admin.users.filterRegisterMethod')"
              class="h-9 appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200"
            >
              <option value="">{{ $t('admin.users.filterRegisterAll') }}</option>
              <option value="email">{{ $t('admin.users.filterRegisterEmail') }}</option>
              <option value="oidc">{{ $t('admin.users.filterRegisterOidc') }}</option>
            </select>
            <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
          </div>
          <div class="relative">
            <select
              v-model="sort"
              :aria-label="$t('admin.users.sortLabel')"
              class="h-9 appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200"
            >
              <option value="created_at:desc">{{ $t('admin.users.sort.createdDesc') }}</option>
              <option value="created_at:asc">{{ $t('admin.users.sort.createdAsc') }}</option>
              <option value="balance:desc">{{ $t('admin.users.sort.balanceDesc') }}</option>
              <option value="balance:asc">{{ $t('admin.users.sort.balanceAsc') }}</option>
              <option value="id:desc">{{ $t('admin.users.sort.idDesc') }}</option>
            </select>
            <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
          </div>
        </div>
        <div class="flex shrink-0 flex-wrap items-center justify-start gap-2 xl:justify-end">
          <button
            class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700 dark:hover:text-surface-50"
            :title="$t('admin.users.reload')"
            @click="reload"
          >
            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
          </button>
          <!-- Auto-refresh toggle. Pill flips to accent when on; the
               pulsing dot is purely cosmetic to signal "live data". -->
          <button
            type="button"
            :title="$t('admin.users.autoRefresh')"
            class="inline-flex h-9 items-center gap-1.5 rounded-xl border px-3 text-sm font-medium transition-all ease-brand active:scale-[0.98]"
            :class="autoRefresh
              ? 'border-accent-500/40 bg-accent-50 text-accent-700 dark:border-accent-500/40 dark:bg-accent-500/15 dark:text-accent-300'
              : 'border-surface-200 bg-surface-0 text-surface-600 hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700 dark:hover:text-surface-50'"
            @click="toggleAutoRefresh"
          >
            <span class="relative flex h-2 w-2">
              <span
                v-if="autoRefresh"
                class="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent-400 opacity-75"
              />
              <span
                class="relative inline-flex h-2 w-2 rounded-full"
                :class="autoRefresh ? 'bg-accent-500' : 'bg-surface-400 dark:bg-surface-500'"
              />
            </span>
            {{ $t('admin.users.autoRefresh') }}
          </button>
          <!-- 更多操作 menu — anchored. data-more-menu lets click-outside
               ignore clicks inside the wrapper. -->
          <div class="relative" data-more-menu>
            <button
              type="button"
              class="inline-flex h-9 items-center gap-1 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm font-medium text-surface-700 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700 dark:hover:text-surface-50"
              :aria-expanded="moreMenuOpen"
              aria-haspopup="menu"
              @click="moreMenuOpen = !moreMenuOpen"
            >
              {{ $t('admin.users.more.label') }}
              <svg class="h-3 w-3 transition-transform" :class="moreMenuOpen ? 'rotate-180' : ''" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
            </button>
            <Transition name="fade">
              <div
                v-if="moreMenuOpen"
                role="menu"
                class="absolute right-0 z-30 mt-1 w-64 origin-top-right overflow-hidden rounded-xl border border-surface-200 bg-surface-0 shadow-elevated dark:border-surface-700 dark:bg-surface-900"
              >
                <button
                  type="button"
                  role="menuitem"
                  class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-surface-700 transition-colors hover:bg-surface-50 hover:text-ink-900 dark:text-surface-200 dark:hover:bg-surface-800 dark:hover:text-surface-50"
                  @click="exportCsv"
                >
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M7 10l5 5 5-5M5 21h14" /></svg>
                  {{ $t('admin.users.more.exportCsv') }}
                </button>
                <button
                  type="button"
                  role="menuitem"
                  class="flex w-full items-center gap-2 border-t border-surface-100 px-3 py-2 text-left text-sm text-surface-700 transition-colors hover:bg-surface-50 hover:text-ink-900 dark:border-surface-800 dark:text-surface-200 dark:hover:bg-surface-800 dark:hover:text-surface-50"
                  @click="purgeClientCache"
                >
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /></svg>
                  {{ $t('admin.users.more.purgeCache') }}
                </button>
              </div>
            </Transition>
          </div>
          <button
            type="button"
            class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-accent-600 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-accent-500 active:scale-[0.98] dark:bg-accent-500 dark:hover:bg-accent-400"
            @click="openCreate"
          >
            <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14" /></svg>
            {{ $t('admin.users.addUser') }}
          </button>
        </div>
      </div>
    </section>

    <!-- Batch action bar — slides in just below the toolbar whenever
         the selection set is non-empty. Sits above the table so it
         doesn't shift the table layout. -->
    <Transition name="slide-fade">
      <div
        v-if="selectedCount > 0"
        class="mb-4 flex flex-col items-stretch gap-2 rounded-xl border border-accent-500/30 bg-accent-50/60 px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between dark:border-accent-500/30 dark:bg-accent-500/10"
      >
        <div class="font-medium text-accent-700 dark:text-accent-300">
          {{ $t('admin.users.batch.selectedCount', { n: selectedCount }) }}
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-surface-200 bg-surface-0 px-3 text-xs font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700"
            @click="batchSuspend"
          >
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="4" width="4" height="16" /><rect x="14" y="4" width="4" height="16" /></svg>
            {{ $t('admin.users.batch.suspend') }}
          </button>
          <button
            type="button"
            class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-surface-200 bg-surface-0 px-3 text-xs font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700"
            @click="batchUnsuspend"
          >
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3" /></svg>
            {{ $t('admin.users.batch.unsuspend') }}
          </button>
          <button
            type="button"
            class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-red-200 bg-red-50 px-3 text-xs font-medium text-red-700 transition-all hover:bg-red-100 active:scale-[0.98] dark:border-red-500/30 dark:bg-red-500/15 dark:text-red-300 dark:hover:bg-red-500/25"
            @click="batchDelete"
          >
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
            {{ $t('admin.users.batch.delete') }}
          </button>
          <button
            type="button"
            class="inline-flex h-8 items-center rounded-lg px-2 text-xs font-medium text-surface-500 transition-colors hover:text-ink-900 dark:text-surface-400 dark:hover:text-surface-50"
            @click="clearSelection"
          >
            {{ $t('admin.users.batch.clear') }}
          </button>
        </div>
      </div>
    </Transition>

    <Skeleton v-if="loading" :rows="6" />

    <template v-else>
      <template v-if="paged.length > 0">
        <div class="grid gap-3 2xl:hidden">
          <article
            v-for="u in paged"
            :key="u.id"
            class="rounded-2xl border border-surface-100 bg-surface-0 p-4 shadow-card transition-colors dark:border-surface-800 dark:bg-surface-900"
            :class="[
              u.status === 'suspended' ? 'opacity-70' : '',
              selected.has(u.id) ? 'border-accent-500/30 bg-accent-50/40 dark:border-accent-500/40 dark:bg-accent-500/10' : '',
            ]"
          >
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div class="flex min-w-0 items-start gap-3">
                <input
                  type="checkbox"
                  class="mt-2 h-4 w-4 shrink-0 cursor-pointer rounded border-surface-300 text-accent-600 focus:ring-accent-500 focus:ring-offset-0 dark:border-surface-600 dark:bg-surface-800"
                  :checked="selected.has(u.id)"
                  :aria-label="$t('admin.users.batch.toggleRow', { email: u.email || '#' + u.id })"
                  @change="(e) => toggleOne(u.id, e)"
                />
                <div :class="['flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-semibold', avatarClass(u)]">{{ avatarInitial(u) }}</div>
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="break-all font-medium leading-6 text-ink-900 dark:text-surface-50">{{ u.email || '—' }}</span>
                    <span
                      v-if="u.email"
                      class="inline-flex shrink-0 whitespace-nowrap items-center rounded-full px-1.5 py-0.5 text-2xs font-medium ring-1 ring-inset"
                      :class="u.email_verified
                        ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-500/15 dark:text-accent-300 dark:ring-accent-500/30'
                        : 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-500/15 dark:text-amber-300 dark:ring-amber-500/30'"
                    >
                      {{ u.email_verified ? $t('admin.users.verified') : $t('admin.users.unverified') }}
                    </span>
                  </div>
                  <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 font-mono text-2xs text-surface-400 dark:text-surface-500">
                    <span>#{{ u.id }}</span>
                    <span class="max-w-full truncate">{{ u.sub_id.slice(0, 12) }}…</span>
                  </div>
                </div>
              </div>
              <span
                class="inline-flex w-fit shrink-0 whitespace-nowrap items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset sm:mt-1"
                :class="u.status === 'active'
                  ? 'bg-accent-50 text-accent-700 ring-accent-200 dark:bg-accent-500/15 dark:text-accent-300 dark:ring-accent-500/40'
                  : 'bg-red-50 text-red-600 ring-red-200 dark:bg-red-500/15 dark:text-red-300 dark:ring-red-500/40'"
              >
                <span class="h-1.5 w-1.5 rounded-full" :class="u.status === 'active' ? 'bg-accent-500' : 'bg-red-500'" />
                {{ u.status === 'active' ? t('admin.users.status.active') : t('admin.users.status.suspended') }}
              </span>
            </div>

            <dl class="mt-4 grid grid-cols-2 gap-2 text-sm lg:grid-cols-4">
              <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
                <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.users.column.balance') }}</dt>
                <dd class="mt-1 flex flex-wrap items-center gap-2">
                  <span class="font-medium tabular-nums text-ink-900 dark:text-surface-50">{{ formatYuan(u.balance_cents) }}</span>
                  <button
                    :title="$t('admin.users.balance.adjust')"
                    class="inline-flex h-6 items-center rounded-lg border border-surface-200 bg-surface-0 px-2 text-2xs font-medium text-surface-600 transition-colors hover:border-accent-200 hover:bg-accent-50 hover:text-accent-700 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-300 dark:hover:border-accent-500/30 dark:hover:bg-accent-500/20 dark:hover:text-accent-200"
                    @click="openBalance(u)"
                  >{{ $t('admin.users.balance.adjustShort') }}</button>
                </dd>
              </div>
              <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
                <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.users.column.autoRenew') }}</dt>
                <dd class="mt-1">
                  <button
                    type="button"
                    role="switch"
                    :aria-checked="u.auto_renew"
                    :title="u.auto_renew ? $t('admin.users.autoRenewOff') : $t('admin.users.autoRenewOn')"
                    :disabled="autoRenewBusy === u.id"
                    class="inline-flex h-7 items-center gap-2 rounded-full border px-2 text-xs font-medium transition-colors disabled:opacity-50"
                    :class="u.auto_renew
                      ? 'border-accent-500/30 bg-accent-50 text-accent-700 dark:border-accent-500/30 dark:bg-accent-500/15 dark:text-accent-300'
                      : 'border-surface-200 bg-surface-0 text-surface-500 hover:bg-surface-50 hover:text-surface-700 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-200'"
                    @click="toggleAutoRenew(u)"
                  >
                    <span
                      class="relative h-4 w-7 rounded-full transition-colors"
                      :class="u.auto_renew ? 'bg-accent-500' : 'bg-surface-300 dark:bg-surface-700'"
                    >
                      <span
                        class="absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-white shadow-sm transition-transform"
                        :class="u.auto_renew ? 'translate-x-3' : 'translate-x-0'"
                      />
                    </span>
                    <span>{{ autoRenewBusy === u.id ? $t('admin.users.autoRenewUpdating') : (u.auto_renew ? $t('admin.users.on') : $t('admin.users.off')) }}</span>
                  </button>
                </dd>
              </div>
              <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
                <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.users.column.registered') }}</dt>
                <dd class="mt-1 truncate text-xs text-surface-700 dark:text-surface-200" :title="absoluteTime(u.created_at)">{{ relativeTime(u.created_at) }}</dd>
              </div>
              <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
                <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.users.column.lastActive') }}</dt>
                <dd class="mt-1 truncate text-xs text-surface-700 dark:text-surface-200">
                  <span v-if="u.last_active_at" :title="absoluteTime(u.last_active_at)">{{ relativeTime(u.last_active_at) }}</span>
                  <span v-else class="text-surface-400 dark:text-surface-600">—</span>
                </dd>
              </div>
            </dl>

            <div class="mt-4 flex flex-wrap items-center justify-end gap-1.5 border-t border-surface-100 pt-3 dark:border-surface-800">
              <button
                :title="u.status === 'suspended' ? $t('admin.users.unsuspend') : $t('admin.users.suspend')"
                class="inline-flex h-8 items-center rounded-lg border border-surface-200 bg-surface-0 px-2.5 text-xs font-medium text-surface-600 transition-colors hover:bg-surface-50 hover:text-ink-900 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
                @click="toggleSuspend(u)"
              >
                {{ u.status === 'suspended' ? $t('admin.users.unsuspend') : $t('admin.users.suspend') }}
              </button>
              <button
                :title="$t('admin.users.delete')"
                class="inline-flex h-8 items-center rounded-lg border border-transparent px-2.5 text-xs font-medium text-red-600 transition-colors hover:border-red-200 hover:bg-red-50 dark:text-red-400 dark:hover:border-red-500/30 dark:hover:bg-red-500/15"
                @click="destroy(u)"
              >
                {{ $t('admin.users.delete') }}
              </button>
            </div>
          </article>
        </div>

        <div
          class="hidden overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-700 dark:bg-surface-900 2xl:block"
        >
        <table class="w-full min-w-[1120px] text-sm">
          <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-400">
            <tr class="border-b border-surface-100 dark:border-surface-700">
              <th class="w-10 px-4 py-3">
                <!-- Header checkbox uses :indeterminate via a function
                     ref because v-bind on :indeterminate isn't a real
                     HTML attribute — only a DOM property. -->
                <input
                  type="checkbox"
                  class="h-4 w-4 cursor-pointer rounded border-surface-300 text-accent-600 focus:ring-accent-500 focus:ring-offset-0 dark:border-surface-600 dark:bg-surface-800"
                  :aria-label="$t('admin.users.batch.toggleAll')"
                  :checked="allVisibleSelected"
                  :ref="(el) => { if (el) (el as HTMLInputElement).indeterminate = someVisibleSelected }"
                  @change="toggleAllVisible"
                />
              </th>
              <th
                class="cursor-pointer select-none px-6 py-3 font-medium transition-colors hover:text-ink-900 dark:hover:text-surface-50"
                @click="sortBy('email')"
              >
                <span class="inline-flex items-center gap-1">
                  {{ $t('admin.users.column.user') }}
                  <span v-if="sortIndicator('email')" class="text-accent-500">{{ sortIndicator('email') }}</span>
                </span>
              </th>
              <th class="px-4 py-3 font-medium">{{ $t('admin.users.column.id') }}</th>
              <th class="px-4 py-3 font-medium">{{ $t('admin.users.column.status') }}</th>
              <th
                class="cursor-pointer select-none px-4 py-3 text-right font-medium transition-colors hover:text-ink-900 dark:hover:text-surface-50"
                @click="sortBy('balance')"
              >
                <span class="inline-flex items-center justify-end gap-1">
                  {{ $t('admin.users.column.balance') }}
                  <span v-if="sortIndicator('balance')" class="text-accent-500">{{ sortIndicator('balance') }}</span>
                </span>
              </th>
              <th class="px-4 py-3 font-medium">{{ $t('admin.users.column.autoRenew') }}</th>
              <th
                class="cursor-pointer select-none px-4 py-3 font-medium transition-colors hover:text-ink-900 dark:hover:text-surface-50"
                @click="sortBy('created_at')"
              >
                <span class="inline-flex items-center gap-1">
                  {{ $t('admin.users.column.registered') }}
                  <span v-if="sortIndicator('created_at')" class="text-accent-500">{{ sortIndicator('created_at') }}</span>
                </span>
              </th>
              <th class="px-4 py-3 font-medium">{{ $t('admin.users.column.lastActive') }}</th>
              <th class="sticky right-0 z-10 min-w-[148px] border-l border-surface-100 bg-surface-0 px-4 py-3 text-right font-medium dark:border-surface-700 dark:bg-surface-900">{{ $t('admin.users.column.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-100 dark:divide-surface-700">
            <tr
              v-for="u in paged"
              :key="u.id"
              class="group/row transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/60"
              :class="[
                u.status === 'suspended' ? 'opacity-70' : '',
                selected.has(u.id) ? 'bg-accent-50/40 dark:bg-accent-500/5' : '',
              ]"
            >
              <td class="w-10 px-4 py-3.5">
                <input
                  type="checkbox"
                  class="h-4 w-4 cursor-pointer rounded border-surface-300 text-accent-600 focus:ring-accent-500 focus:ring-offset-0 dark:border-surface-600 dark:bg-surface-800"
                  :checked="selected.has(u.id)"
                  :aria-label="$t('admin.users.batch.toggleRow', { email: u.email || '#' + u.id })"
                  @change="(e) => toggleOne(u.id, e)"
                />
              </td>
              <!-- User: avatar + email + sub_id slice + verified chip -->
              <td class="px-6 py-3.5">
                <div class="flex items-center gap-3">
                  <div :class="['flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold', avatarClass(u)]">{{ avatarInitial(u) }}</div>
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="truncate font-medium text-ink-900 dark:text-surface-50">{{ u.email || '—' }}</span>
                      <span v-if="u.email" class="inline-flex shrink-0 whitespace-nowrap items-center rounded-full px-1.5 py-0.5 text-2xs font-medium ring-1 ring-inset"
                        :class="u.email_verified
                          ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-500/15 dark:text-accent-300 dark:ring-accent-500/30'
                          : 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-500/15 dark:text-amber-300 dark:ring-amber-500/30'">
                        {{ u.email_verified ? $t('admin.users.verified') : $t('admin.users.unverified') }}
                      </span>
                    </div>
                    <div class="mt-0.5 truncate font-mono text-2xs text-surface-400 dark:text-surface-500">{{ u.sub_id.slice(0, 12) }}…</div>
                  </div>
                </div>
              </td>
              <td class="px-4 py-3.5 font-mono text-xs text-surface-400 tabular-nums dark:text-surface-400">#{{ u.id }}</td>
              <td class="px-4 py-3.5">
                <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                  :class="u.status === 'active'
                    ? 'bg-accent-50 text-accent-700 ring-accent-200 dark:bg-accent-500/15 dark:text-accent-300 dark:ring-accent-500/40'
                    : 'bg-red-50 text-red-600 ring-red-200 dark:bg-red-500/15 dark:text-red-300 dark:ring-red-500/40'">
                  <span class="h-1.5 w-1.5 rounded-full" :class="u.status === 'active' ? 'bg-accent-500' : 'bg-red-500'" />
                  {{ u.status === 'active' ? t('admin.users.status.active') : t('admin.users.status.suspended') }}
                </span>
              </td>
              <td class="px-4 py-3.5">
                <div class="flex items-center justify-end gap-2">
                  <span class="text-right font-medium tabular-nums text-ink-900 dark:text-surface-50">{{ formatYuan(u.balance_cents) }}</span>
                  <button
                    :title="$t('admin.users.balance.adjust')"
                    class="inline-flex h-7 items-center rounded-lg border border-surface-200 bg-surface-0 px-2 text-xs font-medium text-surface-600 transition-colors hover:border-accent-200 hover:bg-accent-50 hover:text-accent-700 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-300 dark:hover:border-accent-500/30 dark:hover:bg-accent-500/20 dark:hover:text-accent-200"
                    @click="openBalance(u)"
                  >{{ $t('admin.users.balance.adjustShort') }}</button>
                </div>
              </td>
              <td class="px-4 py-3.5">
                <button
                  type="button"
                  role="switch"
                  :aria-checked="u.auto_renew"
                  :title="u.auto_renew ? $t('admin.users.autoRenewOff') : $t('admin.users.autoRenewOn')"
                  :disabled="autoRenewBusy === u.id"
                  class="inline-flex h-7 items-center gap-2 rounded-full border px-2 text-xs font-medium transition-colors disabled:opacity-50"
                  :class="u.auto_renew
                    ? 'border-accent-500/30 bg-accent-50 text-accent-700 dark:border-accent-500/30 dark:bg-accent-500/15 dark:text-accent-300'
                    : 'border-surface-200 bg-surface-0 text-surface-500 hover:bg-surface-50 hover:text-surface-700 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-200'"
                  @click="toggleAutoRenew(u)"
                >
                  <span
                    class="relative h-4 w-7 rounded-full transition-colors"
                    :class="u.auto_renew ? 'bg-accent-500' : 'bg-surface-300 dark:bg-surface-700'"
                  >
                    <span
                      class="absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-white shadow-sm transition-transform"
                      :class="u.auto_renew ? 'translate-x-3' : 'translate-x-0'"
                    />
                  </span>
                  <span>{{ autoRenewBusy === u.id ? $t('admin.users.autoRenewUpdating') : (u.auto_renew ? $t('admin.users.on') : $t('admin.users.off')) }}</span>
                </button>
              </td>
              <td class="px-4 py-3.5 text-xs text-surface-500 dark:text-surface-400">
                <span :title="absoluteTime(u.created_at)">{{ relativeTime(u.created_at) }}</span>
              </td>
              <td class="px-4 py-3.5 text-xs text-surface-500 dark:text-surface-400">
                <span v-if="u.last_active_at" :title="absoluteTime(u.last_active_at)">{{ relativeTime(u.last_active_at) }}</span>
                <span v-else class="text-surface-400 dark:text-surface-600">—</span>
              </td>
              <td class="sticky right-0 border-l border-surface-100 bg-surface-0 px-4 py-3.5 whitespace-nowrap transition-colors group-hover/row:bg-surface-50/95 dark:border-surface-700 dark:bg-surface-900 dark:group-hover/row:bg-surface-800/95">
                <div class="flex items-center justify-end gap-1.5">
                  <button
                    :title="u.status === 'suspended' ? $t('admin.users.unsuspend') : $t('admin.users.suspend')"
                    class="inline-flex h-7 items-center rounded-lg border border-surface-200 bg-surface-0 px-2 text-xs font-medium text-surface-600 transition-colors hover:bg-surface-50 hover:text-ink-900 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50"
                    @click="toggleSuspend(u)"
                  >
                    {{ u.status === 'suspended' ? $t('admin.users.unsuspend') : $t('admin.users.suspend') }}
                  </button>
                  <button
                    :title="$t('admin.users.delete')"
                    class="inline-flex h-7 items-center rounded-lg border border-transparent px-2 text-xs font-medium text-red-600 transition-colors hover:border-red-200 hover:bg-red-50 dark:text-red-400 dark:hover:border-red-500/30 dark:hover:bg-red-500/15"
                    @click="destroy(u)"
                  >
                    {{ $t('admin.users.delete') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        </div>
      </template>

      <!-- "No matches" — different from the totally-empty state because
           the user just typed a filter that returned nothing. -->
      <div
        v-else-if="users.length > 0 && paged.length === 0"
        class="rounded-2xl border border-dashed border-surface-200 bg-surface-50/60 px-6 py-12 text-center text-sm text-surface-500 dark:border-surface-700 dark:bg-surface-900/40 dark:text-surface-400"
      >
        {{ $t('admin.users.emptyDescription') }}
      </div>

      <EmptyState
        v-else-if="showEmptyState"
        icon="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0"
        :title="$t('admin.users.empty')"
        :description="$t('admin.users.emptyDescription')"
      />

      <!-- Pagination — only when filtered total exceeds page size. -->
      <div v-if="total > pageSize" class="mt-4 flex items-center justify-between text-xs text-surface-500 dark:text-surface-400">
        <span class="tabular-nums">{{ $t('admin.users.pageSummary', { from: pageFrom, to: pageTo, total }) }}</span>
        <div class="flex items-center gap-1">
          <button
            type="button"
            :disabled="page <= 1"
            :title="$t('admin.users.pagePrev')"
            class="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-surface-200 bg-surface-0 transition-colors hover:border-surface-300 disabled:cursor-not-allowed disabled:opacity-40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700"
            @click="page = Math.max(1, page - 1)"
          >
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18 9 12l6-6" /></svg>
          </button>
          <button
            v-for="n in pageCount"
            :key="n"
            type="button"
            class="inline-flex h-7 min-w-[1.75rem] items-center justify-center rounded-lg px-2 text-xs font-medium tabular-nums transition-colors"
            :class="n === page
              ? 'bg-accent-600 text-white dark:bg-accent-500'
              : 'border border-surface-200 bg-surface-0 text-surface-600 hover:border-surface-300 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700'"
            @click="page = n"
          >{{ n }}</button>
          <button
            type="button"
            :disabled="page >= pageCount"
            :title="$t('admin.users.pageNext')"
            class="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-surface-200 bg-surface-0 transition-colors hover:border-surface-300 disabled:cursor-not-allowed disabled:opacity-40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-200 dark:hover:bg-surface-700"
            @click="page = Math.min(pageCount, page + 1)"
          >
            <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6" /></svg>
          </button>
        </div>
      </div>
    </template>

    <!-- Create-user modal -->
    <div
      v-if="createModal.open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="createModal.open = false"
    >
      <div class="w-full max-w-md animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-700">
          <div>
            <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.users.create.title') }}</h2>
          </div>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="createModal.open = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-4 px-6 py-5" @submit.prevent="submitCreate">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300" for="create-email">{{ $t('admin.users.create.emailLabel') }}</label>
            <input
              id="create-email"
              v-model="createModal.email"
              type="email"
              required
              autocomplete="off"
              :placeholder="$t('admin.users.create.emailPlaceholder')"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-100"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300" for="create-password">{{ $t('admin.users.create.passwordLabel') }}</label>
            <div class="relative">
              <input
                id="create-password"
                v-model="createModal.password"
                :type="createModal.showPassword ? 'text' : 'password'"
                required
                minlength="8"
                autocomplete="new-password"
                :placeholder="$t('admin.users.create.passwordPlaceholder')"
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 pr-11 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-100"
              />
              <button
                type="button"
                :title="createModal.showPassword ? $t('admin.users.create.hidePassword') : $t('admin.users.create.showPassword')"
                class="absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-700 dark:hover:text-surface-200"
                @click="createModal.showPassword = !createModal.showPassword"
              >
                <svg v-if="createModal.showPassword" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" /><path d="M1 1l22 22" /></svg>
                <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg>
              </button>
            </div>
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300" for="create-balance">{{ $t('admin.users.create.initialBalanceLabel') }}</label>
            <input
              id="create-balance"
              v-model="createModal.initialBalanceYuan"
              type="number"
              min="0"
              step="0.01"
              :placeholder="$t('admin.users.create.initialBalancePlaceholder')"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-100"
            />
            <p class="mt-1 text-2xs text-surface-500 dark:text-surface-400">{{ $t('admin.users.create.initialBalanceHint') }}</p>
          </div>
          <p v-if="createModal.err" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ createModal.err }}</p>
          <footer class="-mx-6 -mb-5 flex justify-end gap-2 border-t border-surface-100 px-6 py-4 dark:border-surface-700">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-600 dark:text-surface-200 dark:hover:bg-surface-800" @click="createModal.open = false">{{ $t('admin.users.create.cancel') }}</button>
            <button type="submit" :disabled="createModal.busy" class="inline-flex h-9 items-center rounded-xl bg-accent-600 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-accent-500 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-500 dark:hover:bg-accent-400">{{ createModal.busy ? $t('admin.users.create.submitting') : $t('admin.users.create.submit') }}</button>
          </footer>
        </form>
      </div>
    </div>

    <!-- Balance adjust modal (kept) -->
    <div
      v-if="balanceModal.open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="balanceModal.open = false"
    >
      <div class="w-full max-w-md animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-700">
          <div>
            <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.users.balance.title') }}</h2>
            <p class="mt-0.5 text-xs text-surface-500 dark:text-surface-400">{{ $t('admin.users.balance.current', { amount: balanceModal.user ? formatYuan(balanceModal.user.balance_cents) : '' }) }}</p>
          </div>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="balanceModal.open = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-4 px-6 py-5" @submit.prevent="submitBalance">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.users.balance.amountLabel') }}</label>
            <input
              v-model.number="balanceModal.delta"
              type="number"
              required
              :placeholder="$t('admin.users.balance.amountPlaceholder')"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-100"
            />
            <p class="mt-1 text-2xs text-surface-500 dark:text-surface-400">{{ balanceModal.delta > 0 ? '+' : '' }}{{ formatYuan(balanceModal.delta) }}（{{ balanceModal.delta > 0 ? $t('admin.users.balance.deposit') : balanceModal.delta < 0 ? $t('admin.users.balance.deduct') : $t('admin.users.balance.zeroMarker') }}）</p>
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.users.balance.reasonLabel') }}</label>
            <input
              v-model="balanceModal.reason"
              type="text"
              required
              :placeholder="$t('admin.users.balance.reasonPlaceholder')"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-600 dark:bg-surface-800 dark:text-surface-100"
            />
            <p class="mt-1 text-2xs text-surface-500 dark:text-surface-400">{{ $t('admin.users.balance.ledgerNote') }}</p>
          </div>
          <p v-if="balanceModal.err" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ balanceModal.err }}</p>
          <footer class="-mx-6 -mb-5 flex justify-end gap-2 border-t border-surface-100 px-6 py-4 dark:border-surface-700">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-600 dark:text-surface-200 dark:hover:bg-surface-800" @click="balanceModal.open = false">{{ $t('admin.users.balance.cancel') }}</button>
            <button type="submit" :disabled="balanceModal.busy" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">{{ balanceModal.busy ? $t('admin.users.balance.submitting') : $t('admin.users.balance.confirm') }}</button>
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

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
/* Slide-fade: batch bar slides down from the toolbar.
   Pairs nicely with the rounded pill so it feels anchored. */
.slide-fade-enter-active,
.slide-fade-leave-active {
  transition: opacity 0.18s cubic-bezier(0.16, 1, 0.3, 1), transform 0.18s cubic-bezier(0.16, 1, 0.3, 1);
}
.slide-fade-enter-from,
.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
