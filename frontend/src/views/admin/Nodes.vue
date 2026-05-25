<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatError } from '@/utils/format'

import { nodesApi, type Node, type NodeInput } from '@/api/admin/nodes'
import { inboundsApi, type FleetInbound } from '@/api/admin/inbounds'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import DataPageShell from '@/components/common/DataPageShell.vue'
import { useConfirm } from '@/composables/useConfirm'

const { t } = useI18n()

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const nodes = ref<Node[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const togglingNodeId = ref<number | null>(null)
const query = ref('')
const areaFilter = ref<NodeAreaKey | ''>('')
const provinceFilter = ref('')
const schemeFilter = ref<'' | Node['scheme']>('')
const statusFilter = ref<'' | NodeDisplayStatus>('')
const selected = ref<Set<number>>(new Set())
const bulkBusy = ref(false)
const inbounds = ref<FleetInbound[]>([])
const importInput = ref<HTMLInputElement | null>(null)
const importing = ref(false)

const showEditor = ref(false)
const saving = ref(false)
const formErr = ref<string | null>(null)
const editingNode = ref<Node | null>(null)
const quickImportURL = ref('')

const isEditing = computed(() => editingNode.value !== null)

const AREA_OPTIONS = [
  { key: 'jp', code: 'JP', labelKey: 'admin.nodes.area.jp' },
  { key: 'sg', code: 'SG', labelKey: 'admin.nodes.area.sg' },
  { key: 'hk', code: 'HK', labelKey: 'admin.nodes.area.hk' },
  { key: 'tw', code: 'TW', labelKey: 'admin.nodes.area.tw' },
  { key: 'us', code: 'US', labelKey: 'admin.nodes.area.us' },
  { key: 'gb', code: 'UK', labelKey: 'admin.nodes.area.gb' },
  { key: 'de', code: 'DE', labelKey: 'admin.nodes.area.de' },
  { key: 'fr', code: 'FR', labelKey: 'admin.nodes.area.fr' },
  { key: 'nl', code: 'NL', labelKey: 'admin.nodes.area.nl' },
  { key: 'ca', code: 'CA', labelKey: 'admin.nodes.area.ca' },
  { key: 'au', code: 'AU', labelKey: 'admin.nodes.area.au' },
  { key: 'kr', code: 'KR', labelKey: 'admin.nodes.area.kr' },
  { key: 'in', code: 'IN', labelKey: 'admin.nodes.area.in' },
  { key: 'th', code: 'TH', labelKey: 'admin.nodes.area.th' },
  { key: 'vn', code: 'VN', labelKey: 'admin.nodes.area.vn' },
  { key: 'unknown', code: 'UN', labelKey: 'admin.nodes.area.unknown' },
] as const

type AreaOption = (typeof AREA_OPTIONS)[number]
type NodeAreaKey = AreaOption['key']

interface NodeArea {
  key: NodeAreaKey
  code: string
  label: string
}

function blankForm(): NodeInput {
  return {
    name: '',
    area: 'unknown',
    province: 'unknown',
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
  quickImportURL.value = ''
  formErr.value = null
  showEditor.value = true
}

function openEdit(n: Node) {
  editingNode.value = n
  form.value = {
    name: n.name,
    area: normalizeNodeArea(n.area),
    province: normalizeNodeProvince(n.province),
    scheme: n.scheme,
    host: n.host,
    port: n.port,
    base_path: n.base_path ?? '',
    api_token: '',
    enabled: n.enabled,
  }
  quickImportURL.value = ''
  formErr.value = null
  showEditor.value = true
}

function closeEditor() {
  showEditor.value = false
  editingNode.value = null
  form.value = blankForm()
  quickImportURL.value = ''
  formErr.value = null
}

function buildPayload(): NodeInput {
  const area = normalizeNodeArea(form.value.area)
  return {
    ...form.value,
    name: form.value.name.trim(),
    area,
    province: normalizeNodeProvince(form.value.province),
    host: form.value.host.trim(),
    base_path: form.value.base_path?.trim() ?? '',
    api_token: form.value.api_token?.trim() ?? '',
    port: Math.max(1, Math.min(65535, Math.round(form.value.port || 0))),
  }
}

async function loadInboundSummary() {
  try {
    const result = await inboundsApi.fleet()
    inbounds.value = result.inbounds ?? []
  } catch {
    inbounds.value = []
  }
}

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [nodeRows] = await Promise.all([
      nodesApi.list({
        query: query.value.trim() || undefined,
        area: areaFilter.value || undefined,
        province: provinceFilter.value.trim() || undefined,
        scheme: schemeFilter.value || undefined,
        status: statusFilter.value || undefined,
      }),
      loadInboundSummary(),
    ])
    nodes.value = nodeRows
    reconcileSelection()
  } catch (e: any) {
    error.value = formatError(e, t('admin.nodes.loadFailed'))
  } finally {
    loading.value = false
  }
}

