<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import { useI18n } from 'vue-i18n'

import { portalProfileApi, type UserProfile } from '@/api/portal/profile'
import { portalTrafficApi } from '@/api/portal/traffic'
import ConfirmModal from '@/components/common/ConfirmModal.vue'
import { useConfirm } from '@/composables/useConfirm'
import { formatError } from '@/utils/format'

const { t } = useI18n()
const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()

type Format = 'base64' | 'json' | 'clash' | 'singbox' | 'sip008' | 'wireguard' | 'wireguard-zip'

interface FormatInfo {
  key: Format
  label: string
  hint: string
  apps: string  // suggested apps for this format
  // downloadOnly: true → suppress copy/QR (binary or attach-only)
  downloadOnly?: boolean
}

// Computed so locale switches rebuild the labels at render time.
const formats = computed<FormatInfo[]>(() => [
  { key: 'base64',         label: 'Base64',    hint: t('portal.subscription.formats.base64.hint'),       apps: t('portal.subscription.formats.base64.apps') },
  { key: 'clash',          label: 'Clash',     hint: t('portal.subscription.formats.clash.hint'),        apps: t('portal.subscription.formats.clash.apps') },
  { key: 'singbox',        label: 'Sing-box',  hint: t('portal.subscription.formats.singbox.hint'),      apps: t('portal.subscription.formats.singbox.apps') },
  { key: 'sip008',         label: 'SIP008',    hint: t('portal.subscription.formats.sip008.hint'),       apps: t('portal.subscription.formats.sip008.apps') },
  { key: 'wireguard',      label: 'WireGuard', hint: t('portal.subscription.formats.wireguard.hint'),    apps: t('portal.subscription.formats.wireguard.apps') },
  { key: 'wireguard-zip',  label: 'WG (ZIP)',  hint: t('portal.subscription.formats.wireguardZip.hint'), apps: t('portal.subscription.formats.wireguardZip.apps'), downloadOnly: true },
  { key: 'json',           label: 'JSON',      hint: t('portal.subscription.formats.json.hint'),         apps: t('portal.subscription.formats.json.apps') },
])

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

const activeFormatInfo = computed(() => formats.value.find((f) => f.key === activeFormat.value))

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
    error.value = formatError(e, t('portal.subscription.loadFailed'))
  } finally {
    loading.value = false
  }
}

// Monotonic token so an in-flight generate that finishes after a
// newer click can't overwrite the current QR. Each invocation
// captures its own token; only the latest writes.
let qrToken = 0

