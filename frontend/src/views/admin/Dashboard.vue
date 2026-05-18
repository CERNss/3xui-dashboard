<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { nodesApi, type Node } from '@/api/admin/nodes'

const nodes = ref<Node[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    nodes.value = await nodesApi.list()
  } catch (e: any) {
    error.value = e?.message ?? 'Failed to load nodes'
  } finally {
    loading.value = false
  }
}

async function probe(id: number) {
  try {
    await nodesApi.probe(id)
    await reload()
  } catch (e: any) {
    error.value = e?.message ?? 'Probe failed'
  }
}

async function toggleEnable(n: Node) {
  try {
    if (n.enabled) {
      await nodesApi.disable(n.id)
    } else {
      await nodesApi.enable(n.id)
    }
    await reload()
  } catch (e: any) {
    error.value = e?.message ?? 'Toggle failed'
  }
}

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-6 flex items-center justify-between">
      <h1 class="text-xl font-semibold">{{ $t('nav.dashboard') }}</h1>
      <button
        class="rounded-md border border-surface-300 px-3 py-1.5 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="reload"
      >
        Refresh
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <div v-else class="overflow-x-auto rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900">
      <table class="min-w-full divide-y divide-surface-200 text-sm dark:divide-surface-800">
        <thead class="bg-surface-50 text-left text-xs uppercase tracking-wide text-surface-500 dark:bg-surface-800/40">
          <tr>
            <th class="px-4 py-3">ID</th>
            <th class="px-4 py-3">Name</th>
            <th class="px-4 py-3">Host</th>
            <th class="px-4 py-3">Status</th>
            <th class="px-4 py-3">CPU / Mem</th>
            <th class="px-4 py-3">Xray</th>
            <th class="px-4 py-3 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-200 dark:divide-surface-800">
          <tr v-for="n in nodes" :key="n.id" :class="n.enabled ? '' : 'opacity-60'">
            <td class="px-4 py-3">{{ n.id }}</td>
            <td class="px-4 py-3 font-medium">{{ n.name }}</td>
            <td class="px-4 py-3 font-mono text-xs">{{ n.scheme }}://{{ n.host }}:{{ n.port }}{{ n.base_path }}</td>
            <td class="px-4 py-3">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium"
                :class="{
                  'bg-accent-100 text-accent-800': n.status === 'online',
                  'bg-red-100 text-red-800': n.status === 'offline',
                  'bg-surface-200 text-surface-700': n.status === 'unknown',
                }"
              >{{ n.status }}</span>
            </td>
            <td class="px-4 py-3 tabular-nums">{{ n.cpu_pct.toFixed(1) }}% / {{ n.mem_pct.toFixed(1) }}%</td>
            <td class="px-4 py-3 text-xs">{{ n.xray_version || '—' }}</td>
            <td class="px-4 py-3">
              <div class="flex justify-end gap-2 text-xs">
                <button class="rounded border border-surface-300 px-2 py-1 hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="probe(n.id)">Probe</button>
                <button class="rounded border border-surface-300 px-2 py-1 hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="toggleEnable(n)">
                  {{ n.enabled ? 'Disable' : 'Enable' }}
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="nodes.length === 0">
            <td colspan="7" class="px-4 py-10 text-center text-surface-500">
              No nodes configured. POST /api/admin/nodes to add one.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
