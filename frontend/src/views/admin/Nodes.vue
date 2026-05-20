<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { formatError, nodeStatusLabel } from '@/utils/format'

import { nodesApi, type Node, type NodeInput } from '@/api/admin/nodes'
import Skeleton from '@/components/common/Skeleton.vue'
import EmptyState from '@/components/common/EmptyState.vue'

const nodes = ref<Node[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const showCreate = ref(false)
const creating = ref(false)
const createErr = ref<string | null>(null)
const form = ref<NodeInput>({
  name: '',
  scheme: 'https',
  host: '',
  port: 2053,
  base_path: '',
  api_token: '',
  enabled: true,
})

async function reload() {
  loading.value = true
  error.value = null
  try {
    nodes.value = await nodesApi.list()
  } catch (e: any) {
    error.value = formatError(e, '加载节点列表失败')
  } finally {
    loading.value = false
  }
}

async function probe(id: number) {
  try {
    await nodesApi.probe(id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, '探测失败')
  }
}

async function toggleEnable(n: Node) {
  try {
    if (n.enabled) await nodesApi.disable(n.id)
    else await nodesApi.enable(n.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, '切换状态失败')
  }
}

async function destroy(n: Node) {
  if (!confirm(`确认删除节点 "${n.name}"？\n所有附属的 client_ownerships 会被级联删除。`)) return
  try {
    await nodesApi.remove(n.id)
    await reload()
  } catch (e: any) {
    error.value = formatError(e, '删除失败')
  }
}

async function submit() {
  createErr.value = null
  if (!form.value.name.trim() || !form.value.host.trim() || !(form.value.api_token ?? '').trim()) {
    createErr.value = 'name / host / api_token 必填'
    return
  }
  creating.value = true
  try {
    await nodesApi.create(form.value)
    showCreate.value = false
    form.value = {
      name: '',
      scheme: 'https',
      host: '',
      port: 2053,
      base_path: '',
      api_token: '',
      enabled: true,
    }
    await reload()
  } catch (e: any) {
    createErr.value = formatError(e, '创建失败')
  } finally {
    creating.value = false
  }
}

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex items-end justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">节点列表</h1>
        <p class="mt-1.5 text-sm text-surface-500">每台节点对应一台上游 3x-ui 面板 · 后台 30 秒探测一次</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
          @click="showCreate = true"
        >
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14M5 12h14" /></svg>
          添加节点
        </button>
        <button class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-surface-200 bg-surface-0 text-surface-600 transition-all ease-brand hover:border-surface-300 hover:bg-surface-50 hover:text-ink-900 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800" @click="reload">
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M3 21v-5h5" /></svg>
        </button>
      </div>
    </header>

    <p v-if="error" class="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" :rows="4" />

    <div
      v-else
      class="overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
    >
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">ID</th>
            <th class="px-6 py-3 font-medium">名称</th>
            <th class="px-6 py-3 font-medium">连接</th>
            <th class="px-6 py-3 font-medium">状态</th>
            <th class="px-6 py-3 font-medium">CPU / Mem</th>
            <th class="px-6 py-3 font-medium">Xray</th>
            <th class="px-6 py-3 font-medium">Last Seen</th>
            <th class="px-6 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="n in nodes" :key="n.id" :class="n.enabled ? '' : 'opacity-60'" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-400 tabular-nums">#{{ n.id }}</td>
            <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ n.name }}</td>
            <td class="px-6 py-3.5 font-mono text-xs text-surface-500">{{ n.scheme }}://{{ n.host }}:{{ n.port }}{{ n.base_path }}</td>
            <td class="px-6 py-3.5">
              <span
                class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                :class="{
                  'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800': n.status === 'online',
                  'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800': n.status === 'offline',
                  'bg-surface-100 text-surface-500 ring-surface-200 dark:bg-surface-800 dark:text-surface-400 dark:ring-surface-700': n.status === 'unknown',
                }"
              >
                <span class="h-1.5 w-1.5 rounded-full" :class="{
                  'bg-accent-500 shadow-[0_0_0_3px_rgba(20,184,166,0.18)]': n.status === 'online',
                  'bg-red-500': n.status === 'offline',
                  'bg-surface-400': n.status === 'unknown',
                }" />
                {{ nodeStatusLabel(n.status) }}
              </span>
            </td>
            <td class="px-6 py-3.5 tabular-nums text-surface-600 dark:text-surface-300">{{ n.cpu_pct.toFixed(1) }}% · {{ n.mem_pct.toFixed(1) }}%</td>
            <td class="px-6 py-3.5 font-mono text-xs text-surface-500">{{ n.xray_version || '—' }}</td>
            <td class="px-6 py-3.5 text-xs text-surface-500">{{ n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : '—' }}</td>
            <td class="px-6 py-3.5">
              <div class="flex justify-end gap-0.5">
                <button title="探测" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-accent-50 hover:text-accent-700 dark:hover:bg-accent-950/40 dark:hover:text-accent-300" @click="probe(n.id)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
                </button>
                <button :title="n.enabled ? '禁用' : '启用'" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50" @click="toggleEnable(n)">
                  <svg v-if="n.enabled" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M8 12h8" /></svg>
                  <svg v-else class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3" /></svg>
                </button>
                <button title="删除" class="flex h-7 w-7 items-center justify-center rounded-lg text-surface-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/40 dark:hover:text-red-400" @click="destroy(n)">
                  <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="nodes.length === 0">
            <td colspan="8" class="p-0">
              <EmptyState
                icon="M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1zM5 14h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1zM7 7h.01M7 17h.01"
                title="还没有节点"
                description="先接入第一台上游 3x-ui 面板：填 host、port、base path、API token，dashboard 会自动 probe 它。"
                action-label="添加第一个节点"
                @action="showCreate = true"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Add modal -->
    <div
      v-if="showCreate"
      class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="showCreate = false"
    >
      <div class="w-full max-w-xl animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <header class="flex items-center justify-between border-b border-surface-100 px-6 py-5 dark:border-surface-800">
          <div>
            <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">添加节点</h2>
            <p class="mt-0.5 text-xs text-surface-500">填好 host / token，dashboard 会自动探测</p>
          </div>
          <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800" @click="showCreate = false">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
        </header>
        <form class="space-y-5 px-6 py-5" @submit.prevent="submit">
          <div class="grid grid-cols-2 gap-3.5">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">名称</label>
              <input v-model="form.name" type="text" placeholder="tokyo-1" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">Scheme</label>
              <select v-model="form.scheme" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900">
                <option value="https">https</option>
                <option value="http">http</option>
              </select>
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">Host</label>
              <input v-model="form.host" type="text" placeholder="node1.example.com" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">Port</label>
              <input v-model.number="form.port" type="number" min="1" max="65535" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">Base path</label>
              <input v-model="form.base_path" type="text" placeholder="/admin/ 或空" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">API Token</label>
              <input v-model="form.api_token" type="text" placeholder="从节点 3x-ui 面板 Settings → API Tokens 获取" class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3 py-2 font-mono text-xs transition-colors focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900" />
            </div>
            <div class="col-span-2 flex items-center gap-2">
              <input id="node-enable" v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded-md border-surface-300 text-accent-600 focus:ring-accent-500/30" />
              <label for="node-enable" class="text-sm text-surface-700 dark:text-surface-300">启用（默认 true）</label>
            </div>
          </div>
          <p v-if="createErr" class="rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800">{{ createErr }}</p>
          <footer class="flex justify-end gap-2 border-t border-surface-100 -mx-6 -mb-5 px-6 py-4 dark:border-surface-800">
            <button type="button" class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800" @click="showCreate = false">取消</button>
            <button type="submit" :disabled="creating" class="inline-flex h-9 items-center rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500">
              {{ creating ? '创建中…' : '创建' }}
            </button>
          </footer>
        </form>
      </div>
    </div>
  </div>
</template>
