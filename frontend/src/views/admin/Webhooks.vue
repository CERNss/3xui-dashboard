<script setup lang="ts">
import { onMounted, ref } from 'vue'

import {
  adminWebhooksApi,
  type Webhook,
  type WebhookDelivery,
  type WebhookInput,
  type WebhookFormat,
  type WebhookMethod,
} from '@/api/admin/webhooks'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

const rows = ref<Webhook[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const editor = ref<{
  open: boolean
  mode: 'create' | 'edit'
  id: number | null
  draft: WebhookInput
  busy: boolean
  err: string | null
}>(makeEditor())

function makeEditor() {
  return {
    open: false,
    mode: 'create' as const,
    id: null as number | null,
    draft: blankDraft(),
    busy: false,
    err: null as string | null,
  }
}

function blankDraft(): WebhookInput {
  return {
    name: '',
    url: '',
    events: [],
    enabled: true,
    allow_private: false,
    method: 'POST',
    headers: {},
    body_template: '',
    template_format: 'json',
  }
}

// Headers UI: editable as a flat string `Key: Value` per line; we
// parse on save so admins don't fight a fiddly key-value-pair table.
const headersText = ref('')

function headersToText(h: Record<string, string>): string {
  return Object.entries(h)
    .map(([k, v]) => `${k}: ${v}`)
    .join('\n')
}

function textToHeaders(s: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of s.split('\n')) {
    const idx = line.indexOf(':')
    if (idx <= 0) continue
    const k = line.slice(0, idx).trim()
    const v = line.slice(idx + 1).trim()
    if (k) out[k] = v
  }
  return out
}

// Events UI: editable as comma-separated patterns.
const eventsText = ref('')

