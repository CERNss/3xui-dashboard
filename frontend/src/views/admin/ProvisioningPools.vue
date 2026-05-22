<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { inboundsApi, type FleetInbound } from '@/api/admin/inbounds'
import {
  provisioningPoolsApi,
  type ProvisioningPool,
  type ProvisioningPoolInput,
  type ProvisioningPoolTarget,
  type ProvisioningPoolTargetInput,
} from '@/api/admin/provisioningPools'
import EmptyState from '@/components/common/EmptyState.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { t } = useI18n()
const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const pools = ref<ProvisioningPool[]>([])
const inbounds = ref<FleetInbound[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const poolModal = ref<{
  open: boolean
  mode: 'create' | 'edit'
  id: number | null
  form: ProvisioningPoolInput
  protocolText: string
  busy: boolean
  err: string | null
}>({
  open: false,
  mode: 'create',
  id: null,
  form: blankPool(),
  protocolText: '',
  busy: false,
  err: null,
})

const targetModal = ref<{
  open: boolean
  poolID: number | null
  form: ProvisioningPoolTargetInput
  busy: boolean
  err: string | null
}>({
  open: false,
  poolID: null,
  form: blankTarget(),
  busy: false,
  err: null,
})

const inboundOptions = computed(() =>
  inbounds.value
    .filter(row => row.inbound.enable)
    .sort((a, b) => a.node_name.localeCompare(b.node_name) || a.inbound.port - b.inbound.port),
)

function blankPool(): ProvisioningPoolInput {
  return {
    name: '',
    description: '',
    enabled: true,
    auto_create: false,
    port_min: null,
    port_max: null,
    allowed_protocols: [],
  }
}

function blankTarget(): ProvisioningPoolTargetInput {
  return {
    node_id: 0,
    inbound_tag: '',
    protocol: '',
    max_clients: 0,
    priority: 100,
    enabled: true,
  }
}

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [poolRows, fleet] = await Promise.all([
      provisioningPoolsApi.list(),
      inboundsApi.fleet(),
    ])
    pools.value = poolRows
    inbounds.value = fleet.inbounds
  } catch (e) {
    error.value = formatError(e, t('admin.provisioningPools.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreatePool() {
  poolModal.value = {
    open: true,
    mode: 'create',
    id: null,
    form: blankPool(),
    protocolText: '',
    busy: false,
    err: null,
  }
}

function openEditPool(pool: ProvisioningPool) {
  poolModal.value = {
    open: true,
    mode: 'edit',
    id: pool.id,
    form: {
      name: pool.name,
      description: pool.description ?? '',
      enabled: pool.enabled,
      auto_create: pool.auto_create,
      port_min: pool.port_min ?? null,
      port_max: pool.port_max ?? null,
      allowed_protocols: pool.allowed_protocols ?? [],
    },
    protocolText: (pool.allowed_protocols ?? []).join(', '),
    busy: false,
    err: null,
  }
}

async function savePool() {
  poolModal.value.busy = true
  poolModal.value.err = null
  const f = poolModal.value.form
  const payload: ProvisioningPoolInput = {
    ...f,
    name: f.name.trim(),
    description: f.description?.trim() ?? '',
    port_min: f.port_min ? Math.round(f.port_min) : null,
    port_max: f.port_max ? Math.round(f.port_max) : null,
    allowed_protocols: poolModal.value.protocolText
      .split(',')
      .map(v => v.trim().toLowerCase())
      .filter(Boolean),
  }
  try {
    if (poolModal.value.mode === 'create') {
      await provisioningPoolsApi.create(payload)
    } else if (poolModal.value.id) {
      await provisioningPoolsApi.update(poolModal.value.id, payload)
    }
    poolModal.value.open = false
    await reload()
  } catch (e) {
    poolModal.value.err = formatError(e, t('admin.provisioningPools.saveFailed'))
  } finally {
    poolModal.value.busy = false
  }
}

async function deletePool(pool: ProvisioningPool) {
  const ok = await askConfirm({
    title: t('admin.provisioningPools.confirmDelete'),
    message: t('admin.provisioningPools.confirmDeleteMsg', { name: pool.name }),
    variant: 'danger',
    confirmLabel: t('admin.provisioningPools.delete'),
  })
  if (!ok) return
  try {
    await provisioningPoolsApi.remove(pool.id)
    await reload()
  } catch (e) {
    error.value = formatError(e, t('admin.provisioningPools.saveFailed'))
  }
}

function openAddTarget(pool: ProvisioningPool) {
  targetModal.value = {
    open: true,
    poolID: pool.id,
    form: blankTarget(),
    busy: false,
    err: null,
  }
  if (inboundOptions.value.length > 0) {
    applyInboundChoice(inboundKey(inboundOptions.value[0]))
  }
}

function inboundKey(row: FleetInbound) {
  return `${row.node_id}|${row.inbound.tag}`
}

function applyInboundChoice(key: string) {
  const [nodeIDRaw, tag] = key.split('|')
  const nodeID = Number(nodeIDRaw)
  const row = inboundOptions.value.find(x => x.node_id === nodeID && x.inbound.tag === tag)
  if (!row) return
  targetModal.value.form.node_id = row.node_id
  targetModal.value.form.inbound_tag = row.inbound.tag
  targetModal.value.form.protocol = row.inbound.protocol
}

function currentInboundKey() {
  return `${targetModal.value.form.node_id}|${targetModal.value.form.inbound_tag}`
}

async function saveTarget() {
  if (!targetModal.value.poolID) return
  targetModal.value.busy = true
  targetModal.value.err = null
  const f = targetModal.value.form
  try {
    await provisioningPoolsApi.addTarget(targetModal.value.poolID, {
      ...f,
      max_clients: Math.max(0, Math.round(f.max_clients)),
      priority: Math.max(0, Math.round(f.priority)),
    })
    targetModal.value.open = false
    await reload()
  } catch (e) {
    targetModal.value.err = formatError(e, t('admin.provisioningPools.targetSaveFailed'))
  } finally {
    targetModal.value.busy = false
  }
}

async function toggleTarget(target: ProvisioningPoolTarget) {
  try {
    await provisioningPoolsApi.updateTarget(target.id, { enabled: !target.enabled })
    await reload()
  } catch (e) {
    error.value = formatError(e, t('admin.provisioningPools.targetSaveFailed'))
  }
}

async function deleteTarget(target: ProvisioningPoolTarget) {
  try {
    await provisioningPoolsApi.removeTarget(target.id)
    await reload()
  } catch (e) {
    error.value = formatError(e, t('admin.provisioningPools.targetDeleteFailed'))
  }
}

function capacityText(target: ProvisioningPoolTarget) {
  const used = target.used_clients ?? 0
  if (!target.max_clients) return `${used} / ${t('admin.provisioningPools.unlimited')}`
  return `${used} / ${target.max_clients}`
}

function protocolsText(pool: ProvisioningPool) {
  return pool.allowed_protocols?.length ? pool.allowed_protocols.join(', ') : t('admin.provisioningPools.unlimited')
}

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.provisioningPools.title') }}</h1>
        <p class="mt-1.5 text-sm text-surface-500">{{ $t('admin.provisioningPools.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500" @click="openCreatePool">
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14" /></svg>
          {{ $t('admin.provisioningPools.add') }}
        </button>
        <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800" @click="reload">
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        </button>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" :rows="4" />

    <div v-else-if="pools.length > 0" class="space-y-4">
      <section v-for="pool in pools" :key="pool.id" class="rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
        <header class="flex flex-col gap-3 border-b border-surface-100 px-5 py-4 dark:border-surface-800 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ pool.name }}</h2>
              <span class="rounded-full px-2 py-0.5 text-2xs font-medium ring-1 ring-inset" :class="pool.enabled ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800' : 'bg-surface-100 text-surface-500 ring-surface-200 dark:bg-surface-800 dark:ring-surface-700'">
                {{ pool.enabled ? $t('admin.provisioningPools.enabled') : 'disabled' }}
              </span>
            </div>
            <p class="mt-1 text-xs text-surface-500">{{ pool.description || protocolsText(pool) }}</p>
          </div>
          <div class="flex items-center gap-1.5">
            <button class="rounded-lg border border-surface-200 px-2.5 py-1 text-xs font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="openAddTarget(pool)">{{ $t('admin.provisioningPools.addTarget') }}</button>
            <button :title="$t('admin.provisioningPools.edit')" class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="openEditPool(pool)">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
            </button>
            <button :title="$t('admin.provisioningPools.delete')" class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="deletePool(pool)">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
            </button>
          </div>
        </header>

        <div v-if="pool.targets?.length" class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
              <tr class="border-b border-surface-100 dark:border-surface-800">
                <th class="px-5 py-3 font-medium">{{ $t('admin.provisioningPools.column.target') }}</th>
                <th class="px-5 py-3 font-medium">{{ $t('admin.provisioningPools.column.capacity') }}</th>
                <th class="px-5 py-3 font-medium">{{ $t('admin.provisioningPools.priority') }}</th>
                <th class="px-5 py-3 font-medium">{{ $t('admin.provisioningPools.column.status') }}</th>
                <th class="px-5 py-3 text-right font-medium">{{ $t('admin.provisioningPools.column.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
              <tr v-for="target in pool.targets" :key="target.id" class="hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
                <td class="px-5 py-3.5">
                  <div class="font-medium text-ink-900 dark:text-surface-50">{{ target.node_name || ('#' + target.node_id) }}</div>
                  <div class="mt-0.5 font-mono text-2xs text-surface-500">{{ target.inbound_tag }} · {{ target.protocol || '-' }}</div>
                </td>
                <td class="px-5 py-3.5 tabular-nums text-surface-600 dark:text-surface-300">{{ capacityText(target) }}</td>
                <td class="px-5 py-3.5 tabular-nums text-surface-600 dark:text-surface-300">{{ target.priority }}</td>
                <td class="px-5 py-3.5">
                  <button class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors duration-200" :class="target.enabled ? 'bg-accent-500' : 'bg-surface-200 dark:bg-surface-700'" @click="toggleTarget(target)">
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white shadow-card transition-transform duration-200" :class="target.enabled ? 'translate-x-4' : 'translate-x-0.5'" />
                  </button>
                </td>
                <td class="px-5 py-3.5 text-right">
                  <button class="rounded-lg px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/40" @click="deleteTarget(target)">{{ $t('admin.provisioningPools.delete') }}</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="px-5 py-5 text-sm text-surface-500">{{ $t('admin.provisioningPools.emptyDescription') }}</div>
      </section>
    </div>

    <EmptyState
      v-else
      icon="M4 7h16M7 7v10a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2V7M9 11h6M9 15h6"
      :title="$t('admin.provisioningPools.empty')"
      :description="$t('admin.provisioningPools.emptyDescription')"
      :action-label="$t('admin.provisioningPools.add')"
      @action="openCreatePool"
    />

    <div v-if="poolModal.open" class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm" @click.self="poolModal.open = false">
      <div class="w-full max-w-xl animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-800">
          <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ poolModal.mode === 'create' ? $t('admin.provisioningPools.createTitle') : $t('admin.provisioningPools.editTitle', { id: poolModal.id }) }}</h2>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="poolModal.open = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-4 px-6 py-5" @submit.prevent="savePool">
          <div class="grid grid-cols-2 gap-3.5">
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.name') }}</label>
              <input v-model="poolModal.form.name" required :placeholder="$t('admin.provisioningPools.namePlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.description') }}</label>
              <input v-model="poolModal.form.description" :placeholder="$t('admin.provisioningPools.descriptionPlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.portRange') }}</label>
              <div class="grid grid-cols-2 gap-2">
                <input v-model.number="poolModal.form.port_min" type="number" min="1" max="65535" placeholder="10000" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
                <input v-model.number="poolModal.form.port_max" type="number" min="1" max="65535" placeholder="20000" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
              </div>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.allowedProtocols') }}</label>
              <input v-model="poolModal.protocolText" :placeholder="$t('admin.provisioningPools.protocolPlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2 flex items-center gap-2">
              <input id="pool-enable" v-model="poolModal.form.enabled" type="checkbox" class="h-4 w-4 rounded-md border-surface-300 text-accent-600 focus:ring-accent-500/30" />
              <label for="pool-enable" class="text-sm text-surface-700 dark:text-surface-300">{{ $t('admin.provisioningPools.enabled') }}</label>
            </div>
            <div class="col-span-2 flex items-center gap-2">
              <input id="pool-auto-create" v-model="poolModal.form.auto_create" type="checkbox" class="h-4 w-4 rounded-md border-surface-300 text-accent-600 focus:ring-accent-500/30" />
              <label for="pool-auto-create" class="text-sm text-surface-700 dark:text-surface-300">{{ $t('admin.provisioningPools.autoCreate') }}</label>
            </div>
          </div>
          <p v-if="poolModal.err" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ poolModal.err }}</p>
          <footer class="flex justify-end gap-2 border-t border-surface-100 -mx-6 -mb-5 px-6 py-4 dark:border-surface-800">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="poolModal.open = false">{{ $t('common.cancel') }}</button>
            <button type="submit" :disabled="poolModal.busy" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">{{ poolModal.busy ? $t('admin.provisioningPools.saving') : $t('admin.provisioningPools.submit') }}</button>
          </footer>
        </form>
      </div>
    </div>

    <div v-if="targetModal.open" class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm" @click.self="targetModal.open = false">
      <div class="w-full max-w-lg animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-800">
          <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.provisioningPools.targetCreateTitle') }}</h2>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="targetModal.open = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-4 px-6 py-5" @submit.prevent="saveTarget">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.inbound') }}</label>
            <select :value="currentInboundKey()" required class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" @change="applyInboundChoice(($event.target as HTMLSelectElement).value)">
              <option v-for="row in inboundOptions" :key="inboundKey(row)" :value="inboundKey(row)">
                {{ row.node_name }} · {{ row.inbound.remark || row.inbound.tag }} · :{{ row.inbound.port }} · {{ row.inbound.protocol }}
              </option>
            </select>
          </div>
          <div class="grid grid-cols-2 gap-3.5">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.maxClients') }}</label>
              <input v-model.number="targetModal.form.max_clients" type="number" min="0" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.provisioningPools.priority') }}</label>
              <input v-model.number="targetModal.form.priority" type="number" min="0" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm tabular-nums focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
          </div>
          <div class="flex items-center gap-2">
            <input id="target-enable" v-model="targetModal.form.enabled" type="checkbox" class="h-4 w-4 rounded-md border-surface-300 text-accent-600 focus:ring-accent-500/30" />
            <label for="target-enable" class="text-sm text-surface-700 dark:text-surface-300">{{ $t('admin.provisioningPools.enabled') }}</label>
          </div>
          <p v-if="targetModal.err" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ targetModal.err }}</p>
          <footer class="flex justify-end gap-2 border-t border-surface-100 -mx-6 -mb-5 px-6 py-4 dark:border-surface-800">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="targetModal.open = false">{{ $t('common.cancel') }}</button>
            <button type="submit" :disabled="targetModal.busy || inboundOptions.length === 0" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">{{ targetModal.busy ? $t('admin.provisioningPools.saving') : $t('admin.provisioningPools.submit') }}</button>
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
