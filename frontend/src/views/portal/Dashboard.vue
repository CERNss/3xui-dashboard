<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { portalTrafficApi, type ClientUsage } from '@/api/portal/traffic'
import { portalProfileApi, type UserProfile } from '@/api/portal/profile'

const clients = ref<ClientUsage[]>([])
const profile = ref<UserProfile | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

async function reload() {
  loading.value = true
  error.value = null
  try {
    const [p, c] = await Promise.all([portalProfileApi.get(), portalTrafficApi.own()])
    profile.value = p
    clients.value = c
  } catch (e: any) {
    error.value = e?.message ?? 'Failed to load'
  } finally {
    loading.value = false
  }
}

function formatBytes(n: number): string {
  if (n < 1024) return n + ' B'
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return v.toFixed(2) + ' ' + units[i]
}

const subURL = computed(() => {
  if (!profile.value) return ''
  return location.origin + '/sub/' + profile.value.sub_id
})

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-6 flex items-center justify-between">
      <h1 class="text-xl font-semibold">{{ $t('nav.dashboard') }}</h1>
      <button class="rounded-md border border-surface-300 px-3 py-1.5 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="reload">Refresh</button>
    </header>

    <p v-if="error" class="mb-4 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <section v-else class="space-y-6">
      <div v-if="profile" class="rounded-lg border border-surface-200 bg-surface-0 p-4 shadow-card dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-sm font-semibold text-surface-700 dark:text-surface-300">Subscription URL</h2>
        <p class="mt-2 break-all font-mono text-xs text-surface-600 dark:text-surface-300">{{ subURL }}</p>
        <p class="mt-2 text-xs text-surface-500">Paste this into your Xray-family client (V2RayN, Stash, sing-box, Clash Verge with sub-converter, …).</p>
      </div>

      <div class="rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900">
        <header class="border-b border-surface-200 px-4 py-3 text-sm font-semibold dark:border-surface-800">Your clients</header>
        <table class="min-w-full divide-y divide-surface-200 text-sm dark:divide-surface-800">
          <thead class="bg-surface-50 text-left text-xs uppercase tracking-wide text-surface-500 dark:bg-surface-800/40">
            <tr>
              <th class="px-4 py-2">Node</th>
              <th class="px-4 py-2">Inbound</th>
              <th class="px-4 py-2">Up</th>
              <th class="px-4 py-2">Down</th>
              <th class="px-4 py-2">Total</th>
              <th class="px-4 py-2">Expires</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-surface-200 dark:divide-surface-800">
            <tr v-for="c in clients" :key="c.node_id + ':' + c.inbound_tag">
              <td class="px-4 py-2">{{ c.node_id }}</td>
              <td class="px-4 py-2 font-mono text-xs">{{ c.inbound_tag }}</td>
              <td class="px-4 py-2 tabular-nums">{{ formatBytes(c.up) }}</td>
              <td class="px-4 py-2 tabular-nums">{{ formatBytes(c.down) }}</td>
              <td class="px-4 py-2 tabular-nums">{{ formatBytes(c.total) }}</td>
              <td class="px-4 py-2 text-xs">{{ c.expires_at ? new Date(c.expires_at).toLocaleString() : '—' }}</td>
            </tr>
            <tr v-if="clients.length === 0">
              <td colspan="6" class="px-4 py-10 text-center text-surface-500">No active clients yet.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
