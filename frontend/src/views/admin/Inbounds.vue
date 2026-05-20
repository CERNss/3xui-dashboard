<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { formatError } from '@/utils/format'
import QRCode from 'qrcode'

import {
  clientsApi,
  inboundsApi,
  parseTransport,
  trafficApi,
  type Client,
  type FleetInbound,
  type FleetResult,
  type Inbound,
  type NodeSnapshot,
} from '@/api/admin/inbounds'
import { nodesApi, type Node } from '@/api/admin/nodes'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import InboundEditorModal from './InboundEditorModal.vue'

// ---- state -----------------------------------------------------------------

const data = ref<FleetResult>({ inbounds: [] })
const nodes = ref<Node[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const query = ref('')
const protocolFilter = ref<'all' | 'vless' | 'vmess' | 'trojan' | 'shadowsocks' | 'wireguard'>('all')
const expanded = ref<Set<string>>(new Set()) // "nodeID|tag"
const snapshots = ref<Record<number, NodeSnapshot>>({}) // by node id

// Inbound add/edit modal — full 3x-ui-grade editor lives in
// InboundEditorModal.vue. Parent only tracks the open/mode/context.
const inboundModal = ref<{
  open: boolean
  mode: 'create' | 'edit'
  nodeID: number | null
  tag: string
  source: Inbound | null
}>({ open: false, mode: 'create', nodeID: null, tag: '', source: null })

// Client add/edit modal
const clientModal = ref<{
  open: boolean
  mode: 'create' | 'edit'
  row: FleetInbound | null
  client: Client
  origEmail: string // for edit
  busy: boolean
  err: string | null
}>({
  open: false,
  mode: 'create',
  row: null,
  client: blankClient(),
  origEmail: '',
  busy: false,
  err: null,
})

// QR modal
const qrModal = ref<{
  open: boolean
  title: string
  url: string
  dataURL: string
}>({ open: false, title: '', url: '', dataURL: '' })

// Confirm dialog
const confirmDialog = ref<{
  open: boolean
  title: string
  message: string
  onConfirm: () => Promise<void> | void
  busy: boolean
}>({
  open: false,
  title: '',
  message: '',
  onConfirm: () => {},
  busy: false,
})

// Toast
const toast = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
let toastTimer: number | undefined

function flash(kind: 'ok' | 'err', text: string) {
  toast.value = { kind, text }
  clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toast.value = null), 3500)
}

// ---- helpers ---------------------------------------------------------------

function blankClient(): Client {
  return {
    email: '',
    enable: true,
    id: crypto.randomUUID(),
    password: '',
    flow: '',
    limitIp: 0,
    totalGB: 0,
    expiryTime: 0,
    subId: '',
    comment: '',
  }
}

function formatBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = Math.abs(n)
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return (i === 0 ? v.toFixed(0) : v.toFixed(2)) + ' ' + units[i]
}

function formatLimit(n: number): string {
  return n === 0 ? '∞' : formatBytes(n)
}

function formatExpiry(ms: number): string {
  if (!ms || ms === 0) return '∞'
  const d = new Date(ms)
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString()
}

function rowKey(f: FleetInbound): string {
  return `${f.node_id}|${f.inbound.tag}`
}

// ---- data load -------------------------------------------------------------

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [d, n] = await Promise.all([inboundsApi.fleet(), nodesApi.list()])
    data.value = d
    nodes.value = n
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

async function loadSnapshot(nodeID: number) {
  try {
    snapshots.value[nodeID] = await clientsApi.snapshot(nodeID)
  } catch (e: any) {
    flash('err', formatError(e, 'snapshot 加载失败'))
  }
}

async function toggleExpand(f: FleetInbound) {
  const k = rowKey(f)
  if (expanded.value.has(k)) {
    expanded.value.delete(k)
  } else {
    expanded.value.add(k)
    if (!snapshots.value[f.node_id]) {
      await loadSnapshot(f.node_id)
    }
  }
}

// ---- inbound actions -------------------------------------------------------

async function toggleEnable(f: FleetInbound) {
  try {
    await inboundsApi.setEnable(f.node_id, f.inbound.tag, !f.inbound.enable)
    await reload()
  } catch (e: any) {
    flash('err', formatError(e, '切换失败'))
  }
}

function openCreateInbound() {
  inboundModal.value = {
    open: true,
    mode: 'create',
    nodeID: nodes.value.find((n) => n.enabled)?.id ?? null,
    tag: '',
    source: null,
  }
}

function openEditInbound(f: FleetInbound) {
  inboundModal.value = {
    open: true,
    mode: 'edit',
    nodeID: f.node_id,
    tag: f.inbound.tag,
    source: f.inbound,
  }
}

async function onInboundSaved(created: Inbound) {
  flash('ok', inboundModal.value.mode === 'create' ? `入站已创建 (port ${created.port})` : `入站已更新 (${created.tag})`)
  await reload()
}

// (no legacy submit — InboundEditorModal handles persistence itself)

