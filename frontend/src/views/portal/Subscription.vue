<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'

import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import { portalTrafficApi } from '@/api/portal/traffic'
import { formatError } from '@/utils/format'

type Format = 'base64' | 'json' | 'clash' | 'singbox' | 'sip008'

interface FormatInfo {
  key: Format
  label: string
  hint: string
  apps: string  // suggested apps for this format
}

const formats: FormatInfo[] = [
  { key: 'base64',  label: 'Base64',   hint: '默认链接束',          apps: 'V2RayN · Shadowrocket' },
  { key: 'clash',   label: 'Clash',    hint: '完整 Mihomo 配置',    apps: 'Clash Verge · Mihomo · Stash' },
  { key: 'singbox', label: 'Sing-box', hint: 'sing-box JSON',       apps: 'Sing-box 官方客户端' },
  { key: 'sip008',  label: 'SIP008',   hint: 'Shadowsocks-only',    apps: 'Shadowsocks 原版应用' },
  { key: 'json',    label: 'JSON',     hint: 'Xray 原始 config',    apps: '高级用户 / 自托管 Xray' },
]

const profile = ref<UserProfile | null>(null)
const clientCount = ref(0)
const activeFormat = ref<Format>('base64')
const loading = ref(true)
const error = ref<string | null>(null)
const copyOk = ref(false)
const qrDataURL = ref('')

const subURL = computed(() => {
  if (!profile.value) return ''
  const base = location.origin + '/sub/' + profile.value.sub_id
  return activeFormat.value === 'base64' ? base : base + '?format=' + activeFormat.value
})

async function load() {
  loading.value = true
  error.value = null
  try {
    const [p, clients] = await Promise.all([
      portalProfileApi.get(),
      portalTrafficApi.own(),
    ])
    profile.value = p
    clientCount.value = clients.length
  } catch (e: any) {
    error.value = formatError(e, '加载失败')
  } finally {
    loading.value = false
  }
}

async function regenerateQR() {
  if (!subURL.value) return
  try {
    qrDataURL.value = await QRCode.toDataURL(subURL.value, {
      width: 260,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#0c0e12', light: '#ffffff' },
    })
  } catch {
    qrDataURL.value = ''
  }
}

async function copyURL() {
  if (!subURL.value) return
  try {
    await navigator.clipboard.writeText(subURL.value)
    copyOk.value = true
    setTimeout(() => (copyOk.value = false), 2000)
  } catch {
    // Fallback for non-https / browsers without clipboard API
    const ta = document.createElement('textarea')
    ta.value = subURL.value
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    copyOk.value = true
    setTimeout(() => (copyOk.value = false), 2000)
  }
}

onMounted(load)
watch([subURL, activeFormat], regenerateQR, { immediate: true })
</script>

<template>
  <div>
    <header class="mb-7">
      <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">订阅地址</h1>
      <p class="mt-1.5 text-sm text-surface-500">复制 URL 或扫码 · 多格式按客户端类型自动适配</p>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
    </p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <!-- Empty state: no clients = nothing to subscribe to. Surface the
         purchase CTA so users don't paste an empty URL into their app. -->
    <div
      v-else-if="profile && clientCount === 0"
      class="rounded-2xl border border-surface-100 bg-surface-0 px-6 py-16 text-center dark:border-surface-800 dark:bg-surface-900"
    >
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
        <svg class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2" /><rect x="9" y="9" width="6" height="6" /></svg>
      </div>
      <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">还没有活跃客户端</h3>
      <p class="mt-1 text-xs text-surface-500">订阅 URL 已经生成，但当前没有节点开通 — 先买个套餐</p>
      <router-link to="/portal/plans" class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500">
        去看套餐
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
      </router-link>
    </div>

    <section v-else-if="profile" class="grid grid-cols-1 gap-5 lg:grid-cols-3">
      <!-- Left: URL + format picker -->
      <div class="lg:col-span-2 space-y-5">
        <!-- Format tabs -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">选择格式</h2>
          <p class="mt-1 text-xs text-surface-500">不同客户端用不同格式 — 选错也不要紧，URL 会带上 ?format= 参数自动转</p>
          <div class="mt-4 grid grid-cols-2 gap-2 md:grid-cols-3">
            <button
              v-for="f in formats"
              :key="f.key"
              type="button"
              class="group flex flex-col items-start gap-1 rounded-xl border p-3 text-left transition-all duration-150 ease-brand"
              :class="activeFormat === f.key
                ? 'border-accent-300 bg-accent-50 dark:border-accent-700 dark:bg-accent-950/40'
                : 'border-surface-200 bg-surface-0 hover:border-surface-300 hover:bg-surface-50 dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800'"
              @click="activeFormat = f.key"
            >
              <div class="flex w-full items-center justify-between">
                <span class="text-sm font-semibold" :class="activeFormat === f.key ? 'text-accent-700 dark:text-accent-300' : 'text-ink-900 dark:text-surface-50'">{{ f.label }}</span>
                <svg v-if="activeFormat === f.key" class="h-4 w-4 text-accent-600 dark:text-accent-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              </div>
              <span class="text-2xs text-surface-500">{{ f.hint }}</span>
              <span class="text-2xs text-surface-400">{{ f.apps }}</span>
            </button>
          </div>
        </div>

        <!-- URL display + copy -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-center justify-between">
            <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">订阅 URL</h2>
            <button
              type="button"
              class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
              @click="copyURL"
            >
              <svg v-if="!copyOk" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg>
              <svg v-else class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              {{ copyOk ? '已复制' : '复制' }}
            </button>
          </div>
          <p class="mt-3 break-all rounded-xl bg-surface-50 px-3.5 py-3 font-mono text-xs text-surface-600 dark:bg-surface-800 dark:text-surface-300">{{ subURL }}</p>
        </div>

        <!-- Quick how-to -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">使用方法</h2>
          <ol class="mt-3 space-y-2 text-xs text-surface-600 dark:text-surface-300">
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">1.</span> 选择上方对应你客户端的格式</li>
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">2.</span> 复制 URL，或扫描右侧二维码</li>
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">3.</span> 在客户端选「从 URL 导入」/「Import from URL」</li>
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">4.</span> Mihomo / Clash 也可直接打开链接 — 它会识别 User-Agent</li>
          </ol>
        </div>
      </div>

      <!-- Right: QR -->
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">扫描二维码</h2>
        <p class="mt-1 text-xs text-surface-500">手机客户端直接扫码导入</p>
        <div class="mt-4 flex aspect-square items-center justify-center rounded-2xl border border-surface-100 bg-surface-50 p-3 dark:border-surface-800 dark:bg-surface-800">
          <img v-if="qrDataURL" :src="qrDataURL" alt="subscription QR" class="h-full w-full rounded-lg" />
          <div v-else class="text-2xs text-surface-400">生成中…</div>
        </div>
        <p class="mt-3 text-center text-2xs text-surface-400">{{ formats.find(f => f.key === activeFormat)?.label }} 格式</p>
      </div>
    </section>
  </div>
</template>
