<template>
  <div class="space-y-6 animate-fade-in">
    <div>
      <h2 class="text-lg font-semibold text-gray-900">Overview</h2>
      <p class="text-sm text-gray-500 mt-0.5">Real-time summary from 3x-ui</p>
    </div>

    <!-- Stats grid -->
    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <div v-for="i in 4" :key="i" class="card animate-pulse">
        <div class="h-4 bg-gray-200 rounded w-1/2 mb-3"></div>
        <div class="h-8 bg-gray-200 rounded w-1/3"></div>
      </div>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <StatCard
        title="Total Clients"
        :value="stats?.totalClients ?? 0"
        subtitle="Registered in all inbounds"
        icon="👤"
        icon-bg="bg-blue-100"
      />
      <StatCard
        title="Active Clients"
        :value="stats?.activeClients ?? 0"
        subtitle="Enabled and not expired"
        icon="✅"
        icon-bg="bg-green-100"
      />
      <StatCard
        title="Total Traffic"
        :value="formatBytes((stats?.totalUp ?? 0) + (stats?.totalDown ?? 0))"
        subtitle="Upload + Download"
        icon="📊"
        icon-bg="bg-purple-100"
      />
      <StatCard
        title="Nodes Online"
        :value="stats?.nodesOnline ?? 0"
        subtitle="Responsive nodes"
        icon="🌐"
        icon-bg="bg-yellow-100"
      />
    </div>

    <!-- Traffic breakdown -->
    <div v-if="stats" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-3">Upload Traffic</h3>
        <p class="text-3xl font-bold text-primary-600">{{ formatBytes(stats.totalUp) }}</p>
      </div>
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-3">Download Traffic</h3>
        <p class="text-3xl font-bold text-primary-600">{{ formatBytes(stats.totalDown) }}</p>
      </div>
    </div>

    <!-- Quick links -->
    <div class="card">
      <h3 class="font-semibold text-gray-700 mb-4">Quick Actions</h3>
      <div class="flex flex-wrap gap-3">
        <router-link to="/admin/inbounds" class="btn-primary">Manage Inbounds</router-link>
        <router-link to="/admin/clients" class="btn-secondary">View Clients</router-link>
        <router-link to="/admin/nodes" class="btn-secondary">Check Nodes</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { adminStatsApi } from '@/api'
import type { AdminStats } from '@/types'
import StatCard from '@/components/common/StatCard.vue'

const stats = ref<AdminStats | null>(null)
const loading = ref(true)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`
}

onMounted(async () => {
  try {
    stats.value = await adminStatsApi.get()
  } catch {
    // stats remain null; show zeros
  } finally {
    loading.value = false
  }
})
</script>