function confirmDeleteInbound(f: FleetInbound) {
  confirmDialog.value = {
    open: true,
    title: '删除入站',
    message: `确认删除 ${f.node_name} 上的入站 "${f.inbound.tag}"（端口 ${f.inbound.port}）？\n此操作会同时删除入站下的所有客户端。`,
    busy: false,
    async onConfirm() {
      confirmDialog.value.busy = true
      try {
        await inboundsApi.remove(f.node_id, f.inbound.tag)
        flash('ok', `已删除 ${f.inbound.tag}`)
        confirmDialog.value.open = false
        await reload()
      } catch (e: any) {
        flash('err', formatError(e, '删除失败'))
      } finally {
        confirmDialog.value.busy = false
      }
    },
  }
}

function confirmResetInboundTraffic(f: FleetInbound) {
  confirmDialog.value = {
    open: true,
    title: '重置入站流量',
    message: `将 "${f.inbound.tag}" 上所有客户端的 up/down 计数清零（仅累计计数，不影响 inbound 配置和到期）。`,
    busy: false,
    async onConfirm() {
      confirmDialog.value.busy = true
      try {
        await trafficApi.resetInbound(f.node_id, f.inbound.tag)
        flash('ok', `已重置入站 ${f.inbound.tag} 流量`)
        confirmDialog.value.open = false
        await reload()
        if (snapshots.value[f.node_id]) await loadSnapshot(f.node_id)
      } catch (e: any) {
        flash('err', formatError(e, '重置失败'))
      } finally {
        confirmDialog.value.busy = false
      }
    },
  }
}

// ---- client actions --------------------------------------------------------

// helper: pull clients[] from an inbound's stringified settings
function parseClients(in_: Inbound): Client[] {
  try {
    const s = JSON.parse(in_.settings)
    return Array.isArray(s.clients) ? s.clients : []
  } catch {
    return []
  }
}

function isOnline(nodeID: number, email: string): boolean {
  return snapshots.value[nodeID]?.OnlineEmails?.includes(email) ?? false
}

function lastOnlineAt(nodeID: number, email: string): number | null {
  return snapshots.value[nodeID]?.LastOnlineByEmail?.[email] ?? null
}

function clientStatsByEmail(in_: Inbound, email: string) {
  return (in_.clientStats || []).find((c) => c.email === email)
}

function openAddClient(f: FleetInbound) {
  const c = blankClient()
  if (f.inbound.protocol === 'vless' || f.inbound.protocol === 'vmess') {
    c.id = crypto.randomUUID()
  } else {
    c.password = randomHex(16)
  }
  c.subId = randomHex(8)
  clientModal.value = {
    open: true,
    mode: 'create',
    row: f,
    client: c,
    origEmail: '',
    busy: false,
    err: null,
  }
}

function openEditClient(f: FleetInbound, c: Client) {
  clientModal.value = {
    open: true,
    mode: 'edit',
    row: f,
    client: { ...c },
    origEmail: c.email,
    busy: false,
    err: null,
  }
}

async function submitClient() {
  const m = clientModal.value
  m.err = null
  if (!m.row) return
  if (!m.client.email.trim()) {
    m.err = 'email 必填'
    return
  }
  if (m.row.inbound.protocol === 'trojan' || m.row.inbound.protocol === 'shadowsocks') {
    if (!m.client.password) {
      m.err = 'password 必填 (Trojan/SS)'
      return
    }
  } else {
    if (!m.client.id) {
      m.err = 'UUID 必填 (VLESS/VMess)'
      return
    }
  }
  m.busy = true
  try {
    if (m.mode === 'create') {
      await clientsApi.add(m.row.node_id, m.row.inbound.tag, m.client)
      flash('ok', `客户端已添加：${m.client.email}`)
    } else {
      await clientsApi.update(m.row.node_id, m.row.inbound.tag, m.origEmail, m.client)
      flash('ok', `客户端已更新：${m.client.email}`)
    }
    m.open = false
    await reload()
    await loadSnapshot(m.row.node_id)
  } catch (e: any) {
    m.err = formatError(e, '操作失败')
  } finally {
    m.busy = false
  }
}

function confirmDeleteClient(f: FleetInbound, c: Client) {
  confirmDialog.value = {
    open: true,
    title: '删除客户端',
    message: `确认在 ${f.inbound.tag} 上删除客户端 "${c.email}"？`,
    busy: false,
    async onConfirm() {
      confirmDialog.value.busy = true
      try {
        await clientsApi.remove(f.node_id, f.inbound.tag, c.email)
        flash('ok', `已删除 ${c.email}`)
        confirmDialog.value.open = false
        await reload()
        await loadSnapshot(f.node_id)
      } catch (e: any) {
        flash('err', formatError(e, '删除失败'))
      } finally {
        confirmDialog.value.busy = false
      }
    },
  }
}

function confirmResetClientTraffic(f: FleetInbound, c: Client) {
  confirmDialog.value = {
    open: true,
    title: '重置客户端流量',
    message: `将客户端 "${c.email}" 在 ${f.inbound.tag} 的 up/down 计数清零。`,
    busy: false,
    async onConfirm() {
      confirmDialog.value.busy = true
      try {
        await trafficApi.resetClient(f.node_id, f.inbound.tag, c.email)
        flash('ok', `已重置 ${c.email} 流量`)
        confirmDialog.value.open = false
        await reload()
        await loadSnapshot(f.node_id)
      } catch (e: any) {
        flash('err', formatError(e, '重置失败'))
      } finally {
        confirmDialog.value.busy = false
      }
    },
  }
}

// ---- link / QR -------------------------------------------------------------

function nodeOf(f: FleetInbound): Node | undefined {
  return nodes.value.find((n) => n.id === f.node_id)
}

