<template>
  <div class="space-y-4 animate-fade-in">
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900">Clients</h2>
        <p class="text-sm text-gray-500">All clients across inbounds</p>
      </div>
      <input v-model="search" class="input w-full sm:w-48" placeholder="Search email..." />
    </div>

    <div v-if="loading" class="card flex justify-center py-12">
      <LoadingSpinner size="lg" label="Loading clients..." />
    </div>

    <div v-else class="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Email</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Inbound</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Upload</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Download</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Limit</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Expiry</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-if="filteredClients.length === 0">
              <td colspan="7" class="px-4 py-8 text-center text-gray-400">No clients found</td>
            </tr>
            <tr
              v-for="client in filteredClients"
              :key="`${client.inboundId}-${client.email}`"
              class="hover:bg-gray-50 transition-colors"
            >
              <td class="px-4 py-3 font-medium text-gray-900">{{ client.email }}</td>
              <td class="px-4 py-3 text-gray-600">#{{ client.inboundId }}</td>
              <td class="px-4 py-3 text-gray-600">{{ formatBytes(client.up) }}</td>
              <td class="px-4 py-3 text-gray-600">{{ formatBytes(client.down) }}</td>
              <td class="px-4 py-3 text-gray-600">{{ client.total ? formatBytes(client.total) : '∞' }}</td>
              <td class="px-4 py-3 text-gray-600">{{ client.expiryTime ? formatDate(client.expiryTime) : '∞' }}</td>
              <td class="px-4 py-3">
                <span :class="client.enable ? 'badge-green' : 'badge-red'" class="badge">
                  {{ client.enable ? 'Active' : 'Disabled' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { adminClientsApi } from '@/api'
import type { Inbound, ClientTraffic } from '@/types'
import { useAppStore } from '@/stores/app'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const appStore = useAppStore()
const loading = ref(true)
const search = ref('')
const allClients = ref<ClientTraffic[]>([])

const filteredClients = computed(() =>
  search.value
    ? allClients.value.filter((c) => c.email?.toLowerCase().includes(search.value.toLowerCase()))
    : allClients.value
)

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDate(ms: number): string {
  return new Date(ms).toLocaleDateString()
}

onMounted(async () => {
  try {
    const inbounds: Inbound[] = await adminClientsApi.list()
    allClients.value = inbounds.flatMap((ib) => ib.clientStats ?? [])
  } catch {
    appStore.error('Failed to load clients')
  } finally {
    loading.value = false
  }
})
</script>