async function regenerateQR() {
  if (!subURL.value) return
  const my = ++qrToken
  // Clear immediately so the user sees the placeholder during regen
  // instead of a stale QR with the wrong format label below it.
  qrDataURL.value = ''
  // Download-only formats don't roundtrip through QR — the binary
  // body can't be scanned and the URL is the same one the download
  // button already exposes.
  if (activeFormatInfo.value?.downloadOnly) return
  try {
    const url = await QRCode.toDataURL(subURL.value, {
      width: 260,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#0c0e12', light: '#ffffff' },
    })
    if (my === qrToken) qrDataURL.value = url
  } catch {
    if (my === qrToken) qrDataURL.value = ''
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

// Sub URL rotation invalidates the old /sub/<id> immediately, so keep it
// behind the same styled confirmation flow used elsewhere in the portal.
const rotating = ref(false)
const rotateErr = ref<string | null>(null)

async function rotateSubID() {
  const ok = await askConfirm({
    title: t('portal.subscription.regenerateTitle'),
    message: t('portal.subscription.regenerateConfirm'),
    variant: 'danger',
    confirmLabel: t('portal.subscription.regenerate'),
  })
  if (!ok) return
  rotating.value = true
  rotateErr.value = null
  try {
    const { sub_id } = await portalProfileApi.rotateSubID()
    if (profile.value) profile.value.sub_id = sub_id
  } catch (e: any) {
    rotateErr.value = formatError(e, t('portal.subscription.regenerateFailed'))
  } finally {
    rotating.value = false
  }
}

onMounted(load)
watch([subURL, activeFormat], regenerateQR, { immediate: true })
</script>

<template>
  <div>
    <header class="mb-7">
      <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.subscription.title') }}</h1>
      <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">{{ $t('portal.subscription.subtitle') }}</p>
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
      <h3 class="mt-3 text-sm font-semibold text-surface-700 dark:text-surface-200">{{ $t('portal.subscription.empty') }}</h3>
      <p class="mt-1 text-xs text-surface-500">{{ $t('portal.subscription.emptyDescription') }}</p>
      <router-link to="/portal/plans" class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500">
        {{ $t('portal.subscription.seePlans') }}
        <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6" /></svg>
      </router-link>
    </div>

    <section v-else-if="profile" class="grid grid-cols-1 gap-5 lg:grid-cols-3">
      <!-- Left: URL + format picker -->
      <div class="lg:col-span-2 space-y-5">
        <!-- Format tabs -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.subscription.formats.title') }}</h2>
          <p class="mt-1 text-xs text-surface-500">{{ $t('portal.subscription.formats.hint') }}</p>
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

        <!-- URL display + copy / download -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <div class="flex items-center justify-between">
            <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ activeFormatInfo?.downloadOnly ? $t('portal.subscription.downloadLink') : $t('portal.subscription.urlTitle') }}</h2>
            <a
              v-if="activeFormatInfo?.downloadOnly"
              :href="subURL"
              download
              class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
            >
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
              {{ $t('portal.subscription.downloadFile') }}
            </a>
            <button
              v-else
              type="button"
              class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] dark:bg-accent-600 dark:hover:bg-accent-500"
              @click="copyURL"
            >
              <svg v-if="!copyOk" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" /></svg>
              <svg v-else class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              {{ copyOk ? $t('portal.subscription.copyOk') : $t('common.copy') }}
            </button>
          </div>
          <p class="mt-3 break-all rounded-xl bg-surface-50 px-3.5 py-3 font-mono text-xs text-surface-600 dark:bg-surface-800 dark:text-surface-300">{{ subURL }}</p>
          <div class="mt-3 flex items-center justify-between gap-3">
            <p class="text-2xs text-surface-500">{{ $t('portal.subscription.rotateNote') }}</p>
            <button
              type="button"
              :disabled="rotating"
              class="shrink-0 rounded-lg border border-amber-200 px-2.5 py-1 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-50 disabled:opacity-50 dark:border-amber-800 dark:text-amber-300 dark:hover:bg-amber-950/40"
              @click="rotateSubID"
            >
              {{ rotating ? $t('portal.subscription.regenerating') : $t('portal.subscription.regenerate') }}
            </button>
          </div>
          <p v-if="rotateErr" class="mt-2 rounded-lg bg-red-50 px-2.5 py-1.5 text-2xs text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ rotateErr }}</p>
        </div>

        <!-- Quick how-to -->
        <div class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
          <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.subscription.howToTitle') }}</h2>
          <ol class="mt-3 space-y-2 text-xs text-surface-600 dark:text-surface-300">
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">1.</span> {{ $t('portal.subscription.howTo1') }}</li>
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">2.</span> {{ $t('portal.subscription.howTo2') }}</li>
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">3.</span> {{ $t('portal.subscription.howTo3') }}</li>
            <li class="flex gap-2"><span class="font-semibold text-accent-700 dark:text-accent-300">4.</span> {{ $t('portal.subscription.howTo4') }}</li>
          </ol>
        </div>
      </div>

      <!-- Right: QR (hidden for download-only formats where scanning makes no sense) -->
      <div v-if="!activeFormatInfo?.downloadOnly" class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.subscription.qrTitle') }}</h2>
        <p class="mt-1 text-xs text-surface-500">{{ $t('portal.subscription.qrHint') }}</p>
        <div class="mt-4 flex aspect-square items-center justify-center rounded-2xl border border-surface-100 bg-surface-50 p-3 dark:border-surface-800 dark:bg-surface-800">
          <img v-if="qrDataURL" :src="qrDataURL" alt="subscription QR" class="h-full w-full rounded-lg" />
          <div v-else class="text-2xs text-surface-400">{{ $t('portal.subscription.generating') }}</div>
        </div>
        <p class="mt-3 text-center text-2xs text-surface-400">{{ $t('portal.subscription.formatLabel', { label: activeFormatInfo?.label ?? '' }) }}</p>
      </div>
      <!-- Download-only fallback: gentle prompt to use the download button -->
      <div v-else class="rounded-2xl border border-surface-100 bg-surface-0 p-5 dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.subscription.downloadOnlyTitle') }}</h2>
        <p class="mt-1 text-xs text-surface-500">{{ $t('portal.subscription.downloadOnlyHint') }}</p>
        <div class="mt-4 flex aspect-square items-center justify-center rounded-2xl border border-surface-100 bg-surface-50 p-6 dark:border-surface-800 dark:bg-surface-800">
          <svg class="h-16 w-16 text-surface-300 dark:text-surface-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
        </div>
        <p class="mt-3 text-center text-2xs text-surface-400">{{ $t('portal.subscription.formatLabel', { label: activeFormatInfo?.label ?? '' }) }}</p>
      </div>
    </section>

    <ConfirmModal
      v-if="confirmState"
      :open="confirmState.open"
      :title="confirmState.title"
      :message="confirmState.message"
      :variant="confirmState.variant"
      :confirm-label="confirmState.confirmLabel"
      :cancel-label="confirmState.cancelLabel"
      :busy="confirmState.busy"
      @confirm="settleConfirm(true)"
      @cancel="settleConfirm(false)"
    />
  </div>
</template>
