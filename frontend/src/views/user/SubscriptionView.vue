<template>
  <div class="space-y-6 animate-fade-in max-w-2xl">
    <div>
      <h2 class="text-lg font-semibold text-gray-900">Subscription</h2>
      <p class="text-sm text-gray-500">Your subscription link and QR code</p>
    </div>

    <div v-if="loading" class="card flex justify-center py-12">
      <LoadingSpinner size="lg" />
    </div>

    <template v-else-if="sub">
      <!-- Sub URL -->
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-3">Subscription Link</h3>
        <div class="flex gap-2">
          <input :value="sub.subUrl" class="input flex-1 font-mono text-xs" readonly />
          <button class="btn-primary whitespace-nowrap" @click="copyLink">
            {{ copied ? '✓ Copied!' : 'Copy' }}
          </button>
        </div>
        <p class="text-xs text-gray-400 mt-2">Client email: {{ sub.email || '—' }}</p>
      </div>

      <!-- QR Code -->
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-4">QR Code</h3>
        <div class="flex justify-center">
          <canvas ref="qrCanvas" class="rounded-xl border border-gray-200 p-2" />
        </div>
        <p class="text-xs text-gray-400 text-center mt-3">Scan with your VPN client app</p>
      </div>

      <!-- Supported clients -->
      <div class="card">
        <h3 class="font-semibold text-gray-700 mb-4">Supported Clients</h3>
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <a
            v-for="client in supportedClients"
            :key="client.name"
            :href="client.url"
            target="_blank"
            rel="noopener"
            class="flex items-center gap-2 p-3 rounded-xl border border-gray-200 hover:border-primary-300 hover:bg-primary-50 transition-colors text-sm font-medium text-gray-700"
          >
            <span class="text-xl">{{ client.icon }}</span>
            {{ client.name }}
          </a>
        </div>
      </div>
    </template>

    <div v-else class="card text-center py-12">
      <p class="text-gray-500">No subscription linked to your account.</p>
      <p class="text-sm text-gray-400 mt-1">Contact an administrator to link your XUI subscription.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
import QRCode from 'qrcode'
import { userSubscriptionApi } from '@/api'
import type { SubscriptionInfo } from '@/types'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const sub = ref<SubscriptionInfo | null>(null)
const loading = ref(true)
const copied = ref(false)
const qrCanvas = ref<HTMLCanvasElement | null>(null)

const supportedClients = [
  { name: 'v2rayNG', icon: '📱', url: 'https://github.com/2dust/v2rayNG' },
  { name: 'Clash', icon: '🔀', url: 'https://github.com/Dreamacro/clash' },
  { name: 'Shadowrocket', icon: '🚀', url: 'https://apps.apple.com/us/app/shadowrocket/id932747118' },
  { name: 'Quantumult X', icon: '⚡', url: 'https://quantumult.app' },
  { name: 'Hiddify', icon: '🛡️', url: 'https://github.com/hiddify/hiddify-next' },
  { name: 'Sing-box', icon: '📦', url: 'https://github.com/SagerNet/sing-box' }
]

async function copyLink() {
  if (!sub.value) return
  await navigator.clipboard.writeText(sub.value.subUrl)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2500)
}

async function renderQR() {
  if (!qrCanvas.value || !sub.value) return
  await QRCode.toCanvas(qrCanvas.value, sub.value.subUrl, {
    width: 240,
    margin: 2,
    color: { dark: '#111827', light: '#ffffff' }
  })
}

onMounted(async () => {
  try {
    sub.value = await userSubscriptionApi.get()
    await nextTick()
    await renderQR()
  } catch {
    // no subscription
  } finally {
    loading.value = false
  }
})

watch(sub, async () => {
  await nextTick()
  await renderQR()
})
</script>