function buildClientLink(f: FleetInbound, c: Client): string {
  const inb = f.inbound
  const node = nodeOf(f)
  const host = node?.host ?? '127.0.0.1'
  const port = inb.port
  const ss = (() => {
    try {
      return JSON.parse(inb.streamSettings)
    } catch {
      return {} as any
    }
  })()
  const network = ss.network || 'tcp'
  const security = ss.security || 'none'
  const remark = `${inb.remark || inb.tag} - ${c.email}`
  const enc = encodeURIComponent

  if (inb.protocol === 'vless') {
    const q = new URLSearchParams()
    q.set('type', network)
    q.set('security', security)
    if (c.flow) q.set('flow', c.flow)
    if (security === 'tls' && ss.tlsSettings?.serverName) {
      q.set('sni', ss.tlsSettings.serverName)
    }
    if (security === 'reality' && ss.realitySettings) {
      const r = ss.realitySettings
      if (r.publicKey) q.set('pbk', r.publicKey)
      if (Array.isArray(r.shortIds) && r.shortIds.length) q.set('sid', r.shortIds[0])
      if (Array.isArray(r.serverNames) && r.serverNames.length) q.set('sni', r.serverNames[0])
      if (r.fingerprint) q.set('fp', r.fingerprint)
    }
    if (network === 'ws' && ss.wsSettings?.path) q.set('path', ss.wsSettings.path)
    if (network === 'grpc' && ss.grpcSettings?.serviceName) q.set('serviceName', ss.grpcSettings.serviceName)
    return `vless://${c.id}@${host}:${port}?${q.toString()}#${enc(remark)}`
  }
  if (inb.protocol === 'trojan') {
    const q = new URLSearchParams()
    q.set('security', security || 'tls')
    if (ss.tlsSettings?.serverName) q.set('sni', ss.tlsSettings.serverName)
    return `trojan://${c.password}@${host}:${port}?${q.toString()}#${enc(remark)}`
  }
  if (inb.protocol === 'shadowsocks') {
    let method = 'chacha20-ietf-poly1305'
    try {
      const s = JSON.parse(inb.settings)
      if (s.method) method = s.method
    } catch {
      /* noop */
    }
    const userinfo = btoa(`${method}:${c.password}`).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')
    return `ss://${userinfo}@${host}:${port}#${enc(remark)}`
  }
  if (inb.protocol === 'vmess') {
    const obj: Record<string, any> = {
      v: '2',
      ps: remark,
      add: host,
      port,
      id: c.id,
      aid: 0,
      scy: c.security || 'auto',
      net: network,
      type: 'none',
      tls: security,
    }
    if (network === 'ws') {
      if (ss.wsSettings?.path) obj.path = ss.wsSettings.path
    }
    if (security === 'tls' && ss.tlsSettings?.serverName) obj.sni = ss.tlsSettings.serverName
    return 'vmess://' + btoa(JSON.stringify(obj)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')
  }
  return ''
}

async function copyLink(f: FleetInbound, c: Client) {
  const url = buildClientLink(f, c)
  if (!url) {
    flash('err', '不支持的协议')
    return
  }
  try {
    await navigator.clipboard.writeText(url)
    flash('ok', `链接已复制：${c.email}`)
  } catch {
    flash('err', '复制失败 — 浏览器拒绝剪贴板')
  }
}

async function openQR(f: FleetInbound, c: Client) {
  const url = buildClientLink(f, c)
  if (!url) {
    flash('err', '不支持的协议')
    return
  }
  const dataURL = await QRCode.toDataURL(url, { width: 320, margin: 1 })
  qrModal.value = {
    open: true,
    title: `${f.inbound.tag} · ${c.email}`,
    url,
    dataURL,
  }
}

function randomHex(n: number): string {
  const bytes = new Uint8Array(n)
  crypto.getRandomValues(bytes)
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

function regenUUID() {
  clientModal.value.client.id = crypto.randomUUID()
}

function regenPassword() {
  clientModal.value.client.password = randomHex(16)
}

async function copyQRUrl() {
  try {
    await navigator.clipboard.writeText(qrModal.value.url)
    flash('ok', '已复制')
  } catch {
    flash('err', '复制失败')
  }
}

// ---- derived ---------------------------------------------------------------

const filtered = computed<FleetInbound[]>(() => {
  const q = query.value.trim().toLowerCase()
  return data.value.inbounds.filter((f) => {
    const i = f.inbound
    if (protocolFilter.value !== 'all' && i.protocol.toLowerCase() !== protocolFilter.value) return false
    if (!q) return true
    return (
      f.node_name.toLowerCase().includes(q) ||
      i.tag.toLowerCase().includes(q) ||
      i.remark.toLowerCase().includes(q) ||
      i.protocol.toLowerCase().includes(q) ||
      String(i.port).includes(q)
    )
  })
})

const stats = computed(() => {
  const fleet = data.value.inbounds.map((f) => f.inbound)
  const up = fleet.reduce((s, i) => s + (i.up || 0), 0)
  const down = fleet.reduce((s, i) => s + (i.down || 0), 0)
  const allTime = fleet.reduce((s, i) => s + (i.allTime || 0), 0)
  const inboundCount = fleet.length
  const clientCount = fleet.reduce((s, i) => s + (i.clientStats?.length ?? parseClients(i).length), 0)
  const enabledCount = fleet.filter((i) => i.enable).length
  return { up, down, allTime, inboundCount, enabledCount, clientCount }
})

function protoColor(p: string): string {
  return ({
    vless: 'bg-accent-100 text-accent-800 ring-accent-200',
    vmess: 'bg-primary-100 text-primary-800 ring-primary-200',
    trojan: 'bg-amber-100 text-amber-800 ring-amber-200',
    shadowsocks: 'bg-pink-100 text-pink-800 ring-pink-200',
    wireguard: 'bg-emerald-100 text-emerald-800 ring-emerald-200',
  } as Record<string, string>)[p.toLowerCase()] ?? 'bg-surface-200 text-surface-800 ring-surface-300'
}

function securityColor(s: string): string {
  if (s === 'reality') return 'bg-violet-100 text-violet-800 ring-violet-200'
  if (s === 'tls' || s === 'xtls') return 'bg-primary-100 text-primary-700 ring-primary-200'
  return 'bg-surface-100 text-surface-500 ring-surface-200'
}

// Row-level breakdown: how many clients per inbound are online vs offline.
// Used to render the per-row 客户端 cell with Sub2API-style multi-line meta.
function clientBreakdown(row: FleetInbound): { total: number; online: number; offline: number } {
  const clients = parseClients(row.inbound)
  const total = clients.length
  let online = 0
  for (const c of clients) if (isOnline(row.node_id, c.email)) online++
  return { total, online, offline: total - online }
}

// Returns a traffic-usage ratio in [0,1] for the inbound. Falls back to 0 when
// `total` (limit) is 0/undefined — for unlimited inbounds the bar stays empty.
function trafficRatio(row: FleetInbound): number {
  const used = (row.inbound.up || 0) + (row.inbound.down || 0)
  const limit = row.inbound.total || 0
  if (limit <= 0) return 0
  return Math.min(1, used / limit)
}

// Bar color tier — green/amber/red — Marzban-style traffic gradient.
function trafficBarClass(ratio: number): string {
  if (ratio <= 0)    return 'bg-surface-200 dark:bg-surface-700'
  if (ratio < 0.6)   return 'bg-gradient-to-r from-accent-400 to-accent-500'
  if (ratio < 0.85)  return 'bg-gradient-to-r from-amber-400 to-amber-500'
  return 'bg-gradient-to-r from-red-400 to-red-500'
}

onMounted(reload)
</script>

<template>
  <div>
    <!-- Header -->
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">入站列表</h1>
        <p class="mt-1.5 text-sm text-surface-500">跨节点聚合 · 流量 · 客户端 · 点击行展开看每个入站下的客户端</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
          @click="openCreateInbound"
        >
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 5v14M5 12h14" />
          </svg>
          添加入站
        </button>
        <button
          class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800"
          title="刷新"
          @click="reload"
        >
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" />
          </svg>
        </button>
      </div>
    </header>

    <!-- KPI strip — flat hairline cards. Single accent only on the leading "Up" icon. -->
    <section class="mb-6 grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 transition-colors hover:border-surface-200 dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
        <div class="flex items-center gap-1.5 text-2xs font-medium text-surface-500">
          <svg class="h-3 w-3 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5M5 12l7-7 7 7" /></svg>
          上传
        </div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(stats.up) }}</div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 transition-colors hover:border-surface-200 dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
        <div class="flex items-center gap-1.5 text-2xs font-medium text-surface-500">
          <svg class="h-3 w-3 text-primary-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12l7 7 7-7" /></svg>
          下载
        </div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(stats.down) }}</div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 transition-colors hover:border-surface-200 dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
        <div class="text-2xs font-medium text-surface-500">总用量</div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(stats.up + stats.down) }}</div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 transition-colors hover:border-surface-200 dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
        <div class="text-2xs font-medium text-surface-500">累计</div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ formatBytes(stats.allTime) }}</div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 transition-colors hover:border-surface-200 dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
        <div class="text-2xs font-medium text-surface-500">入站</div>
        <div class="mt-2 flex items-baseline gap-1.5">
          <span class="text-lg font-semibold tracking-tight text-ink-900 tabular-nums dark:text-surface-50">{{ stats.inboundCount }}</span>
          <span class="text-2xs text-surface-400">{{ stats.enabledCount }} 启用</span>
        </div>
      </div>
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-4 transition-colors hover:border-surface-200 dark:border-surface-800 dark:bg-surface-900 dark:hover:border-surface-700">
        <div class="text-2xs font-medium text-surface-500">客户端</div>
        <div class="mt-2 text-lg font-semibold tracking-tight text-accent-600 tabular-nums dark:text-accent-400">{{ stats.clientCount }}</div>
      </div>
    </section>

    <!-- Toolbar -->
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <div class="relative">
        <svg class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
        <input
          v-model="query"
          type="text"
          placeholder="搜索 备注 / 协议 / 端口 / 节点 / tag"
          class="h-9 w-80 rounded-xl border border-surface-200 bg-surface-0 pl-9 pr-3 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
        />
      </div>
      <div class="flex h-9 items-center gap-0.5 rounded-xl border border-surface-200 bg-surface-0 p-1 text-xs dark:border-surface-700 dark:bg-surface-900">
        <button
          v-for="p in (['all','vless','vmess','trojan','shadowsocks','wireguard'] as const)"
          :key="p"
          class="rounded-lg px-3 py-1 font-medium transition-all duration-150 ease-brand"
          :class="protocolFilter === p
            ? 'bg-ink-900 text-white shadow-card dark:bg-accent-600'
            : 'text-surface-500 hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-50'"
          @click="protocolFilter = p"
        >
          {{ p === 'all' ? '全部' : p }}
        </button>
      </div>
    </div>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800">{{ error }}</p>
    <div
      v-if="data.node_errors && Object.keys(data.node_errors).length"
      class="mb-4 rounded-xl border border-amber-100 bg-amber-50 px-4 py-3 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300"
    >
      <div class="mb-1 font-semibold">部分节点拉取失败（其他正常显示）：</div>
      <ul class="list-inside list-disc">
        <li v-for="(msg, id) in data.node_errors" :key="id"><b>node {{ id }}:</b> {{ msg }}</li>
      </ul>
    </div>

    <!-- Table -->
    <Skeleton v-if="loading" :rows="6" />
    <div
      v-else
      class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
    >
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="w-8 px-4 py-3 font-medium"></th>
            <th class="px-4 py-3 font-medium">节点</th>
            <th class="px-4 py-3 font-medium">备注 · Tag · 端口</th>
            <th class="px-4 py-3 font-medium">协议</th>
            <th class="hidden"></th>
            <th class="px-4 py-3 font-medium">客户端</th>
            <th class="px-4 py-3 font-medium">流量 · 用量</th>
            <th class="px-4 py-3 text-right font-medium">累计</th>
            <th class="px-4 py-3 font-medium">到期</th>
            <th class="px-4 py-3 text-center font-medium">启用</th>
            <th class="px-4 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <template v-for="row in filtered" :key="rowKey(row)">
            <tr
              class="group cursor-pointer transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40"
              :class="!row.inbound.enable ? 'opacity-60' : ''"
              @click="toggleExpand(row)"
            >
              <td class="px-4 py-3.5">
                <svg
                  class="h-3.5 w-3.5 shrink-0 text-surface-400 transition-transform duration-200 ease-brand"
                  :class="expanded.has(rowKey(row)) ? 'rotate-90 text-accent-600' : ''"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2.2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="m9 18 6-6-6-6" />
                </svg>
              </td>
              <!-- 节点 — chip + IP underneath -->
              <td class="px-4 py-4">
                <div class="inline-flex items-center gap-1.5 rounded-full bg-surface-100 px-2.5 py-0.5 text-xs font-medium text-surface-600 dark:bg-surface-800 dark:text-surface-300">
                  <span class="h-1.5 w-1.5 rounded-full bg-accent-500"></span>
                  {{ row.node_name }}
                </div>
                <div class="mt-1 font-mono text-2xs text-surface-400">node #{{ row.node_id }}</div>
              </td>
              <!-- 备注 / Tag / Port — three lines -->
              <td class="px-4 py-4">
                <div class="font-medium text-ink-900 dark:text-surface-50">{{ row.inbound.remark || '—' }}</div>
                <div class="mt-0.5 font-mono text-2xs text-surface-400">{{ row.inbound.tag }}</div>
                <div class="mt-1 font-mono text-2xs text-surface-500">:{{ row.inbound.port }}</div>
              </td>
              <!-- 协议 — protocol big, transport+security inline below -->
              <td class="px-4 py-4">
                <div class="flex flex-col gap-1">
                  <span class="inline-flex w-fit items-center gap-1 rounded-md px-1.5 py-0.5 text-2xs font-medium ring-1 ring-inset" :class="protoColor(row.inbound.protocol)">
                    <span class="h-1.5 w-1.5 rounded-full bg-current opacity-60"></span>
                    {{ row.inbound.protocol }}
                  </span>
                  <div class="flex items-center gap-1">
                    <span class="rounded-md bg-surface-100 px-1.5 py-0.5 text-2xs font-medium text-surface-600 ring-1 ring-inset ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700">
                      {{ parseTransport(row.inbound.streamSettings).network }}
                    </span>
                    <span class="rounded-md px-1.5 py-0.5 text-2xs font-medium ring-1 ring-inset" :class="securityColor(parseTransport(row.inbound.streamSettings).security)">
                      {{ parseTransport(row.inbound.streamSettings).security }}
                    </span>
                  </div>
                </div>
              </td>
              <!-- 端口 — collapsed into 备注 cell. Skip -->
              <td class="hidden" />
              <!-- 客户端 — count + online/offline breakdown -->
              <td class="px-4 py-4">
                <div class="text-sm font-semibold text-ink-900 tabular-nums dark:text-surface-50">{{ clientBreakdown(row).total }}</div>
                <div class="mt-0.5 flex items-center gap-2 text-2xs text-surface-500">
                  <span class="inline-flex items-center gap-1">
                    <span class="h-1.5 w-1.5 rounded-full bg-accent-500"></span>
                    {{ clientBreakdown(row).online }}
                  </span>
                  <span class="text-surface-300 dark:text-surface-700">·</span>
                  <span class="inline-flex items-center gap-1">
                    <span class="h-1.5 w-1.5 rounded-full bg-surface-300 dark:bg-surface-700"></span>
                    {{ clientBreakdown(row).offline }}
                  </span>
                </div>
              </td>
              <!-- 流量 / 限额 — numbers + Marzban-style progress bar -->
              <td class="px-4 py-4 tabular-nums">
                <div class="flex items-baseline justify-between gap-2">
                  <span class="text-sm font-medium text-ink-900 dark:text-surface-50">{{ formatBytes(row.inbound.up + row.inbound.down) }}</span>
                  <span class="text-2xs text-surface-400">/ {{ formatLimit(row.inbound.total) }}</span>
                </div>
                <div class="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-surface-100 dark:bg-surface-800">
                  <div
                    class="h-full rounded-full transition-all duration-500 ease-brand"
                    :class="trafficBarClass(trafficRatio(row))"
                    :style="{ width: (trafficRatio(row) * 100).toFixed(1) + '%' }"
                  />
                </div>
                <div class="mt-1 flex items-center gap-2 text-2xs text-surface-500">
                  <span class="inline-flex items-center gap-0.5">
                    <svg class="h-2.5 w-2.5 text-accent-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path d="M12 19V5M5 12l7-7 7 7" /></svg>
                    {{ formatBytes(row.inbound.up) }}
                  </span>
                  <span class="inline-flex items-center gap-0.5">
                    <svg class="h-2.5 w-2.5 text-primary-500" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path d="M12 5v14M5 12l7 7 7-7" /></svg>
                    {{ formatBytes(row.inbound.down) }}
                  </span>
                </div>
              </td>
              <!-- 累计 -->
              <td class="px-4 py-4 text-right font-mono tabular-nums text-xs text-surface-500">{{ formatBytes(row.inbound.allTime) }}</td>
              <!-- 到期 -->
              <td class="px-4 py-4 text-xs text-surface-500">{{ formatExpiry(row.inbound.expiryTime) }}</td>
              <!-- 启用 toggle -->
              <td class="px-4 py-4 text-center">
                <button
                  class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors duration-200 ease-brand"
                  :class="row.inbound.enable ? 'bg-accent-500' : 'bg-surface-200 dark:bg-surface-700'"
                  @click.stop="toggleEnable(row)"
                >
                  <span
                    class="inline-block h-4 w-4 transform rounded-full bg-white shadow-card transition-transform duration-200 ease-brand"
                    :class="row.inbound.enable ? 'translate-x-4' : 'translate-x-0.5'"
                  />
                </button>
              </td>
              <!-- 操作 — always-visible labeled mini-buttons (Sub2API style) -->
              <td class="px-4 py-4" @click.stop>
                <div class="flex items-center justify-end gap-1">
                  <button title="编辑入站" class="inline-flex h-7 items-center gap-1 rounded-lg px-2 text-2xs font-medium text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="openEditInbound(row)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 20h9M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" /></svg>
                    编辑
                  </button>
                  <button title="重置流量" class="inline-flex h-7 items-center gap-1 rounded-lg px-2 text-2xs font-medium text-surface-500 transition-colors hover:bg-amber-50 hover:text-amber-700 dark:hover:bg-amber-950/40 dark:hover:text-amber-300" @click="confirmResetInboundTraffic(row)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
                      <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
                      <path d="M21 3v5h-5" />
                      <path d="M3 21v-5h5" />
                    </svg>
                    重置
                  </button>
                  <button title="删除入站" class="inline-flex h-7 items-center gap-1 rounded-lg px-2 text-2xs font-medium text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="confirmDeleteInbound(row)">
                    <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
                    删除
                  </button>
                </div>
              </td>
            </tr>

            <!-- Expanded row: clients sub-table -->
            <tr v-if="expanded.has(rowKey(row))">
              <td colspan="11" class="bg-surface-50/60 px-6 py-4 dark:bg-surface-800/30">
                <div class="mb-3 flex items-center justify-between">
                  <h3 class="text-sm font-semibold text-surface-700 dark:text-surface-300">
                    {{ row.inbound.tag }} · 客户端
                    <span class="ml-1 text-xs text-surface-400">({{ parseClients(row.inbound).length }})</span>
                  </h3>
                  <button
                    class="inline-flex items-center gap-1.5 rounded-md bg-accent-600 px-3 py-1 text-xs font-medium text-white hover:bg-accent-700"
                    @click="openAddClient(row)"
                  >
                    <svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14" /></svg>
                    添加客户端
                  </button>
                </div>

                <div class="overflow-x-auto rounded-lg border border-surface-200 bg-surface-0 dark:border-surface-700 dark:bg-surface-900">
                  <table class="min-w-full divide-y divide-surface-200 text-xs dark:divide-surface-800">
                    <thead class="bg-surface-50 text-left uppercase tracking-wider text-surface-400 dark:bg-surface-800/40">
                      <tr>
                        <th class="px-3 py-2">状态</th>
                        <th class="px-3 py-2">Email</th>
                        <th class="px-3 py-2">凭据</th>
                        <th class="px-3 py-2 text-right">流量 ↑ / ↓</th>
                        <th class="px-3 py-2 text-right">配额</th>
                        <th class="px-3 py-2">到期</th>
                        <th class="px-3 py-2">最后在线</th>
                        <th class="px-3 py-2 text-right">操作</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-surface-200 dark:divide-surface-800">
                      <tr v-for="c in parseClients(row.inbound)" :key="c.email" class="hover:bg-surface-50 dark:hover:bg-surface-800/30">
                        <td class="px-3 py-2">
                          <span
                            class="inline-flex h-2 w-2 rounded-full"
                            :class="isOnline(row.node_id, c.email) ? 'bg-accent-500 shadow-[0_0_0_3px_rgba(20,184,166,0.18)]' : 'bg-surface-300 dark:bg-surface-700'"
                            :title="isOnline(row.node_id, c.email) ? 'online' : 'offline'"
                          />
                        </td>
                        <td class="px-3 py-2 font-medium">{{ c.email }}</td>
                        <td class="px-3 py-2 font-mono text-2xs text-surface-500">
                          {{ (c.id ?? c.password ?? '').slice(0, 12) }}{{ (c.id ?? c.password ?? '').length > 12 ? '…' : '' }}
                        </td>
                        <td class="px-3 py-2 text-right font-mono tabular-nums">
                          <div>{{ formatBytes(clientStatsByEmail(row.inbound, c.email)?.up ?? 0) }}</div>
                          <div class="text-eyebrow text-surface-400">{{ formatBytes(clientStatsByEmail(row.inbound, c.email)?.down ?? 0) }}</div>
                        </td>
                        <td class="px-3 py-2 text-right font-mono tabular-nums">{{ formatLimit(c.totalGB ?? 0) }}</td>
                        <td class="px-3 py-2 text-surface-500">{{ formatExpiry(c.expiryTime ?? 0) }}</td>
                        <td class="px-3 py-2 text-surface-500">
                          {{ lastOnlineAt(row.node_id, c.email)
                            ? new Date((lastOnlineAt(row.node_id, c.email) ?? 0) * 1000).toLocaleString()
                            : '—' }}
                        </td>
                        <td class="px-3 py-2">
                          <div class="flex justify-end gap-1">
                            <button title="复制链接" class="rounded p-1 text-surface-500 hover:bg-surface-100 hover:text-accent-700 dark:hover:bg-surface-800" @click="copyLink(row, c)">
                              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg>
                            </button>
                            <button title="QR 码" class="rounded p-1 text-surface-500 hover:bg-surface-100 hover:text-primary-700 dark:hover:bg-surface-800" @click="openQR(row, c)">
                              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="3" y="14" width="7" height="7" /><path d="M14 14h3v3M21 21v-7m0 0h-3" /></svg>
                            </button>
                            <button title="重置流量" class="rounded p-1 text-surface-500 hover:bg-surface-100 hover:text-amber-700 dark:hover:bg-surface-800" @click="confirmResetClientTraffic(row, c)">
                              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
                                <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
                                <path d="M21 3v5h-5" />
                                <path d="M3 21v-5h5" />
                              </svg>
                            </button>
                            <button title="编辑" class="rounded p-1 text-surface-500 hover:bg-surface-100 hover:text-surface-900 dark:hover:bg-surface-800 dark:hover:text-surface-100" @click="openEditClient(row, c)">
                              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 20h9M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z" /></svg>
                            </button>
                            <button title="删除" class="rounded p-1 text-surface-500 hover:bg-red-50 hover:text-red-700 dark:hover:bg-red-950" @click="confirmDeleteClient(row, c)">
                              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
                            </button>
                          </div>
                        </td>
                      </tr>
                      <tr v-if="parseClients(row.inbound).length === 0">
                        <td colspan="8" class="px-3 py-8 text-center text-surface-400">
                          这个入站还没有客户端，点 "添加客户端" 开始。
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </td>
            </tr>
          </template>
          <tr v-if="filtered.length === 0">
            <td colspan="11" class="p-0">
              <EmptyState
                v-if="data.inbounds.length === 0"
                icon="M12 14a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0"
                title="还没有入站"
                description="入站是端口 + 协议的组合，把客户端流量送到节点。每个节点至少建一个。"
                action-label="添加第一个入站"
                @action="openCreateInbound"
              />
              <EmptyState
                v-else
                icon="M21 21l-4.3-4.3M11 18a7 7 0 1 1 0-14 7 7 0 0 1 0 14z"
                title="没有匹配的入站"
                :description="`搜索 ${JSON.stringify(query)} 在 ${data.inbounds.length} 个入站里没找到。换关键词或重置过滤。`"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Toast -->
    <transition name="fade">
      <div
        v-if="toast"
        class="fixed bottom-6 right-6 z-50 rounded-lg px-4 py-3 shadow-lg ring-1"
        :class="toast.kind === 'ok'
          ? 'bg-accent-600 text-white ring-accent-700'
          : 'bg-red-600 text-white ring-red-700'"
      >
        {{ toast.text }}
      </div>
    </transition>

    <!-- Add/Edit Inbound modal — full 5-tab editor in its own component -->
    <InboundEditorModal
      :open="inboundModal.open"
      :mode="inboundModal.mode"
      :node-i-d="inboundModal.nodeID"
      :tag="inboundModal.tag"
      :source="inboundModal.source"
      :nodes="nodes"
      @close="inboundModal.open = false"
      @saved="onInboundSaved"
    />

    <!-- Add/Edit Client modal -->
    <div
      v-if="clientModal.open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="clientModal.open = false"
    >
      <div class="w-full max-w-xl animate-scale-in rounded-2xl bg-surface-0 p-6 shadow-2xl dark:bg-surface-900">
        <header class="mb-5 flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ clientModal.mode === 'create' ? '添加客户端' : '编辑客户端' }}</h2>
            <p class="mt-0.5 text-xs text-surface-500" v-if="clientModal.row">
              在 <code class="rounded bg-surface-100 px-1 dark:bg-surface-800">{{ clientModal.row.inbound.tag }}</code> ({{ clientModal.row.inbound.protocol }})
            </p>
          </div>
          <button class="rounded p-1 text-surface-400 hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800" @click="clientModal.open = false">
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>

        <form class="space-y-4" @submit.prevent="submitClient">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">Email (label)</label>
              <input v-model="clientModal.client.email" type="text" placeholder="alice or alice@example.com" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div v-if="clientModal.row && (clientModal.row.inbound.protocol === 'vless' || clientModal.row.inbound.protocol === 'vmess')">
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">UUID</label>
              <div class="flex gap-1">
                <input v-model="clientModal.client.id" type="text" class="flex-1 rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-xs dark:border-surface-700 dark:bg-surface-900" />
                <button type="button" class="rounded-lg border border-surface-200 px-2 text-xs hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="regenUUID" title="生成新 UUID">↻</button>
              </div>
            </div>
            <div v-else-if="clientModal.row">
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">Password</label>
              <div class="flex gap-1">
                <input v-model="clientModal.client.password" type="text" class="flex-1 rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-xs dark:border-surface-700 dark:bg-surface-900" />
                <button type="button" class="rounded-lg border border-surface-200 px-2 text-xs hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="regenPassword" title="生成">↻</button>
              </div>
            </div>
            <div v-if="clientModal.row && clientModal.row.inbound.protocol === 'vless'">
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">Flow</label>
              <select v-model="clientModal.client.flow" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900">
                <option value="">(none)</option>
                <option value="xtls-rprx-vision">xtls-rprx-vision</option>
              </select>
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">流量限额 (bytes, 0=无限)</label>
              <input v-model.number="clientModal.client.totalGB" type="number" min="0" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">到期 (unix ms, 0=永不)</label>
              <input v-model.number="clientModal.client.expiryTime" type="number" min="0" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">IP 限制 (0=无)</label>
              <input v-model.number="clientModal.client.limitIp" type="number" min="0" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">Sub ID</label>
              <input v-model="clientModal.client.subId" type="text" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2">
              <label class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">备注</label>
              <input v-model="clientModal.client.comment" type="text" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2 flex items-center gap-2">
              <input id="client-enable" v-model="clientModal.client.enable" type="checkbox" class="h-4 w-4 rounded border-surface-300 text-accent-600" />
              <label for="client-enable" class="text-sm text-surface-700 dark:text-surface-300">启用</label>
            </div>
          </div>

          <p v-if="clientModal.err" class="rounded-lg bg-red-50 px-4 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ clientModal.err }}</p>

          <footer class="flex justify-end gap-2 border-t border-surface-200 pt-4 dark:border-surface-800">
            <button type="button" class="rounded-lg border border-surface-200 px-4 py-2 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="clientModal.open = false">取消</button>
            <button type="submit" :disabled="clientModal.busy" class="rounded-lg bg-accent-600 px-4 py-2 text-sm font-medium text-white hover:bg-accent-700 disabled:opacity-60">
              {{ clientModal.busy ? '处理中…' : (clientModal.mode === 'create' ? '添加' : '保存') }}
            </button>
          </footer>
        </form>
      </div>
    </div>

    <!-- QR modal -->
    <div
      v-if="qrModal.open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="qrModal.open = false"
    >
      <div class="w-full max-w-md animate-scale-in rounded-2xl bg-surface-0 p-6 shadow-2xl dark:bg-surface-900">
        <header class="mb-4 flex items-center justify-between">
          <h2 class="text-base font-semibold">{{ qrModal.title }}</h2>
          <button class="rounded p-1 text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800" @click="qrModal.open = false">
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <div class="flex flex-col items-center gap-4">
          <img :src="qrModal.dataURL" alt="QR" class="rounded-lg border border-surface-200 dark:border-surface-700" />
          <textarea readonly :value="qrModal.url" rows="3" class="w-full resize-none rounded-lg border border-surface-200 bg-surface-50 p-2 font-mono text-2xs dark:border-surface-700 dark:bg-surface-800"></textarea>
          <button class="rounded-lg bg-accent-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-accent-700" @click="copyQRUrl">复制链接</button>
        </div>
      </div>
    </div>

    <!-- Confirm dialog -->
    <div
      v-if="confirmDialog.open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="confirmDialog.open = false"
    >
      <div class="w-full max-w-md animate-scale-in rounded-2xl bg-surface-0 p-6 shadow-2xl dark:bg-surface-900">
        <h3 class="mb-2 text-lg font-semibold">{{ confirmDialog.title }}</h3>
        <p class="whitespace-pre-line text-sm text-surface-600 dark:text-surface-300">{{ confirmDialog.message }}</p>
        <footer class="mt-5 flex justify-end gap-2">
          <button class="rounded-lg border border-surface-200 px-4 py-1.5 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="confirmDialog.open = false">取消</button>
          <button :disabled="confirmDialog.busy" class="rounded-lg bg-red-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-60" @click="confirmDialog.onConfirm()">
            {{ confirmDialog.busy ? '处理中…' : '确认' }}
          </button>
        </footer>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