let filterTimer: ReturnType<typeof setTimeout> | null = null

watch([query, areaFilter, provinceFilter, schemeFilter, statusFilter], () => {
  if (filterTimer) clearTimeout(filterTimer)
  filterTimer = setTimeout(() => {
    void reload()
  }, 250)
})

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

async function batchProbe() {
  const targets = selectedNodes.value.length > 0 ? selectedNodes.value : filteredNodes.value
  if (targets.length === 0) return
  bulkBusy.value = true
  try {
    const results = await Promise.allSettled(targets.map((n) => nodesApi.probe(n.id)))
    const failed = results.filter((r) => r.status === 'rejected').length
    if (failed > 0) error.value = t('admin.nodes.batch.probeResult', { ok: results.length - failed, fail: failed })
    clearSelection()
    await reload()
  } finally {
    bulkBusy.value = false
  }
}

async function batchDelete() {
  if (selected.value.size === 0) return
  const ok = await askConfirm({
    title: t('admin.nodes.batch.deleteTitle'),
    message: t('admin.nodes.batch.deleteMsg', { n: selected.value.size }),
    variant: 'danger',
    confirmLabel: t('admin.nodes.delete'),
  })
  if (!ok) return
  bulkBusy.value = true
  try {
    const ids = Array.from(selected.value)
    const results = await Promise.allSettled(ids.map((id) => nodesApi.remove(id)))
    const failed = results.filter((r) => r.status === 'rejected').length
    if (failed > 0) error.value = t('admin.nodes.batch.deleteResult', { ok: results.length - failed, fail: failed })
    clearSelection()
    await reload()
  } finally {
    bulkBusy.value = false
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

function normalizedPanelPath(basePath: string | undefined | null): string {
  const raw = (basePath ?? '').trim()
  if (!raw || raw === '/') return '/panel'
  const withLeading = raw.startsWith('/') ? raw : `/${raw}`
  return withLeading.endsWith('/') ? withLeading.slice(0, -1) : withLeading
}

function panelInboundURL(n: Node): string {
  return `${n.scheme}://${n.host}:${n.port}${normalizedPanelPath(n.base_path)}/inbounds`
}

function nodeConnectionURL(n: Node): string {
  return `${n.scheme}://${n.host}:${n.port}${normalizedPanelPath(n.base_path)}`
}

function compactHost(host: string): string {
  // Iterate by code point so multi-byte chars (CJK, IDN, emoji) stay
  // whole; string.slice() splits surrogate pairs.
  const chars = Array.from(host.trim())
  if (chars.length <= 24) return chars.join('')
  return `${chars.slice(0, 13).join('')}…${chars.slice(-8).join('')}`
}

function nodeConnectionLabel(n: Node): string {
  const pathLabel = n.base_path ? '/...' : ''
  return `${n.scheme}://${compactHost(n.host)}:${n.port}${pathLabel}`
}

function applyQuickImportURL(value = quickImportURL.value) {
  const parsed = parsePanelURL(value)
  if (!parsed) return
  form.value.scheme = parsed.scheme
  form.value.host = parsed.host
  form.value.port = parsed.port
  form.value.base_path = parsed.basePath
}

function parsePanelURL(value: string): { scheme: 'http' | 'https'; host: string; port: number; basePath: string } | null {
  const raw = value.trim()
  if (!raw) return null
  const normalized = raw.startsWith('//') ? `https:${raw}` : /^[a-z][a-z\d+.-]*:\/\//i.test(raw) ? raw : `https://${raw}`
  let url: URL
  try {
    url = new URL(normalized)
  } catch {
    return null
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
  const host = url.hostname.replace(/^\[(.*)]$/, '$1')
  if (!host) return null
  const scheme = url.protocol === 'http:' ? 'http' : 'https'
  const port = url.port ? Number(url.port) : scheme === 'http' ? 80 : 443
  if (!Number.isInteger(port) || port < 1 || port > 65535) return null
  return {
    scheme,
    host,
    port,
    basePath: panelPathFromURLPath(url.pathname),
  }
}

function panelPathFromURLPath(pathname: string): string {
  const segments = pathname.split('/').filter(Boolean)
  if (segments.length === 0) return '/panel/'
  const panelIndex = segments.findIndex((item) => item.toLowerCase() === 'panel')
  if (panelIndex >= 0) {
    return `/${segments.slice(0, panelIndex + 1).join('/')}/`
  }
  const apiIndex = segments.findIndex((item) => item.toLowerCase() === 'api')
  if (apiIndex > 0) {
    return `/${segments.slice(0, apiIndex).join('/')}/`
  }
  return `/${segments.join('/')}/`
}

function normalizeNodeArea(area?: string | null): NodeAreaKey {
  const raw = (area || '').trim().toLowerCase()
  if (raw === 'other') return 'unknown'
  return AREA_OPTIONS.some((item) => item.key === raw) ? (raw as NodeAreaKey) : 'unknown'
}

function normalizeNodeProvince(province?: string | null): string {
  const raw = (province || '').trim()
  return raw || 'unknown'
}

function nodeProvinceText(province?: string | null): string {
  const normalized = normalizeNodeProvince(province)
  return normalized.toLowerCase() === 'unknown' || normalized === '未知' ? t('admin.nodes.area.unknown') : normalized
}

function nodeArea(n: Node): NodeArea {
  const area = AREA_OPTIONS.find((item) => item.key === normalizeNodeArea(n.area))
  if (!area) {
    return {
      key: 'unknown',
      code: 'UN',
      label: t('admin.nodes.area.unknown'),
    }
  }
  return {
    key: area.key,
    code: area.code,
    label: t(area.labelKey),
  }
}

const areaOptions = computed<NodeArea[]>(() =>
  AREA_OPTIONS.map((area) => ({
    key: area.key,
    code: area.code,
    label: t(area.labelKey),
  })),
)

function nodeLocationLabel(n: Node): string {
  const area = nodeArea(n).label
  return `${area} / ${nodeProvinceText(n.province)}`
}

function nodeLocationText(n: Node): string {
  const area = nodeArea(n).label
  return `${area} · ${nodeProvinceText(n.province)}`
}

function nodeAreaFlag(n: Node): string {
  const area = nodeArea(n)
  if (area.key === 'unknown') return ''
  const flagCode = area.key === 'gb' ? 'GB' : area.key.toUpperCase()
  return Array.from(flagCode)
    .map((char) => String.fromCodePoint(127397 + char.charCodeAt(0)))
    .join('')
}

type NodeSortKey = 'name' | 'scheme' | 'clients' | 'status'
const sortKey = ref<NodeSortKey>('name')
const sortDir = ref<'asc' | 'desc'>('asc')

const filteredNodes = computed(() => {
  const rows = [...nodes.value]
  rows.sort((a, b) => {
    const dir = sortDir.value === 'asc' ? 1 : -1
    if (sortKey.value === 'clients') {
      return (nodeClientCount(a) - nodeClientCount(b)) * dir
    }
    if (sortKey.value === 'status') {
      return nodeStatusText(a).localeCompare(nodeStatusText(b)) * dir
    }
    return String(a[sortKey.value] ?? '').localeCompare(String(b[sortKey.value] ?? '')) * dir
  })
  return rows
})

const selectedCount = computed(() => selected.value.size)
const selectedNodes = computed(() => filteredNodes.value.filter((n) => selected.value.has(n.id)))
const allVisibleSelected = computed(() => filteredNodes.value.length > 0 && filteredNodes.value.every((n) => selected.value.has(n.id)))
const someVisibleSelected = computed(() => filteredNodes.value.some((n) => selected.value.has(n.id)) && !allVisibleSelected.value)
const onlineCount = computed(() => nodes.value.filter((n) => nodeDisplayStatus(n) === 'online').length)
const offlineCount = computed(() => nodes.value.filter((n) => nodeDisplayStatus(n) === 'offline').length)
const disabledCount = computed(() => nodes.value.filter((n) => nodeDisplayStatus(n) === 'disabled').length)

const hasNodeFilters = computed(() =>
  query.value.trim() !== '' ||
  areaFilter.value !== '' ||
  provinceFilter.value.trim() !== '' ||
  schemeFilter.value !== '' ||
  statusFilter.value !== '',
)

function sortBy(key: NodeSortKey) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}

function sortIndicator(key: NodeSortKey): string {
  if (sortKey.value !== key) return ''
  return sortDir.value === 'asc' ? '↑' : '↓'
}

function toggleAllVisible(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  if (checked) filteredNodes.value.forEach((n) => selected.value.add(n.id))
  else filteredNodes.value.forEach((n) => selected.value.delete(n.id))
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

function reconcileSelection() {
  const visibleIDs = new Set(nodes.value.map((n) => n.id))
  selected.value = new Set(Array.from(selected.value).filter((id) => visibleIDs.has(id)))
}

function nodeInboundCount(n: Node): number {
  return inbounds.value.filter((item) => item.node_id === n.id).length
}

function nodeClientCount(n: Node): number {
  return inbounds.value
    .filter((item) => item.node_id === n.id)
    .reduce((sum, item) => sum + (item.inbound.clientStats?.length ?? 0), 0)
}

function nodeAuthLabel(): string {
  return t('admin.nodes.authBearer')
}

function nodeLatencyLabel(n: Node): string {
  if (!n.enabled) return '—'
  return n.last_seen_at ? t('admin.nodes.latencyUnknown') : '—'
}

function protocolClass(n: Node): string {
  return n.scheme === 'https'
    ? 'border-accent-500/30 bg-accent-500/10 text-accent-700 dark:text-accent-300'
    : 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300'
}

function exportJson() {
  const rows = filteredNodes.value.map((n) => ({
    name: n.name,
    area: normalizeNodeArea(n.area),
    province: normalizeNodeProvince(n.province),
    scheme: n.scheme,
    host: n.host,
    port: n.port,
    base_path: n.base_path,
    api_token: '',
    enabled: n.enabled,
  }))
  const blob = new Blob([JSON.stringify({ nodes: rows }, null, 2)], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  a.href = url
  a.download = `nodes-${ts}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function openImport() {
  importInput.value?.click()
}

async function importJson(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  importing.value = true
  try {
    const raw = await file.text()
    const parsed = JSON.parse(raw) as { nodes?: NodeInput[] } | NodeInput[]
    const rows = Array.isArray(parsed) ? parsed : parsed.nodes
    if (!Array.isArray(rows)) {
      error.value = t('admin.nodes.importInvalid')
      return
    }
    const results = await Promise.allSettled(rows.map((row) => nodesApi.create({
      name: String(row.name ?? ''),
      area: normalizeNodeArea(String(row.area || '')),
      province: normalizeNodeProvince(String(row.province ?? '')),
      scheme: row.scheme === 'http' ? 'http' : 'https',
      host: String(row.host ?? ''),
      port: Number(row.port ?? 2053),
      base_path: String(row.base_path ?? ''),
      api_token: String(row.api_token ?? ''),
      enabled: Boolean(row.enabled ?? true),
    })))
    const failed = results.filter((r) => r.status === 'rejected').length
    if (failed > 0) error.value = t('admin.nodes.importResult', { ok: results.length - failed, fail: failed })
    await reload()
  } catch (err) {
    error.value = t('admin.nodes.importInvalid')
  } finally {
    importing.value = false
  }
}

function clearFilters() {
  query.value = ''
  areaFilter.value = ''
  provinceFilter.value = ''
  schemeFilter.value = ''
  statusFilter.value = ''
}

onMounted(reload)
onUnmounted(() => {
  if (filterTimer) clearTimeout(filterTimer)
})
</script>

<template>
  <div>
    <DataPageShell body-class="flex flex-col">
      <template #toolbar>
        <input ref="importInput" class="hidden" type="file" accept="application/json,.json" @change="importJson" />

        <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <div class="flex min-w-0 flex-1 flex-col gap-2 lg:flex-row lg:items-center">
            <div class="relative min-w-0 flex-1 xl:max-w-lg">
              <svg class="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
              <input
                v-model="query"
                type="text"
                :placeholder="$t('admin.nodes.searchPlaceholder')"
                class="h-10 w-full rounded-xl border border-surface-200 bg-surface-50/80 pl-9 pr-3 text-sm text-ink-900 transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:bg-surface-0 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-white/10 dark:bg-ink-800/90 dark:text-surface-50"
              />
            </div>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:flex lg:items-center">
              <div class="relative">
                <select v-model="areaFilter" :aria-label="$t('admin.nodes.filterArea')" class="h-10 w-full appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-white/10 dark:bg-ink-800/90 dark:text-surface-100 lg:w-40" :disabled="areaOptions.length === 0">
                  <option value="">{{ $t('admin.nodes.filterAreaAll') }}</option>
                  <option v-for="area in areaOptions" :key="area.key" :value="area.key">{{ area.label }}</option>
                </select>
                <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
              </div>
              <div class="relative">
                <input
                  v-model="provinceFilter"
                  type="text"
                  :aria-label="$t('admin.nodes.filterProvince')"
                  :placeholder="$t('admin.nodes.filterProvincePlaceholder')"
                  class="h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm text-surface-700 transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-white/10 dark:bg-ink-800/90 dark:text-surface-100 lg:w-36"
                />
              </div>
              <div class="relative">
                <select v-model="schemeFilter" :aria-label="$t('admin.nodes.filterProtocol')" class="h-10 w-full appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-white/10 dark:bg-ink-800/90 dark:text-surface-100 lg:w-36">
                  <option value="">{{ $t('admin.nodes.filterProtocolAll') }}</option>
                  <option value="https">HTTPS</option>
                  <option value="http">HTTP</option>
                </select>
                <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
              </div>
              <div class="relative">
                <select v-model="statusFilter" :aria-label="$t('admin.nodes.filterStatus')" class="h-10 w-full appearance-none rounded-xl border border-surface-200 bg-surface-0 pl-3 pr-8 text-sm text-surface-700 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-white/10 dark:bg-ink-800/90 dark:text-surface-100 lg:w-36">
                  <option value="">{{ $t('admin.nodes.filterStatusAll') }}</option>
                  <option value="online">{{ $t('admin.nodes.status.online') }}</option>
                  <option value="offline">{{ $t('admin.nodes.status.offline') }}</option>
                  <option value="unknown">{{ $t('admin.nodes.status.unknown') }}</option>
                  <option value="disabled">{{ $t('admin.status.nodeState.disabled') }}</option>
                </select>
                <svg class="pointer-events-none absolute right-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6" /></svg>
              </div>
            </div>
          </div>

          <div class="flex shrink-0 flex-wrap items-center gap-2">
            <button class="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] disabled:cursor-wait disabled:opacity-60 dark:border-white/10 dark:bg-white/5 dark:text-surface-200 dark:hover:bg-white/10 dark:hover:text-white" :title="$t('admin.nodes.reload')" :disabled="loading" @click="reload">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
            </button>
            <button type="button" class="inline-flex h-10 items-center gap-1.5 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm font-medium text-surface-700 transition-all hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] disabled:cursor-wait disabled:opacity-60 dark:border-white/10 dark:bg-white/5 dark:text-surface-100 dark:hover:bg-white/10 dark:hover:text-white" :disabled="bulkBusy || filteredNodes.length === 0" @click="batchProbe">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3" /></svg>
              {{ $t('admin.nodes.batch.testConnection') }}
            </button>
            <button type="button" class="inline-flex h-10 items-center gap-1.5 rounded-xl border border-primary-200 bg-primary-50 px-3 text-sm font-medium text-primary-700 transition-all hover:bg-primary-100 active:scale-[0.98] disabled:cursor-wait disabled:opacity-60 dark:border-primary-400/25 dark:bg-primary-500/15 dark:text-primary-100 dark:hover:bg-primary-500/25" :disabled="bulkBusy || filteredNodes.length === 0" @click="batchProbe">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z" /><path d="m9 12 2 2 4-5" /></svg>
              {{ $t('admin.nodes.batch.qualityCheck') }}
            </button>
            <button type="button" class="inline-flex h-10 w-10 items-center justify-center rounded-xl border border-red-200 bg-red-50 text-red-600 transition-all hover:bg-red-100 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45 dark:border-red-400/25 dark:bg-red-500/15 dark:text-red-200 dark:hover:bg-red-500/25" :title="$t('admin.nodes.batch.delete')" :disabled="bulkBusy || selectedCount === 0" @click="batchDelete">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
            </button>
            <button type="button" class="inline-flex h-10 items-center gap-1.5 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm font-medium text-surface-700 transition-all hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] disabled:cursor-wait disabled:opacity-60 dark:border-white/10 dark:bg-white/5 dark:text-surface-100 dark:hover:bg-white/10 dark:hover:text-white" :disabled="importing" @click="openImport">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12" /><path d="m7 8 5-5 5 5" /><path d="M5 21h14" /></svg>
              {{ $t('admin.nodes.import') }}
            </button>
            <button type="button" class="inline-flex h-10 items-center gap-1.5 rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm font-medium text-surface-700 transition-all hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-white/10 dark:bg-white/5 dark:text-surface-100 dark:hover:bg-white/10 dark:hover:text-white" @click="exportJson">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12" /><path d="m7 10 5 5 5-5" /><path d="M5 21h14" /></svg>
              {{ $t('admin.nodes.export') }}
            </button>
            <button class="inline-flex h-10 items-center gap-1.5 rounded-xl bg-accent-500 px-3.5 text-sm font-semibold text-ink-950 shadow-card transition-all hover:bg-accent-400 active:scale-[0.98]" @click="openCreate">
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14" /></svg>
              {{ $t('admin.nodes.addNode') }}
            </button>
          </div>
        </div>
      </template>

      <div v-if="error" class="m-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</div>

      <div v-if="loading" class="p-4">
        <Skeleton :rows="6" />
      </div>

      <template v-else>
        <div
          v-if="filteredNodes.length > 0"
          class="grid gap-3 overflow-y-auto p-3 xl:hidden"
        >
        <article
          v-for="n in filteredNodes"
          :key="n.id"
          class="rounded-2xl border border-surface-100 bg-surface-0 p-4 shadow-card transition-colors dark:border-surface-800 dark:bg-surface-900"
          :class="selected.has(n.id) ? 'border-accent-500/40 bg-accent-50/40 dark:border-accent-500/40 dark:bg-accent-500/10' : ''"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 items-start gap-3">
              <input
                type="checkbox"
                class="mt-1 h-4 w-4 shrink-0 cursor-pointer rounded border-surface-300 text-accent-600 focus:ring-accent-500 focus:ring-offset-0 dark:border-surface-600 dark:bg-surface-800"
                :checked="selected.has(n.id)"
                :aria-label="$t('admin.nodes.batch.toggleRow', { name: n.name })"
                @change="(e) => toggleOne(n.id, e)"
              />
              <div class="min-w-0">
                <div class="font-mono text-2xs text-surface-400">#{{ n.id }}</div>
                <h2 class="mt-1 break-words text-base font-semibold leading-6 text-ink-900 dark:text-surface-50">{{ n.name }}</h2>
              </div>
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
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.location') }}</dt>
              <dd class="mt-1">
                <span
                  class="inline-flex max-w-full items-center gap-1.5 rounded-lg border border-surface-200 bg-surface-0 px-2 py-1 text-xs font-medium text-surface-700 dark:border-white/10 dark:bg-white/5 dark:text-surface-200"
                  :title="nodeLocationLabel(n)"
                >
                  <span v-if="nodeAreaFlag(n)" class="text-sm leading-none">{{ nodeAreaFlag(n) }}</span>
                  <span v-else class="h-2 w-2 rounded-full bg-surface-300 dark:bg-surface-600" />
                  <span class="truncate">{{ nodeLocationText(n) }}</span>
                </span>
              </dd>
            </div>
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium text-surface-400">{{ $t('admin.nodes.column.protocol') }}</dt>
              <dd class="mt-1">
                <span class="inline-flex rounded-lg border px-2 py-0.5 font-mono text-xs font-semibold uppercase" :class="protocolClass(n)">{{ n.scheme }}</span>
              </dd>
            </div>
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.cpuMem') }}</dt>
              <dd class="mt-1 tabular-nums text-surface-700 dark:text-surface-200">{{ n.cpu_pct.toFixed(1) }}% · {{ n.mem_pct.toFixed(1) }}%</dd>
            </div>
            <div class="rounded-xl border border-surface-100 px-3 py-2 dark:border-surface-800">
              <dt class="text-2xs font-medium uppercase tracking-wider text-surface-400">{{ $t('admin.nodes.column.clients') }}</dt>
              <dd class="mt-1 tabular-nums text-surface-700 dark:text-surface-200">{{ nodeClientCount(n) }} · {{ nodeInboundCount(n) }}</dd>
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
        v-else-if="!hasNodeFilters"
        class="flex min-h-[320px] items-center justify-center"
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
        v-else
        class="flex min-h-[320px] items-center justify-center"
      >
        <EmptyState
          icon="M3 6h18M4 12h16M6 18h12"
          :title="$t('admin.nodes.noMatchTitle')"
          :description="$t('admin.nodes.noMatchDescription')"
        />
      </div>

      <div
        v-if="filteredNodes.length > 0"
        class="hidden min-h-0 flex-1 overflow-auto xl:block"
      >
        <table class="w-full min-w-[1080px] table-fixed text-sm">
          <colgroup>
            <col class="w-[42px]" />
            <col class="w-[18%]" />
            <col class="w-[8%]" />
            <col class="w-[25%]" />
            <col class="w-[8%]" />
            <col class="w-[13%]" />
            <col class="w-[8%]" />
            <col class="w-[7%]" />
            <col class="w-[9%]" />
            <col class="w-[7%]" />
          </colgroup>
          <thead class="bg-surface-50 text-left text-xs font-semibold text-surface-500 dark:bg-ink-800 dark:text-surface-300">
            <tr class="h-12 border-b border-surface-200 dark:border-white/10">
              <th class="px-3 align-middle">
                <input
                  type="checkbox"
                  class="h-4 w-4 cursor-pointer rounded border-surface-300 bg-surface-0 text-accent-600 focus:ring-accent-500 focus:ring-offset-0 dark:border-surface-500 dark:bg-ink-900 dark:text-accent-500"
                  :aria-label="$t('admin.nodes.batch.toggleAll')"
                  :checked="allVisibleSelected"
                  :ref="(el) => { if (el) (el as HTMLInputElement).indeterminate = someVisibleSelected }"
                  @change="toggleAllVisible"
                />
              </th>
              <th class="px-3 align-middle">
                <button type="button" class="inline-flex items-center gap-1 transition-colors hover:text-ink-900 dark:hover:text-white" @click="sortBy('name')">
                  {{ $t('admin.nodes.column.name') }}
                  <span class="text-accent-600 dark:text-accent-300">{{ sortIndicator('name') }}</span>
                </button>
              </th>
              <th class="px-3 align-middle">
                <button type="button" class="inline-flex items-center gap-1 transition-colors hover:text-ink-900 dark:hover:text-white" @click="sortBy('scheme')">
                  {{ $t('admin.nodes.column.protocol') }}
                  <span class="text-accent-600 dark:text-accent-300">{{ sortIndicator('scheme') }}</span>
                </button>
              </th>
              <th class="px-3 align-middle">{{ $t('admin.nodes.column.address') }}</th>
              <th class="px-3 align-middle">{{ $t('admin.nodes.column.auth') }}</th>
              <th class="px-3 align-middle">
                <span class="inline-flex rounded-lg bg-accent-50 px-2 py-1 text-accent-700 ring-1 ring-inset ring-accent-100 dark:bg-accent-500/15 dark:text-accent-200 dark:ring-accent-400/20">{{ $t('admin.nodes.column.location') }}</span>
              </th>
              <th class="px-3 align-middle">
                <button type="button" class="inline-flex items-center gap-1 transition-colors hover:text-ink-900 dark:hover:text-white" @click="sortBy('clients')">
                  {{ $t('admin.nodes.column.clients') }}
                  <span class="text-accent-600 dark:text-accent-300">{{ sortIndicator('clients') }}</span>
                </button>
              </th>
              <th class="px-3 align-middle">{{ $t('admin.nodes.column.latency') }}</th>
              <th class="px-3 align-middle">
                <button type="button" class="inline-flex items-center gap-1 transition-colors hover:text-ink-900 dark:hover:text-white" @click="sortBy('status')">
                  {{ $t('admin.nodes.column.status') }}
                  <span class="text-accent-600 dark:text-accent-300">{{ sortIndicator('status') }}</span>
                </button>
              </th>
              <th class="px-3 text-right align-middle">{{ $t('admin.users.column.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-100 dark:divide-white/10">
            <tr
              v-for="n in filteredNodes"
              :key="n.id"
              class="group/row h-[72px] text-surface-700 transition-colors hover:bg-surface-50/70 dark:text-surface-200 dark:hover:bg-white/[0.04]"
              :class="selected.has(n.id) ? 'bg-accent-50 dark:bg-accent-500/[0.08]' : ''"
            >
              <td class="px-3 align-middle">
                <input
                  type="checkbox"
                  class="h-4 w-4 cursor-pointer rounded border-surface-300 bg-surface-0 text-accent-600 focus:ring-accent-500 focus:ring-offset-0 dark:border-surface-500 dark:bg-ink-900 dark:text-accent-500"
                  :checked="selected.has(n.id)"
                  :aria-label="$t('admin.nodes.batch.toggleRow', { name: n.name })"
                  @change="(e) => toggleOne(n.id, e)"
                />
              </td>
              <td class="px-3 align-middle">
                <div class="min-w-0">
                  <div class="truncate font-semibold text-ink-900 dark:text-surface-50" :title="n.name">{{ n.name }}</div>
                  <div class="mt-0.5 flex min-w-0 items-center gap-2 font-mono text-2xs text-surface-400 dark:text-surface-500">
                    <span>#{{ n.id }}</span>
                    <span class="truncate">{{ n.xray_version || '—' }}</span>
                  </div>
                </div>
              </td>
              <td class="px-3 align-middle">
                <span class="inline-flex rounded-lg border px-2 py-1 font-mono text-xs font-semibold uppercase" :class="protocolClass(n)">{{ n.scheme }}</span>
              </td>
              <td class="px-3 align-middle">
                <div class="flex min-w-0 flex-col justify-center">
                  <div class="block w-full truncate font-mono text-xs leading-5 text-surface-600 dark:text-surface-200" :title="nodeConnectionURL(n)">
                    {{ nodeConnectionLabel(n) }}
                  </div>
                  <a
                    class="mt-1 inline-flex w-fit items-center gap-1 text-xs font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-300 dark:hover:text-accent-200"
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
                <span class="inline-flex rounded-lg border border-surface-200 bg-surface-50 px-2 py-1 font-mono text-2xs text-surface-600 dark:border-white/10 dark:bg-white/5 dark:text-surface-300">{{ nodeAuthLabel() }}</span>
              </td>
              <td class="px-3 align-middle">
                <span
                  class="inline-flex max-w-full items-center gap-2 rounded-lg border border-surface-200 bg-surface-0 px-2 py-1 text-xs font-medium text-surface-700 dark:border-white/10 dark:bg-white/5 dark:text-surface-200"
                  :title="nodeLocationLabel(n)"
                >
                  <span v-if="nodeAreaFlag(n)" class="text-sm leading-none">{{ nodeAreaFlag(n) }}</span>
                  <span v-else class="h-2 w-2 rounded-full bg-surface-300 dark:bg-surface-600" />
                  <span class="truncate">{{ nodeLocationText(n) }}</span>
                </span>
              </td>
              <td class="px-3 align-middle">
                <div class="tabular-nums text-ink-900 dark:text-surface-50">{{ nodeClientCount(n) }}</div>
                <div class="text-2xs text-surface-400 dark:text-surface-500">{{ $t('admin.nodes.inboundCount', { n: nodeInboundCount(n) }) }}</div>
              </td>
              <td class="px-3 align-middle">
                <span class="font-mono text-xs text-surface-500 dark:text-surface-300">{{ nodeLatencyLabel(n) }}</span>
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
              <td class="px-3 text-right align-middle">
                <div class="ml-auto inline-flex items-center gap-0.5 rounded-xl border border-surface-200 bg-surface-50 px-1 py-1 dark:border-white/10 dark:bg-white/5">
                  <button :title="$t('admin.nodes.probe')" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-0 hover:text-accent-700 dark:text-surface-400 dark:hover:bg-white/10 dark:hover:text-accent-200" @click="probe(n.id)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 7-6-14-3 7H2" /></svg>
                  </button>
                  <button :title="$t('admin.nodes.edit')" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-0 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-white/10 dark:hover:text-white" @click="openEdit(n)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
                  </button>
                  <button :title="$t('admin.nodes.delete')" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:text-surface-400 dark:hover:bg-red-500/15 dark:hover:text-red-300" @click="destroy(n)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      </template>

      <template #footer>
        <div class="flex flex-col gap-3 text-sm text-surface-600 sm:flex-row sm:items-center sm:justify-between dark:text-surface-300">
          <div class="flex flex-wrap items-center gap-3">
            <span>{{ $t('admin.nodes.resultRange', { shown: filteredNodes.length, total: nodes.length }) }}</span>
            <span class="inline-flex items-center gap-1.5 rounded-lg bg-accent-50 px-2 py-1 text-xs font-medium text-accent-700 dark:bg-accent-500/15 dark:text-accent-200">
              <span class="h-1.5 w-1.5 rounded-full bg-accent-500" />
              {{ $t('admin.nodes.footerOnline', { n: onlineCount }) }}
            </span>
            <span class="inline-flex items-center gap-1.5 rounded-lg bg-red-50 px-2 py-1 text-xs font-medium text-red-600 dark:bg-red-500/15 dark:text-red-300">
              <span class="h-1.5 w-1.5 rounded-full bg-red-500" />
              {{ $t('admin.nodes.footerOffline', { n: offlineCount }) }}
            </span>
            <span class="inline-flex items-center gap-1.5 rounded-lg bg-surface-100 px-2 py-1 text-xs font-medium text-surface-600 dark:bg-surface-800 dark:text-surface-300">
              {{ $t('admin.nodes.footerDisabled', { n: disabledCount }) }}
            </span>
          </div>
          <div class="flex items-center gap-2">
            <span v-if="selectedCount > 0" class="text-xs font-medium text-accent-700 dark:text-accent-300">{{ $t('admin.nodes.batch.selectedCount', { n: selectedCount }) }}</span>
            <button v-if="selectedCount > 0" type="button" class="rounded-lg px-2 py-1 text-xs font-medium text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-white" @click="clearSelection">{{ $t('admin.nodes.batch.clear') }}</button>
            <button v-if="hasNodeFilters" type="button" class="rounded-lg px-2 py-1 text-xs font-medium text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-white" @click="clearFilters">{{ $t('admin.nodes.clearFilters') }}</button>
          </div>
        </div>
      </template>
    </DataPageShell>

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
            <div class="col-span-2 rounded-xl border border-accent-100 bg-accent-50/55 p-3 dark:border-accent-500/20 dark:bg-accent-500/10">
              <label class="mb-1.5 block text-xs font-medium text-accent-800 dark:text-accent-100">{{ $t('admin.nodes.quickImportLabel') }}</label>
              <input
                v-model="quickImportURL"
                type="text"
                :placeholder="$t('admin.nodes.quickImportPlaceholder')"
                class="block w-full rounded-xl border border-accent-200 bg-surface-0 px-3 py-2 font-mono text-xs text-ink-900 transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-accent-500/30 dark:bg-surface-900 dark:text-surface-50"
                @input="applyQuickImportURL()"
                @change="applyQuickImportURL()"
              />
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.name') }}</label>
              <input v-model="form.name" type="text" :placeholder="$t('admin.nodes.namePlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.areaLabel') }}</label>
              <select v-model="form.area" required class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900">
                <option v-for="area in areaOptions" :key="area.key" :value="area.key">{{ area.label }}</option>
              </select>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.nodes.provinceLabel') }}</label>
              <input v-model="form.province" type="text" :placeholder="$t('admin.nodes.provincePlaceholder')" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900" />
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
