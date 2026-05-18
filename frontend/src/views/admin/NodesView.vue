<template>
  <div class="space-y-4 animate-fade-in">
    <div>
      <h2 class="text-lg font-semibold text-gray-900">Nodes</h2>
      <p class="text-sm text-gray-500">{{ nodes.length }} registered nodes</p>
    </div>

    <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      <div v-for="i in 3" :key="i" class="card animate-pulse">
        <div class="h-4 bg-gray-200 rounded w-2/3 mb-3"></div>
        <div class="h-3 bg-gray-200 rounded w-1/2 mb-2"></div>
        <div class="h-3 bg-gray-200 rounded w-1/3"></div>
      </div>
    </div>

    <div v-else-if="nodes.length === 0" class="card text-center py-12 text-gray-400">
      No nodes configured
    </div>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      <div
        v-for="node in nodes"
        :key="node.id"
        class="card hover:shadow-md transition-shadow"
      >
        <!-- Header -->
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="font-semibold text-gray-900">{{ node.name }}</h3>
            <p class="text-xs text-gray-500">{{ node.address }}:{{ node.port }}</p>
          </div>
          <span
            class="badge"
            :class="{
              'badge-green': node.status === 'online',
              'badge-red': node.status === 'offline',
              'badge-gray': node.status === 'unknown'
            }"
          >
            {{ node.status }}
          </span>
        </div>

        <!-- Metrics -->
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div class="bg-gray-50 rounded-lg p-3">
            <p class="text-gray-500 text-xs mb-0.5">CPU</p>
            <p class="font-semibold text-gray-800">{{ node.cpuPct?.toFixed(1) ?? '—' }}%</p>
            <div class="mt-1 h-1.5 bg-gray-200 rounded-full overflow-hidden">
              <div
                class="h-full bg-primary-500 rounded-full"
                :style="{ width: `${Math.min(node.cpuPct ?? 0, 100)}%` }"
              />
            </div>
          </div>
          <div class="bg-gray-50 rounded-lg p-3">
            <p class="text-gray-500 text-xs mb-0.5">Memory</p>
            <p class="font-semibold text-gray-800">{{ node.memPct?.toFixed(1) ?? '—' }}%</p>
            <div class="mt-1 h-1.5 bg-gray-200 rounded-full overflow-hidden">
              <div
                class="h-full bg-blue-500 rounded-full"
                :style="{ width: `${Math.min(node.memPct ?? 0, 100)}%` }"
              />
            </div>
          </div>
        </div>

        <!-- Footer info -->
        <div class="mt-3 flex items-center justify-between text-xs text-gray-500">
          <span>Latency: {{ node.latencyMs ? `${node.latencyMs}ms` : '—' }}</span>
          <span>Xray {{ node.xrayVersion || '—' }}</span>
          <span>Up {{ formatUptime(node.uptimeSecs) }}</span>
        </div>

        <div v-if="node.lastError" class="mt-2 text-xs text-red-500 bg-red-50 rounded px-2 py-1 truncate">
          {{ node.lastError }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { adminNodesApi } from '@/api'
import type { Node } from '@/types'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const nodes = ref<Node[]>([])
const loading = ref(true)

function formatUptime(secs: number): string {
  if (!secs) return '—'
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  if (d > 0) return `${d}d ${h}h`
  const m = Math.floor((secs % 3600) / 60)
  return `${h}h ${m}m`
}

onMounted(async () => {
  try {
    nodes.value = await adminNodesApi.list()
  } catch {
    appStore.error('Failed to load nodes')
  } finally {
    loading.value = false
  }
})
</script>
