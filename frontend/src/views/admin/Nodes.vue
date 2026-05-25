<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatError } from '@/utils/format'

import { nodesApi, type Node, type NodeInput } from '@/api/admin/nodes'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import { useConfirm } from '@/composables/useConfirm'

const { t } = useI18n()

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const nodes = ref<Node[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const togglingNodeId = ref<number | null>(null)

const showEditor = ref(false)
const saving = ref(false)
const formErr = ref<string | null>(null)
const editingNode = ref<Node | null>(null)

const isEditing = computed(() => editingNode.value !== null)

function blankForm(): NodeInput {
  return {
    name: '',
    scheme: 'https',
    host: '',
    port: 2053,
    base_path: '',
    api_token: '',
    enabled: true,
  }
}

const form = ref<NodeInput>(blankForm())

function openCreate() {
  editingNode.value = null
  form.value = blankForm()
  formErr.value = null
  showEditor.value = true
}

function openEdit(n: Node) {
  editingNode.value = n
  form.value = {
    name: n.name,
    scheme: n.scheme,
    host: n.host,
    port: n.port,
    base_path: n.base_path ?? '',
    api_token: '',
    enabled: n.enabled,
  }
  formErr.value = null
  showEditor.value = true
}

function closeEditor() {
  showEditor.value = false
  editingNode.value = null
  form.value = blankForm()
  formErr.value = null
}

function buildPayload(): NodeInput {
  return {
    ...form.value,
    name: form.value.name.trim(),
    host: form.value.host.trim(),
    base_path: form.value.base_path?.trim() ?? '',
    api_token: form.value.api_token?.trim() ?? '',
    port: Math.max(1, Math.min(65535, Math.round(form.value.port || 0))),
  }
}

async function reload() {
  loading.value = true
  error.value = null
  try {
    nodes.value = await nodesApi.list()
  } catch (e: any) {
    error.value = formatError(e, t('admin.nodes.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function probe(id: number) {
  try {
    await nodesApi.probe(id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, t('admin.nodes.probeFailed'))
  }
}

async function toggleEnable(n: Node) {
  togglingNodeId.value = n.id
  try {
    if (n.enabled) await nodesApi.disable(n.id)
    else await nodesApi.enable(n.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, t('admin.nodes.toggleFailed'))
  } finally {
    togglingNodeId.value = null
  }
}

async function destroy(n: Node) {
  const ok = await askConfirm({
    title: t('admin.nodes.confirmDelete'),
    message: t('admin.nodes.confirmDeleteMsg', { name: n.name }),
    variant: 'danger',
    confirmLabel: t('admin.nodes.delete'),
  })
  if (!ok) return
  try {
    await nodesApi.remove(n.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, t('admin.nodes.deleteFailed'))
  }
}

async function submit() {
  formErr.value = null
  const payload = buildPayload()
  if (!payload.name || !payload.host || (!isEditing.value && !payload.api_token)) {
    formErr.value = isEditing.value ? t('admin.nodes.requiredFieldsEdit') : t('admin.nodes.requiredFields')
    return
  }
  saving.value = true
  try {
    if (editingNode.value) {
      await nodesApi.update(editingNode.value.id, payload)
    } else {
      await nodesApi.create(payload)
    }
    closeEditor()
    await reload()
  } catch (e: any) {
    formErr.value = formatError(e, isEditing.value ? t('admin.nodes.updateFailed') : t('admin.nodes.createFailed'))
  } finally {
    saving.value = false
  }
}

type NodeDisplayStatus = 'online' | 'offline' | 'unknown' | 'disabled'

function nodeDisplayStatus(n: Node): NodeDisplayStatus {
  if (!n.enabled) return 'disabled'
  if (n.status === 'online' || n.status === 'offline' || n.status === 'unknown') {
    return n.status
  }
  return 'unknown'
}

function nodeStatusText(n: Node): string {
  const status = nodeDisplayStatus(n)
  if (status === 'disabled') return t('admin.status.nodeState.disabled')
  return t(`admin.nodes.status.${status}`)
}

function nodeStatusBadgeClass(n: Node): string {
  switch (nodeDisplayStatus(n)) {
    case 'online':
      return 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
    case 'offline':
      return 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'
    case 'disabled':
      return 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-700/55 dark:text-surface-100 dark:ring-surface-500/50'
    default:
      return 'bg-surface-100 text-surface-500 ring-surface-200 dark:bg-surface-800 dark:text-surface-400 dark:ring-surface-700'
  }
}

function nodeStatusDotClass(n: Node): string {
  switch (nodeDisplayStatus(n)) {
    case 'online':
      return 'bg-accent-500 shadow-[0_0_0_3px_rgba(20,184,166,0.18)]'
    case 'offline':
      return 'bg-red-500'
    case 'disabled':
      return 'bg-surface-500 dark:bg-surface-300'
    default:
      return 'bg-surface-400'
  }
}

function normalizedBasePath(basePath: string | undefined | null): string {
  const raw = (basePath ?? '').trim()
  if (!raw || raw === '/') return ''
  const withLeading = raw.startsWith('/') ? raw : `/${raw}`
  return withLeading.endsWith('/') ? withLeading.slice(0, -1) : withLeading
}

function panelInboundURL(n: Node): string {
  return `${n.scheme}://${n.host}:${n.port}${normalizedBasePath(n.base_path)}/panel/inbounds`
}

function nodeConnectionURL(n: Node): string {
  return `${n.scheme}://${n.host}:${n.port}${normalizedBasePath(n.base_path)}`
}

function compactHost(host: string): string {
  // Iterate by code point so multi-byte chars (CJK, IDN, emoji) stay
  // whole; string.slice() splits surrogate pairs.
  const chars = Array.from(host.trim())
  if (chars.length <= 24) return chars.join('')
  return `${chars.slice(0, 13).join('')}…${chars.slice(-8).join('')}`
}

function nodeConnectionLabel(n: Node): string {
  const pathLabel = normalizedBasePath(n.base_path) ? '/...' : ''
  return `${n.scheme}://${compactHost(n.host)}:${n.port}${pathLabel}`
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
          {{ $t('admin.nodes.addNode') }}
        </button>
        <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800" @click="reload">
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        </button>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" :rows="4" />

    <template v-else>
      <div
        v-if="nodes.length > 0"
        class="grid gap-3 xl:hidden"
      >
        <article
          v-for="n in nodes"
          :key="n.id"
          class="rounded-2xl border border-surface-100 bg-surface-0 p-4 shadow-card dark:border-surface-800 dark:bg-surface-900"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="font-mono text-2xs text-surface-400">#{{ n.id }}</div>
              <h2 class="mt-1 break-words text-base font-semibold leading-6 text-ink-900 dark:text-surface-50">{{ n.name }}</h2>
            </div>
            <span
              class="inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset"
              :class="nodeStatusBadgeClass(n)"
            >
              <span class="h-1.5 w-1.5 rounded-full" :class="nodeStatusDotClass(n)" />
              {{ nodeStatusText(n) }}
            </span>
          </div>

          <div class="mt-3 rounded-xl bg-surface-50 px-3 py-2.5 dark:bg-surface-800/70">
            <div
              class="truncate font-mono text-xs leading-5 text-surface-600 dark:text-surface-300"
              :title="nodeConnectionURL(n)"
            >
              {{ nodeConnectionLabel(n) }}
            </div>
            <a
              class="mt-2 inline-flex items-center gap-1 text-xs font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-300"
              :href="panelInboundURL(n)"
              target="_blank"
              rel="noreferrer"
            >
              {{ $t('admin.nodes.openPanel') }}
              <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 17 17 7" /><path d="M8 7h9v9" /></svg>
            </a>
          </div>

          <dl class="mt-3 grid grid-cols-2 gap-2 text-sm sm:grid-cols-4">
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.cpuMem') }}</dt>
              <dd class="mt-1 tabular-nums text-surface-700 dark:text-surface-200">{{ n.cpu_pct.toFixed(1) }}% · {{ n.mem_pct.toFixed(1) }}%</dd>
            </div>
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.xray') }}</dt>
              <dd class="mt-1 truncate font-mono text-xs text-surface-700 dark:text-surface-200">{{ n.xray_version || '—' }}</dd>
            </div>
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.lastSeen') }}</dt>
              <dd class="mt-1 truncate text-xs text-surface-700 dark:text-surface-200">{{ n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : '—' }}</dd>
            </div>
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.schedule') }}</dt>
              <dd class="mt-1">
                <button
                  type="button"
                  role="switch"
                  :aria-checked="n.enabled"
                  :aria-label="n.enabled ? $t('admin.nodes.disable') : $t('admin.nodes.enable')"
                  :title="n.enabled ? $t('admin.nodes.disable') : $t('admin.nodes.enable')"
                  :disabled="togglingNodeId === n.id"
                  class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-accent-500/40 disabled:cursor-wait disabled:opacity-60"
                  :class="n.enabled ? 'bg-accent-500' : 'bg-surface-300 dark:bg-surface-700'"
                  @click="toggleEnable(n)"
                >
                  <span
                    class="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform"
                    :class="n.enabled ? 'translate-x-5' : 'translate-x-0'"
                  />
                </button>
              </dd>
            </div>
          </dl>

          <div class="mt-4 flex flex-wrap justify-end border-t border-surface-100 pt-3 dark:border-surface-800">
            <div class="inline-flex items-center gap-1 rounded-full border border-surface-200 bg-surface-50 px-1.5 py-1 shadow-sm dark:border-surface-700/80 dark:bg-surface-950/70">
              <button :title="$t('admin.nodes.probe')" class="flex h-8 w-8 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-surface-0 hover:text-accent-700 dark:hover:bg-surface-800 dark:hover:text-accent-300" @click="probe(n.id)">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 7-6-14-3 7H2" /></svg>
              </button>
              <button :title="$t('admin.nodes.edit')" class="flex h-8 w-8 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-surface-0 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="openEdit(n)">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
              </button>
              <button :title="$t('admin.nodes.delete')" class="flex h-8 w-8 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="destroy(n)">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
              </button>
            </div>
          </div>
        </article>
      </div>

      <div
        v-else
        class="rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
      >
        <EmptyState
          icon="M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01"
          :title="$t('admin.nodes.empty')"
          :description="$t('admin.nodes.emptyDescription')"
          :action-label="$t('admin.nodes.emptyAction')"
          @action="openCreate"
        />
      </div>

      <div
        v-if="nodes.length > 0"
        class="hidden overflow-x-auto overflow-y-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900 xl:block"
      >
        <table class="w-full min-w-[1120px] table-fixed text-sm">
          <colgroup>
            <col class="w-[56px]" />
            <col class="w-[162px]" />
            <col class="w-[250px]" />
            <col class="w-[116px]" />
            <col class="w-[122px]" />
            <col class="w-[88px]" />
            <col class="w-[166px]" />
            <col class="w-[82px]" />
            <col class="w-[130px]" />
          </colgroup>
          <thead class="bg-surface-50/70 text-left text-2xs font-semibold uppercase tracking-caps text-surface-500 dark:bg-surface-900/70 dark:text-surface-400">
            <tr class="h-11 border-b border-surface-100 dark:border-surface-800">
              <th class="px-3 align-middle font-medium">ID</th>
              <th class="px-3 align-middle font-medium">{{ $t('admin.nodes.column.name') }}</th>
              <th class="px-3 align-middle font-medium">{{ $t('admin.nodes.column.connection') }}</th>
              <th class="px-3 align-middle font-medium">{{ $t('admin.nodes.column.status') }}</th>
              <th class="px-3 align-middle font-medium">{{ $t('admin.nodes.column.cpuMem') }}</th>
              <th class="px-3 align-middle font-medium">{{ $t('admin.nodes.column.xray') }}</th>
              <th class="px-3 align-middle font-medium">{{ $t('admin.nodes.column.lastSeen') }}</th>
              <th class="px-3 text-center align-middle font-medium">{{ $t('admin.nodes.column.schedule') }}</th>
              <th class="px-3 text-right align-middle font-medium">{{ $t('admin.users.column.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
            <tr
              v-for="n in nodes"
              :key="n.id"
              class="h-[68px] transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40"
            >
              <td class="px-3 align-middle font-mono text-xs tabular-nums text-surface-400">#{{ n.id }}</td>
              <td class="px-3 align-middle">
                <div class="truncate font-medium text-ink-900 dark:text-surface-50" :title="n.name">{{ n.name }}</div>
              </td>
              <td class="px-3 align-middle">
                <div class="flex min-w-0 flex-col justify-center">
                  <div
                    class="block w-full truncate font-mono text-xs leading-5 text-surface-500"
                    :title="nodeConnectionURL(n)"
                  >
                    {{ nodeConnectionLabel(n) }}
                  </div>
                  <a
                    class="mt-1 inline-flex w-fit items-center gap-1 text-xs font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-300"
                    :href="panelInboundURL(n)"
                    target="_blank"
                    rel="noreferrer"
                  >
                    {{ $t('admin.nodes.openPanel') }}
                    <svg class="h-3 w-3 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 17 17 7" /><path d="M8 7h9v9" /></svg>
                  </a>
                </div>
              </td>
              <td class="px-3 align-middle">
                <span
                  class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset"
                  :class="nodeStatusBadgeClass(n)"
                >
                  <span class="h-1.5 w-1.5 rounded-full" :class="nodeStatusDotClass(n)" />
                  {{ nodeStatusText(n) }}
                </span>
              </td>
              <td class="px-3 align-middle">
                <span class="whitespace-nowrap tabular-nums text-surface-600 dark:text-surface-300">{{ n.cpu_pct.toFixed(1) }}% · {{ n.mem_pct.toFixed(1) }}%</span>
              </td>
              <td class="px-3 align-middle">
                <div class="truncate font-mono text-xs text-surface-500" :title="n.xray_version || '—'">{{ n.xray_version || '—' }}</div>
              </td>
              <td class="px-3 align-middle">
                <span class="block truncate text-xs text-surface-500" :title="n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : '—'">{{ n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : '—' }}</span>
              </td>
              <td class="px-3 text-center align-middle">
                <button
                  type="button"
                  role="switch"
                  :aria-checked="n.enabled"
                  :aria-label="n.enabled ? $t('admin.nodes.disable') : $t('admin.nodes.enable')"
                  :title="n.enabled ? $t('admin.nodes.disable') : $t('admin.nodes.enable')"
                  :disabled="togglingNodeId === n.id"
                  class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-accent-500/40 disabled:cursor-wait disabled:opacity-60"
                  :class="n.enabled ? 'bg-accent-500' : 'bg-surface-300 dark:bg-surface-700'"
                  @click="toggleEnable(n)"
                >
                  <span
                    class="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform"
                    :class="n.enabled ? 'translate-x-5' : 'translate-x-0'"
                  />
                </button>
              </td>
              <td class="px-3 text-right align-middle">
                <div class="ml-auto inline-flex items-center gap-1 rounded-full border border-surface-200 bg-surface-50 px-1.5 py-1 shadow-sm dark:border-surface-700/80 dark:bg-surface-950/70">
                  <button :title="$t('admin.nodes.probe')" class="flex h-7 w-7 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-surface-0 hover:text-accent-700 dark:hover:bg-surface-800 dark:hover:text-accent-300" @click="probe(n.id)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 7-6-14-3 7H2" /></svg>
                  </button>
                  <button :title="$t('admin.nodes.edit')" class="flex h-7 w-7 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-surface-0 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="openEdit(n)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
                  </button>
                  <button :title="$t('admin.nodes.delete')" class="flex h-7 w-7 items-center justify-center rounded-full text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="destroy(n)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- Add/Edit modal -->
    <div
      v-if="showEditor"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="closeEditor"
    >
      <div class="w-full max-w-xl animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-800">
          <div>
            <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ isEditing ? $t('admin.nodes.editTitle', { name: editingNode?.name }) : $t('admin.nodes.createTitle') }}</h2>
            <p class="mt-0.5 text-xs text-surface-500">{{ isEditing ? $t('admin.nodes.editHint') : $t('admin.nodes.createHint') }}</p>
          </div>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="closeEditor">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-5 px-6 py-5" @submit.prevent="submit">
          <div class="grid grid-cols-2 gap-3.5">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.name') }}</label>
              <input v-model="form.name" type="text" :placeholder="$t('admin.nodes.namePlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.scheme') }}</label>
              <select v-model="form.scheme" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900">
                <option value="https">https</option>
                <option value="http">http</option>
              </select>
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.host') }}</label>
              <input v-model="form.host" type="text" :placeholder="$t('admin.nodes.hostPlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.port') }}</label>
              <input v-model.number="form.port" type="number" min="1" max="65535" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.basePath') }}</label>
              <input v-model="form.base_path" type="text" :placeholder="$t('admin.nodes.basePathPlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.apiToken') }}</label>
              <input v-model="form.api_token" type="text" :placeholder="isEditing ? $t('admin.nodes.apiTokenEditPlaceholder') : $t('admin.nodes.apiTokenPlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-xs transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
              <p v-if="isEditing" class="mt-1.5 text-xs text-surface-500">{{ $t('admin.nodes.apiTokenKeepHint') }}</p>
            </div>
            <div class="col-span-2 flex items-center justify-between rounded-xl border border-surface-100 px-3 py-2.5 dark:border-surface-800">
              <label for="node-enable" class="text-sm text-surface-700 dark:text-surface-300">{{ $t('admin.nodes.enableDefaultLabel') }}</label>
              <button
                id="node-enable"
                type="button"
                role="switch"
                :aria-checked="form.enabled"
                class="relative inline-flex h-5 w-9 shrink-0 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-accent-500/40"
                :class="form.enabled ? 'bg-accent-500' : 'bg-surface-300 dark:bg-surface-700'"
                @click="form.enabled = !form.enabled"
              >
                <span
                  class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform"
                  :class="form.enabled ? 'translate-x-4' : 'translate-x-0'"
                />
              </button>
            </div>
          </div>
          <p v-if="formErr" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800">{{ formErr }}</p>
          <footer class="flex justify-end gap-2 border-t border-surface-100 -mx-6 -mb-5 px-6 py-4 dark:border-surface-800">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="closeEditor">{{ $t('common.cancel') }}</button>
            <button type="submit" :disabled="saving" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">
              {{ saving ? (isEditing ? $t('admin.nodes.updating') : $t('admin.nodes.creating')) : (isEditing ? $t('admin.nodes.save') : $t('admin.nodes.submit')) }}
            </button>
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
