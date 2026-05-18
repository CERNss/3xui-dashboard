<template>
  <div class="space-y-6 animate-fade-in">
    <div>
      <h2 class="text-lg font-semibold text-gray-900">My Dashboard</h2>
      <p class="text-sm text-gray-500">Welcome back, {{ auth.user?.username }}</p>
    </div>

    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div v-for="i in 2" :key="i" class="card animate-pulse">
        <div class="h-4 bg-gray-200 rounded w-1/2 mb-3"></div>
        <div class="h-8 bg-gray-200 rounded w-1/3"></div>
      </div>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <!-- Traffic gauge -->
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-4">Traffic Usage</h3>
        <div v-if="traffic" class="flex flex-col items-center">
          <!-- Circular progress -->
          <div class="relative w-32 h-32">
            <svg class="w-32 h-32 -rotate-90" viewBox="0 0 128 128">
              <circle cx="64" cy="64" r="54" fill="none" stroke="#e5e7eb" stroke-width="12" />
              <circle
                cx="64" cy="64" r="54" fill="none"
                :stroke="usagePercent > 80 ? '#ef4444' : usagePercent > 60 ? '#f59e0b' : '#3b82f6'"
                stroke-width="12"
                stroke-linecap="round"
                :stroke-dasharray="circumference"
                :stroke-dashoffset="circumference - (circumference * Math.min(usagePercent, 100)) / 100"
                class="transition-all duration-700"
              />
            </svg>
            <div class="absolute inset-0 flex flex-col items-center justify-center">
              <span class="text-2xl font-bold text-gray-900">{{ usagePercent.toFixed(0) }}%</span>
              <span class="text-xs text-gray-500">used</span>
            </div>
          </div>
          <div class="mt-4 text-center">
            <p class="text-sm text-gray-600">
              {{ formatBytes(traffic.up + traffic.down) }}
              <span class="text-gray-400">/ {{ traffic.total ? formatBytes(traffic.total) : '∞' }}</span>
            </p>
            <p class="text-xs text-gray-400 mt-0.5">↑ {{ formatBytes(traffic.up) }} · ↓ {{ formatBytes(traffic.down) }}</p>
          </div>
        </div>
        <div v-else class="text-center py-6 text-gray-400 text-sm">
          No traffic data available.<br />
          <span class="text-xs">Link your XUI account in your profile.</span>
        </div>
      </div>

      <!-- Expiry / Status -->
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-4">Account Status</h3>
        <div v-if="traffic" class="space-y-4">
          <div class="flex items-center justify-between">
            <span class="text-sm text-gray-500">Status</span>
            <span :class="traffic.enable ? 'badge-green' : 'badge-red'" class="badge">
              {{ traffic.enable ? 'Active' : 'Disabled' }}
            </span>
          </div>
          <div v-if="traffic.expiryTime" class="flex items-center justify-between">
            <span class="text-sm text-gray-500">Expires</span>
            <span class="text-sm font-medium" :class="daysLeft < 7 ? 'text-red-600' : 'text-gray-800'">
              {{ daysLeft > 0 ? `${daysLeft} days` : 'Expired' }}
            </span>
          </div>
          <div v-else class="flex items-center justify-between">
            <span class="text-sm text-gray-500">Expires</span>
            <span class="text-sm font-medium text-green-600">Never</span>
          </div>
        </div>
        <div v-else class="py-4 text-sm text-gray-400 text-center">No data</div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="card">
      <h3 class="font-semibold text-gray-700 mb-4">Quick Actions</h3>
      <div class="flex flex-wrap gap-3">
        <router-link to="/user/subscription" class="btn-primary">
          View Subscription
        </router-link>
        <router-link to="/user/profile" class="btn-secondary">
          Edit Profile
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { userTrafficApi } from '@/api'
import type { ClientTraffic } from '@/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const traffic = ref<ClientTraffic | null>(null)
const loading = ref(true)
const circumference = 2 * Math.PI * 54

const usagePercent = computed(() => {
  if (!traffic.value || !traffic.value.total) return 0
  return ((traffic.value.up + traffic.value.down) / traffic.value.total) * 100
})

const daysLeft = computed(() => {
  if (!traffic.value?.expiryTime) return Infinity
  return Math.max(0, Math.ceil((traffic.value.expiryTime - Date.now()) / 86400000))
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`
}

onMounted(async () => {
  try {
    traffic.value = await userTrafficApi.get()
  } catch {
    // no traffic linked — acceptable
  } finally {
    loading.value = false
  }
})
</script>