async function reload() {
  loading.value = true
  error.value = null
  try {
    rows.value = await adminWebhooksApi.list()
  } catch (e: any) {
    error.value = formatError(e, '加载 webhooks 失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editor.value = makeEditor()
  editor.value.open = true
  headersText.value = ''
  eventsText.value = '*'
}

function openEdit(w: Webhook) {
  editor.value = {
    open: true,
    mode: 'edit',
    id: w.id,
    draft: {
      name: w.name,
      url: w.url,
      events: w.events,
      enabled: w.enabled,
      allow_private: w.allow_private,
      method: w.method,
      headers: { ...w.headers },
      body_template: w.body_template,
      template_format: w.template_format,
    },
    busy: false,
    err: null,
  }
  headersText.value = headersToText(w.headers)
  eventsText.value = w.events.join(', ')
}

async function save() {
  editor.value.err = null
  editor.value.busy = true
  try {
    editor.value.draft.headers = textToHeaders(headersText.value)
    editor.value.draft.events = eventsText.value
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
    if (editor.value.mode === 'create') {
      await adminWebhooksApi.create(editor.value.draft)
    } else if (editor.value.id) {
      await adminWebhooksApi.update(editor.value.id, editor.value.draft)
    }
    editor.value.open = false
    await reload()
  } catch (e: any) {
    editor.value.err = formatError(e, '保存失败')
  } finally {
    editor.value.busy = false
  }
}

async function destroy(w: Webhook) {
  const ok = await askConfirm({
    title: '删除 webhook',
    message: `Webhook "${w.name}" 及其历史 deliveries 会被级联删除。`,
    variant: 'danger',
    confirmLabel: '删除',
  })
  if (!ok) return
  try {
    await adminWebhooksApi.remove(w.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, '删除失败')
  }
}

// Deliveries panel: lazy-fetched per webhook on first expand. The
// underlying endpoint returns the most-recent N rows (server-side
// limit; client just renders what it gets). Closes by re-clicking
// the toggle.
const deliveries = ref<Record<number, WebhookDelivery[]>>({})
const expandedID = ref<number | null>(null)
const deliveriesLoading = ref<number | null>(null)
const deliveriesErr = ref<string | null>(null)

async function toggleDeliveries(w: Webhook) {
  if (expandedID.value === w.id) {
    expandedID.value = null
    return
  }
  expandedID.value = w.id
  if (deliveries.value[w.id]) return // already cached
  deliveriesLoading.value = w.id
  deliveriesErr.value = null
  try {
    deliveries.value[w.id] = await adminWebhooksApi.deliveries(w.id)
  } catch (e: any) {
    deliveriesErr.value = formatError(e, '加载 deliveries 失败')
  } finally {
    deliveriesLoading.value = null
  }
}

function deliveryChipClass(status: string): string {
  switch (status) {
    case 'success':
      return 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
    case 'failed':
      return 'bg-red-50 text-red-700 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'
    default:
      return 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800'
  }
}

// Per-row pending state so admin sees "test 中…" feedback while
// the call is in flight, and a transient flash on completion.
const testBusy = ref<number | null>(null)
const testFlash = ref<{ id: number; kind: 'ok' | 'err'; text: string } | null>(null)

async function testFire(w: Webhook) {
  testBusy.value = w.id
  testFlash.value = null
  try {
    await adminWebhooksApi.test(w.id)
    testFlash.value = { id: w.id, kind: 'ok', text: '已派发，查看下方记录' }
    // Auto-expand the deliveries panel + refresh — admin almost
    // always wants to see the result of the test fire, not just
    // a "sent" confirmation.
    deliveries.value[w.id] = await adminWebhooksApi.deliveries(w.id)
    expandedID.value = w.id
  } catch (e: any) {
    testFlash.value = { id: w.id, kind: 'err', text: formatError(e, 'test 发送失败') }
  } finally {
    testBusy.value = null
    // Auto-clear after 4s so the table doesn't accumulate stale notices.
    setTimeout(() => {
      if (testFlash.value?.id === w.id) testFlash.value = null
    }, 4000)
  }
}

function chipForMethod(m: WebhookMethod): string {
  return ({
    GET: 'bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-950/40 dark:text-sky-300 dark:ring-sky-800',
    POST: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:ring-emerald-800',
    PUT: 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800',
    DELETE: 'bg-red-50 text-red-700 ring-red-200 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800',
    PATCH: 'bg-violet-50 text-violet-700 ring-violet-200 dark:bg-violet-950/40 dark:text-violet-300 dark:ring-violet-800',
  } as Record<string, string>)[m] ?? ''
}

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">Webhooks</h1>
        <p class="mt-1.5 text-sm text-surface-500">配置外部回调：事件订阅 + GET/POST + Header 自定义 + Body Template</p>
      </div>
      <button
        type="button"
        class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
        @click="openCreate"
      >
        + 新建 webhook
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" :rows="4" />

    <div v-else-if="rows.length > 0" class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">名称</th>
            <th class="px-6 py-3 font-medium">Method</th>
            <th class="px-6 py-3 font-medium">URL</th>
            <th class="px-6 py-3 font-medium">事件</th>
            <th class="px-6 py-3 font-medium">模板</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <template v-for="w in rows" :key="w.id">
          <tr class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ w.name }}</td>
            <td class="px-6 py-3.5">
              <span class="inline-flex items-center rounded-md px-2 py-0.5 text-2xs font-medium ring-1 ring-inset" :class="chipForMethod(w.method)">{{ w.method }}</span>
            </td>
            <td class="px-6 py-3.5 font-mono text-xs text-surface-600 dark:text-surface-300">{{ w.url }}</td>
            <td class="px-6 py-3.5 font-mono text-2xs text-surface-500">{{ w.events.join(', ') || '*' }}</td>
            <td class="px-6 py-3.5 text-2xs text-surface-500">
              {{ w.body_template ? `${w.template_format} • ${w.body_template.length} chars` : `default (${w.template_format})` }}
            </td>
            <td class="px-6 py-3.5">
              <span v-if="w.enabled" class="inline-flex items-center rounded-full bg-accent-50 px-2 py-0.5 text-xs font-medium text-accent-700 ring-1 ring-inset ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800">启用</span>
              <span v-else class="inline-flex items-center rounded-full bg-surface-100 px-2 py-0.5 text-xs font-medium text-surface-500 ring-1 ring-inset ring-surface-200 dark:bg-surface-800 dark:text-surface-400 dark:ring-surface-700">禁用</span>
            </td>
            <td class="px-6 py-3.5 text-right">
              <div class="inline-flex items-center gap-1">
                <button class="rounded-lg border border-surface-200 px-2.5 py-1 text-xs font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" type="button" @click="toggleDeliveries(w)">{{ expandedID === w.id ? '收起' : '记录' }}</button>
                <button
                  type="button"
                  class="rounded-lg border border-surface-200 px-2.5 py-1 text-xs font-medium text-surface-700 transition-colors hover:bg-surface-50 disabled:opacity-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
                  :disabled="testBusy === w.id"
                  @click="testFire(w)"
                >
                  {{ testBusy === w.id ? '发送中…' : 'test' }}
                </button>
                <span
                  v-if="testFlash && testFlash.id === w.id"
                  class="ml-1 text-2xs"
                  :class="testFlash.kind === 'ok' ? 'text-accent-600' : 'text-red-600'"
                >
                  {{ testFlash.text }}
                </span>
                <button class="rounded-lg border border-surface-200 px-2.5 py-1 text-xs font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" type="button" @click="openEdit(w)">编辑</button>
                <button class="rounded-lg border border-red-200 px-2.5 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-50 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-950/40" type="button" @click="destroy(w)">删除</button>
              </div>
            </td>
          </tr>
          <!-- Expanded deliveries subrow — fetched lazily on first
               "记录" click, cached after that. Shows the N most-recent
               attempts for this webhook so admin can verify the
               receiver is actually getting hit. -->
          <tr v-if="expandedID === w.id">
            <td colspan="7" class="bg-surface-50/60 px-6 py-4 dark:bg-surface-800/40">
              <div v-if="deliveriesLoading === w.id" class="text-xs text-surface-500">加载 deliveries…</div>
              <p v-else-if="deliveriesErr" class="text-xs text-red-600">{{ deliveriesErr }}</p>
              <div v-else-if="deliveries[w.id]?.length === 0" class="text-xs text-surface-500">还没有投递记录</div>
              <div v-else class="space-y-1.5">
                <div
                  v-for="d in deliveries[w.id]"
                  :key="d.id"
                  class="grid grid-cols-[auto_1fr_auto_auto] items-center gap-3 rounded-lg border border-surface-100 bg-surface-0 px-3 py-1.5 text-2xs dark:border-surface-700 dark:bg-surface-900"
                >
                  <span class="inline-flex items-center rounded-full px-1.5 py-0.5 font-medium ring-1 ring-inset" :class="deliveryChipClass(d.status)">{{ d.status }}</span>
                  <span class="font-mono text-surface-600 dark:text-surface-300">{{ d.event_type }}</span>
                  <span class="font-mono text-surface-500">attempt {{ d.attempt }} · HTTP {{ d.http_status || '—' }}</span>
                  <span class="font-mono text-surface-400">{{ new Date(d.scheduled_at).toLocaleString() }}</span>
                  <p v-if="d.error" class="col-span-4 truncate font-mono text-red-600">err: {{ d.error }}</p>
                </div>
              </div>
            </td>
          </tr>
          </template>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else
      icon="M13 10V3L4 14h7v7l9-11h-7z"
      title="还没有 webhook"
      description="新建一个 webhook 把订单 / 节点 / 客户端事件推到外部系统。"
    />

    <!-- Editor modal -->
    <div
      v-if="editor.open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/50 p-4"
      @click.self="editor.open = false"
    >
      <div class="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-4 dark:border-surface-800">
          <h2 class="text-lg font-semibold">{{ editor.mode === 'create' ? '新建 webhook' : `编辑 webhook #${editor.id}` }}</h2>
          <button type="button" class="rounded p-1 text-surface-400 hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800" @click="editor.open = false">
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>

        <form class="flex-1 space-y-4 overflow-y-auto px-6 py-5" @submit.prevent="save">
          <div class="grid grid-cols-2 gap-3">
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">名称</span>
              <input v-model="editor.draft.name" type="text" required class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900" />
            </label>
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">Method</span>
              <select v-model="editor.draft.method" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900">
                <option value="POST">POST</option>
                <option value="GET">GET</option>
                <option value="PUT">PUT</option>
                <option value="DELETE">DELETE</option>
                <option value="PATCH">PATCH</option>
              </select>
            </label>
          </div>

          <label class="block">
            <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">URL</span>
            <input v-model="editor.draft.url" type="url" required class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-sm dark:border-surface-700 dark:bg-surface-900" placeholder="https://hooks.example.com/incoming" />
          </label>

          <label class="block">
            <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">事件订阅（逗号分隔，<code>*</code> 表示全部，<code>order.*</code> 表示前缀匹配）</span>
            <input v-model="eventsText" type="text" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-sm dark:border-surface-700 dark:bg-surface-900" placeholder="order.*, node.offline, client.expired" />
          </label>

          <div class="grid grid-cols-2 gap-3">
            <label class="block">
              <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">Body Format</span>
              <select v-model="editor.draft.template_format" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900">
                <option value="json">JSON (application/json)</option>
                <option value="form">Form (urlencoded)</option>
                <option value="text">Text (text/plain)</option>
                <option value="raw">Raw (自定 Content-Type)</option>
              </select>
            </label>
            <div class="flex items-end gap-4">
              <label class="flex items-center gap-2 text-sm">
                <input v-model="editor.draft.enabled" type="checkbox" class="h-4 w-4 rounded border-surface-300" />
                启用
              </label>
              <label class="flex items-center gap-2 text-sm">
                <input v-model="editor.draft.allow_private" type="checkbox" class="h-4 w-4 rounded border-surface-300" />
                允许内网 URL
              </label>
            </div>
          </div>

          <label class="block">
            <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">
              Body Template（Go text/template；留空使用默认 JSON envelope）
            </span>
            <textarea v-model="editor.draft.body_template" rows="6" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-xs dark:border-surface-700 dark:bg-surface-900" placeholder="留空使用默认 JSON envelope" />
            <p class="mt-1 text-2xs text-surface-500">可用变量（Go text/template）：<code>.Event</code> <code>.Version</code> <code>.Timestamp</code> <code>.Data.&lt;字段名&gt;</code>。访问不存在的字段渲染为空，不会断开 delivery。</p>
          </label>

          <label class="block">
            <span class="mb-1 block text-xs font-medium text-surface-600 dark:text-surface-300">自定义 Headers（每行一条 <code>Key: Value</code>）</span>
            <textarea v-model="headersText" rows="3" class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-xs dark:border-surface-700 dark:bg-surface-900" placeholder="Authorization: Bearer xxx&#10;X-Custom-Header: value" />
          </label>

          <p v-if="editor.err" class="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ editor.err }}</p>

          <div class="flex justify-end gap-2 border-t border-surface-100 pt-4 dark:border-surface-800">
            <button type="button" class="rounded-lg border border-surface-200 px-4 py-2 text-sm font-medium text-surface-700 hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="editor.open = false">取消</button>
            <button type="submit" :disabled="editor.busy" class="rounded-lg bg-ink-900 px-4 py-2 text-sm font-medium text-white hover:bg-ink-800 disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">
              {{ editor.busy ? '保存中…' : '保存' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <ConfirmModal
      v-if="confirmState"
      :open="confirmState.open"
      :title="confirmState.title"
      :message="confirmState.message"
      :confirm-label="confirmState.confirmLabel"
      :variant="confirmState.variant"
      @confirm="settleConfirm(true)"
      @cancel="settleConfirm(false)"
    />
  </div>
</template>
