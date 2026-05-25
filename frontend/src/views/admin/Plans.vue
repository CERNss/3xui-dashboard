<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminPlansApi, type AdminPlan, type CreatePlanInput } from '@/api/admin/plans'
import { provisioningPoolsApi, type ProvisioningPool } from '@/api/admin/provisioningPools'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { t } = useI18n()

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const plans = ref<AdminPlan[]>([])
const pools = ref<ProvisioningPool[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const modal = ref<{
  open: boolean
  mode: 'create' | 'edit'
  id: number | null
  form: CreatePlanInput
  busy: boolean
  err: string | null
}>({
  open: false,
  mode: 'create',
  id: null,
  form: blankForm(),
  busy: false,
  err: null,
})

function blankForm(): CreatePlanInput {
  return {
    name: '',
    description: '',
    duration_days: 30,
    traffic_limit_bytes: 100 * 1024 * 1024 * 1024, // 100 GB
    price_cents: 500,
    ip_limit: 0,
    provisioning_pool_id: null,
    enabled: true,
  }
}

// Convenience UI fields: traffic in GB (with 0 = unlimited).
// Both sides Math.round so a user typing "100.5" can't ship a fractional-byte
// value to the backend (GORM int64 would coerce, losing precision).
const trafficGB = computed({
  get: () => Math.round(modal.value.form.traffic_limit_bytes / (1024 * 1024 * 1024)),
  set: (v: number) => (modal.value.form.traffic_limit_bytes = Math.round(v) * 1024 * 1024 * 1024),
})
// price_cents is canonical (integer cents). On set we Math.round both the
// multiplication (handles 0.1*100 = 10.000000000000002) and the result.
// On get we return the raw division — for integer cents under ~$1B the
// quotient is representable cleanly; v-model.number will read it back.
const priceYuan = computed({
  get: () => modal.value.form.price_cents / 100,
  set: (v: number) => (modal.value.form.price_cents = Math.round(v * 100)),
})

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [planRows, poolRows] = await Promise.all([
      adminPlansApi.list(),
      provisioningPoolsApi.list(),
    ])
    plans.value = planRows
    pools.value = poolRows
  } catch (e: any) {
    error.value = formatError(e, t('admin.plans.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  modal.value = {
    open: true,
    mode: 'create',
    id: null,
    form: blankForm(),
    busy: false,
    err: null,
  }
}

function openEdit(p: AdminPlan) {
  modal.value = {
    open: true,
    mode: 'edit',
    id: p.id,
    form: {
      name: p.name,
      description: p.description ?? '',
      duration_days: p.duration_days,
      traffic_limit_bytes: p.traffic_limit_bytes,
      price_cents: p.price_cents,
      ip_limit: p.ip_limit ?? 0,
      provisioning_pool_id: p.provisioning_pool_id ?? null,
      enabled: p.enabled,
    },
    busy: false,
    err: null,
  }
}

async function submit() {
  modal.value.busy = true
  modal.value.err = null
  // Final integer-coerce of any field the backend expects as int64.
  // The computed setters already round, but a user could paste e.g.
  // "30.5" directly into duration_days which is bound raw.
  const f = modal.value.form
  const payload: CreatePlanInput = {
    ...f,
    duration_days: Math.max(1, Math.round(f.duration_days)),
    traffic_limit_bytes: Math.max(0, Math.round(f.traffic_limit_bytes)),
    price_cents: Math.max(0, Math.round(f.price_cents)),
    ip_limit: Math.max(0, Math.round(f.ip_limit ?? 0)),
    provisioning_pool_id: f.provisioning_pool_id ? Number(f.provisioning_pool_id) : null,
  }
  try {
    if (modal.value.mode === 'create') {
      await adminPlansApi.create(payload)
    } else if (modal.value.id) {
      await adminPlansApi.update(modal.value.id, payload)
    }
    modal.value.open = false
    await reload()
  } catch (e: any) {
    modal.value.err = formatError(e, t('admin.plans.saveFailed'))
  } finally {
    modal.value.busy = false
  }
}

async function destroy(p: AdminPlan) {
  const ok = await askConfirm({
    title: t('admin.plans.confirmDelete'),
    message: t('admin.plans.confirmDeleteMsg', { name: p.name }),
    variant: 'danger',
    confirmLabel: t('admin.plans.delete'),
  })
  if (!ok) return
  try {
    await adminPlansApi.remove(p.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, t('admin.plans.deleteFailed'))
  }
}

async function toggleEnable(p: AdminPlan) {
  try {
    await adminPlansApi.update(p.id, { enabled: !p.enabled })
    await reload()
  } catch (e: any) {
    error.value = formatError(e, t('admin.plans.toggleFailed'))
  }
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

function formatTraffic(bytes: number): string {
  if (bytes === 0) return '∞'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1024) return (gb / 1024).toFixed(1) + ' TB'
  return Math.round(gb) + ' GB'
}

function poolName(id?: number | null): string {
  if (!id) return t('admin.plans.provisioningPoolNone')
  return pools.value.find(p => p.id === id)?.name ?? `#${id}`
}

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-5 flex justify-end">
      <div class="flex items-center gap-2">
        <button
          class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
          @click="openCreate"
        >
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14" /></svg>
          {{ $t('admin.plans.create') }}
        </button>
        <button
          class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800"
          :title="$t('admin.plans.reload')"
          @click="reload"
        >
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        </button>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" :rows="4" />

    <div
      v-else-if="plans.length > 0"
      class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
    >
      <table class="min-w-full text-sm">
        <thead class="text-left text-xs font-semibold uppercase tracking-wider text-surface-500 dark:text-surface-400">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">{{ $t('admin.plans.column.id') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.plans.column.name') }}</th>
            <th class="px-6 py-3 text-right font-medium">{{ $t('admin.plans.column.price') }}</th>
            <th class="px-6 py-3 text-right font-medium">{{ $t('admin.plans.column.traffic') }}</th>
            <th class="px-6 py-3 text-right font-medium">{{ $t('admin.plans.column.duration') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.plans.column.status') }}</th>
            <th class="px-6 py-3 text-right font-medium">{{ $t('admin.plans.column.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="p in plans" :key="p.id" :class="!p.enabled ? 'opacity-60' : ''" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-400 tabular-nums">#{{ p.id }}</td>
            <td class="px-6 py-3.5">
              <div class="font-medium text-ink-900 dark:text-surface-50">{{ p.name }}</div>
              <div v-if="p.description" class="mt-0.5 text-2xs text-surface-500">{{ p.description }}</div>
              <div class="mt-1 text-2xs text-surface-500">{{ $t('admin.plans.provisioningPool') }}: {{ poolName(p.provisioning_pool_id) }}</div>
            </td>
            <td class="px-6 py-3.5 text-right tabular-nums font-medium text-ink-900 dark:text-surface-50">{{ formatYuan(p.price_cents) }}</td>
            <td class="px-6 py-3.5 text-right tabular-nums">{{ formatTraffic(p.traffic_limit_bytes) }}</td>
            <td class="px-6 py-3.5 text-right tabular-nums">{{ p.duration_days }} {{ $t('admin.plans.unitDays') }}</td>
            <td class="px-6 py-3.5">
              <button
                class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors duration-200 ease-brand"
                :class="p.enabled ? 'bg-accent-500' : 'bg-surface-200 dark:bg-surface-700'"
                @click="toggleEnable(p)"
              >
                <span class="inline-block h-4 w-4 transform rounded-full bg-white shadow-card transition-transform duration-200 ease-brand" :class="p.enabled ? 'translate-x-4' : 'translate-x-0.5'" />
              </button>
            </td>
            <td class="px-6 py-3.5">
              <div class="flex items-center justify-end gap-0.5">
                <button :title="$t('admin.plans.edit')" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="openEdit(p)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 20h9M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" /></svg>
                </button>
                <button :title="$t('admin.plans.delete')" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="destroy(p)">
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
      icon="M9 11l3 3L22 4M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"
      :title="$t('admin.plans.empty')"
      :description="$t('admin.plans.emptyDescription')"
      :action-label="$t('admin.plans.create')"
      @action="openCreate"
    />

    <!-- Create/edit modal -->
    <div
      v-if="modal.open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="modal.open = false"
    >
      <div class="w-full max-w-xl animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-800">
          <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ modal.mode === 'create' ? $t('admin.plans.createTitle') : $t('admin.plans.editTitle', { id: modal.id }) }}</h2>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="modal.open = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-4 px-6 py-5" @submit.prevent="submit">
          <div class="grid grid-cols-2 gap-3.5">
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.name') }}</label>
              <input v-model="modal.form.name" type="text" required :placeholder="$t('admin.plans.namePlaceholder')"
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.description') }}</label>
              <input v-model="modal.form.description" type="text" :placeholder="$t('admin.plans.descriptionPlaceholder')"
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.price') }}</label>
              <input v-model.number="priceYuan" type="number" step="0.01" min="0" required
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.durationDays') }}</label>
              <input v-model.number="modal.form.duration_days" type="number" min="1" required
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.trafficGB') }}</label>
              <input v-model.number="trafficGB" type="number" min="0" required
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.ipLimit') }}</label>
              <input v-model.number="modal.form.ip_limit" type="number" min="0"
                class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.plans.provisioningPool') }}</label>
              <select v-model="modal.form.provisioning_pool_id" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900">
                <option :value="null">{{ $t('admin.plans.provisioningPoolNone') }}</option>
                <option v-for="pool in pools" :key="pool.id" :value="pool.id">
                  {{ pool.name }}{{ pool.enabled ? '' : ' · disabled' }}
                </option>
              </select>
              <p class="mt-1.5 text-xs text-surface-500">{{ $t('admin.plans.provisioningPoolHint') }}</p>
            </div>
            <div class="col-span-2 flex items-center gap-2">
              <input id="plan-enable" v-model="modal.form.enabled" type="checkbox" class="h-4 w-4 rounded-md border-surface-300 text-accent-600 focus:ring-accent-500/30" />
              <label for="plan-enable" class="text-sm text-surface-700 dark:text-surface-300">{{ $t('admin.plans.enableLabel') }}</label>
            </div>
          </div>
          <p v-if="modal.err" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ modal.err }}</p>
          <footer class="flex justify-end gap-2 border-t border-surface-100 -mx-6 -mb-5 px-6 py-4 dark:border-surface-800">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="modal.open = false">{{ $t('common.cancel') }}</button>
            <button type="submit" :disabled="modal.busy" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">{{ modal.busy ? $t('admin.plans.saving') : $t('admin.plans.submit') }}</button>
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
