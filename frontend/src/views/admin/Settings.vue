<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { formatError } from '@/utils/format'

import { settingsApi, type SettingItem } from '@/api/admin/settings'
import { adminPlansApi, type AdminPlan } from '@/api/admin/plans'
import { useBrandingStore } from '@/stores/branding'

const { t, locale } = useI18n()
const branding = useBrandingStore()

const PUBLIC_REGISTRATION_KEY = 'public_registration_enabled'
const EMAIL_VERIFICATION_REQUIRED_KEY = 'email_verification_required'
const EMAIL_DOMAIN_ALLOWLIST_KEY = 'email_domain_allowlist'
const NEW_USER_INITIAL_BALANCE_KEY = 'new_user_initial_balance_cents'
const NEW_USER_PLAN_IDS_KEY = 'new_user_plan_ids'
const REGISTRATION_SETTINGS_KEYS = [
  PUBLIC_REGISTRATION_KEY,
  EMAIL_VERIFICATION_REQUIRED_KEY,
  EMAIL_DOMAIN_ALLOWLIST_KEY,
] as const
const OIDC_KEYS = [
  'oidc_issuer',
  'oidc_client_id',
  'oidc_client_secret',
  'oidc_redirect_url',
  'oidc_scopes',
  'oidc_display_name',
  'oidc_icon_url',
  'oidc_auth_url',
  'oidc_token_url',
  'oidc_jwks_url',
  'oidc_userinfo_url',
] as const
const OIDC_REQUIRED_KEYS = ['oidc_issuer', 'oidc_client_id', 'oidc_client_secret', 'oidc_redirect_url'] as const

const items = ref<SettingItem[]>([])
const plans = ref<AdminPlan[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const savingKey = ref<string | null>(null)
const savingNewUserPolicy = ref(false)
const savingOIDC = ref(false)
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
const iconFile = ref<File | null>(null)
const iconPreview = ref('')
const iconBusy = ref(false)
const iconFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
const newUserInitialBalanceYuan = ref('0.00')
const starterPlanIDs = ref<number[]>([])
const oidcDrafts = ref<Record<string, string>>({})
const domainInput = ref('')
const savingRegistrationKey = ref<string | null>(null)

// Mutable copy of values keyed by setting key — keeps the form
// editable without re-fetching after each Save.
const drafts = ref<Record<string, string>>({})

async function load() {
  loading.value = true
  error.value = null
  try {
    const [settings, planRows] = await Promise.all([settingsApi.list(), adminPlansApi.list()])
    items.value = settings
    plans.value = planRows
    items.value.forEach((it) => {
      drafts.value[it.key] = it.value
    })
    newUserInitialBalanceYuan.value = centsToYuan(drafts.value[NEW_USER_INITIAL_BALANCE_KEY] || '0')
    starterPlanIDs.value = parsePlanIDs(drafts.value[NEW_USER_PLAN_IDS_KEY] || '')
    OIDC_KEYS.forEach((key) => {
      oidcDrafts.value[key] = drafts.value[key] || ''
    })
  } catch (e: any) {
    error.value = formatError(e, t('admin.settings.loadFailed'))
  } finally {
    loading.value = false
  }
}

function onIconPicked(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0] ?? null
  iconFile.value = file
  iconFlash.value = null
  if (iconPreview.value) URL.revokeObjectURL(iconPreview.value)
  iconPreview.value = file ? URL.createObjectURL(file) : ''
}

async function uploadIcon() {
  if (!iconFile.value) return
  iconBusy.value = true
  iconFlash.value = null
  try {
    const res = await settingsApi.uploadBrandIcon(iconFile.value)
    branding.setIcon(res.url)
    iconFile.value = null
    if (iconPreview.value) URL.revokeObjectURL(iconPreview.value)
    iconPreview.value = ''
    iconFlash.value = { kind: 'ok', text: t('admin.settings.branding.uploaded') }
    await load()
  } catch (e: any) {
    iconFlash.value = { kind: 'err', text: formatError(e, t('admin.settings.branding.uploadFailed')) }
  } finally {
    iconBusy.value = false
  }
}

// Prefer the localized label in flash text so users in zh locale
// see "余额阈值 已保存" instead of "Low balance threshold 已保存".
function localizedLabel(it: SettingItem): string {
  return locale.value === 'zh' && it.label_zh ? it.label_zh : it.label
}

async function save(it: SettingItem) {
  savingKey.value = it.key
  flash.value = null
  try {
    const draft = drafts.value[it.key] ?? ''
    await settingsApi.set(it.key, draft)
    flash.value = { kind: 'ok', text: t('admin.settings.savedFlash', { label: localizedLabel(it) }) }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, t('admin.settings.saveFailed')) }
  } finally {
    savingKey.value = null
  }
}

async function saveNewUserPolicy() {
  const parsed = Number(newUserInitialBalanceYuan.value || '0')
  if (!Number.isFinite(parsed) || parsed < 0) {
    flash.value = { kind: 'err', text: t('admin.settings.newUserPolicy.invalidBalance') }
    return
  }
  savingNewUserPolicy.value = true
  flash.value = null
  try {
    const cents = String(Math.round(parsed * 100))
    const planIDs = [...new Set(starterPlanIDs.value)].sort((a, b) => a - b).join(',')
    await Promise.all([
      settingsApi.set(NEW_USER_INITIAL_BALANCE_KEY, cents),
      settingsApi.set(NEW_USER_PLAN_IDS_KEY, planIDs),
    ])
    flash.value = { kind: 'ok', text: t('admin.settings.newUserPolicy.saved') }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, t('admin.settings.saveFailed')) }
  } finally {
    savingNewUserPolicy.value = false
  }
}

async function saveOIDCSettings() {
  savingOIDC.value = true
  flash.value = null
  try {
    await Promise.all(OIDC_KEYS.map((key) => settingsApi.set(key, oidcDrafts.value[key] ?? '')))
    flash.value = { kind: 'ok', text: t('admin.settings.oidc.saved') }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, t('admin.settings.saveFailed')) }
  } finally {
    savingOIDC.value = false
  }
}

function parsePlanIDs(raw: string): number[] {
  const seen = new Set<number>()
  raw.split(',').forEach((part) => {
    const id = Number(part.trim())
    if (Number.isInteger(id) && id > 0) seen.add(id)
  })
  return [...seen]
}

function centsToYuan(raw: string): string {
  const cents = Number(raw || '0')
  if (!Number.isFinite(cents) || cents <= 0) return '0.00'
  return (Math.round(cents) / 100).toFixed(2)
}

function formatYuan(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

function settingFallback(key: string): string {
  return items.value.find((row) => row.key === key)?.env_fallback || ''
}

function settingItem(key: string): SettingItem | undefined {
  return items.value.find((row) => row.key === key)
}

function effectiveSettingValue(key: string, fallback = ''): string {
  const draft = drafts.value[key]
  if (draft !== undefined && draft !== '') return draft
  const it = settingItem(key)
  return it?.env_fallback || it?.default || fallback
}

function effectiveSettingBool(key: string, fallback = false): boolean {
  const raw = effectiveSettingValue(key, fallback ? 'true' : 'false').trim().toLowerCase()
  return raw === 'true' || raw === '1' || raw === 'yes' || raw === 'on'
}

function parseDomainAllowlist(raw: string): string[] {
  const seen = new Set<string>()
  raw.split(',').forEach((part) => {
    const domain = normalizeDomain(part)
    if (domain) seen.add(domain)
  })
  return [...seen]
}

function normalizeDomain(raw: string): string {
  return raw.trim().replace(/^@+/, '').replace(/[,，]+$/g, '').toLowerCase()
}

const emailDomainTags = computed(() => parseDomainAllowlist(drafts.value[EMAIL_DOMAIN_ALLOWLIST_KEY] || ''))

async function saveRegistrationSetting(key: string, value: string) {
  savingRegistrationKey.value = key
  flash.value = null
  try {
    drafts.value[key] = value
    await settingsApi.set(key, value)
    flash.value = { kind: 'ok', text: t('admin.settings.registration.saved') }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, t('admin.settings.saveFailed')) }
  } finally {
    savingRegistrationKey.value = null
  }
}

async function setRegistrationBool(key: string, value: boolean) {
  await saveRegistrationSetting(key, value ? 'true' : 'false')
}

async function addDomainInput() {
  const parts = domainInput.value.split(/[\s,，]+/).map(normalizeDomain).filter(Boolean)
  if (parts.length === 0) return
  const merged = new Set([...emailDomainTags.value, ...parts])
  domainInput.value = ''
  await saveRegistrationSetting(EMAIL_DOMAIN_ALLOWLIST_KEY, [...merged].join(','))
}

function onDomainInputKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',') {
    event.preventDefault()
    void addDomainInput()
  }
}

async function removeDomain(domain: string) {
  const next = emailDomainTags.value.filter((item) => item !== domain)
  await saveRegistrationSetting(EMAIL_DOMAIN_ALLOWLIST_KEY, next.join(','))
}

async function clearOverride(it: SettingItem) {
  if (!it.has_override) return
  savingKey.value = it.key
  flash.value = null
  try {
    await settingsApi.clear(it.key)
    flash.value = { kind: 'ok', text: t('admin.settings.revertedFlash', { label: localizedLabel(it) }) }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, t('admin.settings.resetDefaultsFailed')) }
  } finally {
    savingKey.value = null
  }
}

const grouped = computed(() => {
  const buckets: Record<string, SettingItem[]> = {}
  for (const it of items.value) {
    if (it.key === NEW_USER_INITIAL_BALANCE_KEY || it.key === NEW_USER_PLAN_IDS_KEY) continue
    if ((REGISTRATION_SETTINGS_KEYS as readonly string[]).includes(it.key)) continue
    if ((OIDC_KEYS as readonly string[]).includes(it.key)) continue
    const g = it.group || 'other'
    buckets[g] = buckets[g] || []
    buckets[g].push(it)
  }
  return buckets
})

const generalGrouped = computed(() => {
  const buckets: Record<string, SettingItem[]> = {}
  for (const [group, rows] of Object.entries(grouped.value)) {
    if (group === 'registration') continue
    buckets[group] = rows
  }
  return buckets
})

const oidcStatus = computed(() => {
  const hasAllRequired = OIDC_REQUIRED_KEYS.every((key) => {
    const draft = (oidcDrafts.value[key] || '').trim()
    return draft || settingFallback(key)
  })
  return hasAllRequired ? t('admin.settings.oidc.enabled') : t('admin.settings.oidc.disabled')
})

const starterPlanSummary = computed(() => {
  if (starterPlanIDs.value.length === 0) return t('admin.settings.newUserPolicy.allPlans')
  const names = plans.value
    .filter((p) => starterPlanIDs.value.includes(p.id))
    .map((p) => p.name)
  return names.length > 0 ? names.join(', ') : t('admin.settings.newUserPolicy.noMatchingPlans')
})

function groupLabel(g: string): string {
  return ({
    registration: t('admin.settings.groupRegistration'),
    subscription: t('admin.settings.groupSubscription'),
    traffic: t('admin.settings.groupTraffic'),
    other: t('admin.settings.groupOther'),
  } as Record<string, string>)[g] ?? g
}

// SMTP test bench state — ops affordance, not part of the settings
// repo (no DB row, just a one-shot send).
const smtpTo = ref('')
const smtpBusy = ref(false)
const smtpFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

async function sendSMTPTest() {
  smtpBusy.value = true
  smtpFlash.value = null
  try {
    await settingsApi.smtpTest(smtpTo.value)
    smtpFlash.value = { kind: 'ok', text: t('admin.settings.smtpSendOk', { to: smtpTo.value }) }
  } catch (e: any) {
    smtpFlash.value = { kind: 'err', text: formatError(e, t('admin.settings.smtpFailed')) }
  } finally {
    smtpBusy.value = false
  }
}

// Tab state — runtime settings are split by operator intent:
// general overrides, security/auth, signup defaults, user-facing
// messages (SMTP), and ops-facing notifications.
type Tab = 'general' | 'securityAuth' | 'userDefaults' | 'messages' | 'notifications'
const tabs: Tab[] = ['general', 'securityAuth', 'userDefaults', 'messages', 'notifications']
const tab = ref<Tab>('general')

function tabLabel(target: Tab): string {
  return ({
    general: t('admin.settings.generalTab'),
    securityAuth: t('admin.settings.securityAuthTab'),
    userDefaults: t('admin.settings.userDefaultsTab'),
    messages: t('admin.settings.messagesTab'),
    notifications: t('admin.settings.notificationsTab'),
  } as Record<Tab, string>)[target]
}

onMounted(load)
</script>

<template>
  <div>
    <header class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-xl font-semibold">{{ $t('admin.settings.title') }}</h1>
      </div>
      <button
        class="rounded-md border border-surface-300 px-3 py-1.5 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="load"
      >
        {{ $t('admin.settings.refresh') }}
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
    <p
      v-if="flash"
      :class="[
        'mb-4 rounded px-3 py-2 text-sm',
        flash.kind === 'ok' ? 'bg-accent-50 text-accent-800' : 'bg-red-50 text-red-700',
      ]"
    >{{ flash.text }}</p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <div v-else class="space-y-6">
      <section class="rounded-lg border border-surface-200 bg-surface-0 p-4 shadow-card dark:border-surface-800 dark:bg-surface-900">
        <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div class="flex items-center gap-4">
            <div class="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl bg-accent-500 text-white shadow-card ring-1 ring-accent-700/30">
              <img v-if="iconPreview || branding.iconUrl" :src="iconPreview || branding.iconUrl" alt="" class="h-12 w-12 rounded-xl object-cover" />
              <svg v-else class="h-8 w-8" viewBox="0 0 24 24" fill="currentColor" stroke="none">
                <path d="M13 2 3 14h7l-1 8 11-13h-7l0-7z" />
              </svg>
            </div>
            <div>
              <h2 class="text-base font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.branding.title') }}</h2>
            </div>
          </div>
          <div class="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center">
            <label class="inline-flex h-9 cursor-pointer items-center justify-center rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-200 dark:hover:bg-surface-800">
              <input class="sr-only" type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" @change="onIconPicked" />
              {{ $t('admin.settings.branding.choose') }}
            </label>
            <button
              class="inline-flex h-9 items-center justify-center rounded-xl bg-primary-600 px-3 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-60"
              :disabled="!iconFile || iconBusy"
              @click="uploadIcon"
            >
              {{ iconBusy ? $t('admin.settings.branding.uploading') : $t('admin.settings.branding.upload') }}
            </button>
          </div>
        </div>
        <p
          v-if="iconFlash"
          class="mt-3 rounded px-3 py-2 text-sm"
          :class="iconFlash.kind === 'ok' ? 'bg-accent-50 text-accent-800' : 'bg-red-50 text-red-700'"
        >
          {{ iconFlash.text }}
        </p>
      </section>

      <nav
        class="flex flex-wrap items-center gap-1"
        role="tablist"
        aria-label="Settings surfaces"
      >
        <button
          v-for="t in tabs"
          :key="t"
          type="button"
          role="tab"
          :aria-selected="tab === t"
          :class="[
            'inline-flex items-center gap-1.5 rounded-xl px-3.5 py-1.5 text-sm font-medium transition-all ease-brand active:scale-[0.98]',
            tab === t
              ? 'bg-accent-50 text-accent-700 ring-1 ring-inset ring-accent-500/30 dark:bg-accent-500/15 dark:text-accent-300 dark:ring-accent-500/30'
              : 'text-surface-500 hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-50',
          ]"
          @click="tab = t"
        >
          <svg v-if="t === 'general'" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
          <svg v-else-if="t === 'securityAuth'" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="m9 12 2 2 4-5" /></svg>
          <svg v-else-if="t === 'userDefaults'" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21a8 8 0 0 0-16 0" /><circle cx="12" cy="7" r="4" /><path d="M17 11h4M19 9v4" /></svg>
          <svg v-else-if="t === 'messages'" class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" /><path d="m22 6-10 7L2 6" /></svg>
          <svg v-else class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" /></svg>
          {{ tabLabel(t) }}
        </button>
      </nav>

      <section
        v-show="tab === 'userDefaults'"
        class="rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900"
      >
        <header class="flex flex-col gap-1 border-b border-surface-200 px-4 py-3 dark:border-surface-800 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.newUserPolicy.title') }}</h2>
          </div>
          <span class="rounded-full bg-accent-50 px-2.5 py-1 text-2xs font-medium text-accent-700 ring-1 ring-inset ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800">
            {{ starterPlanSummary }}
          </span>
        </header>
        <div class="grid gap-5 p-4 lg:grid-cols-[minmax(220px,0.55fr),1fr]">
          <div>
            <label class="block text-xs font-medium text-surface-600 dark:text-surface-300" for="new-user-initial-balance">
              {{ $t('admin.settings.newUserPolicy.initialBalance') }}
            </label>
            <div class="mt-2 flex items-center rounded-xl border border-surface-200 bg-surface-0 px-3 focus-within:border-accent-500 focus-within:ring-4 focus-within:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900">
              <span class="text-sm text-surface-400">¥</span>
              <input
                id="new-user-initial-balance"
                v-model="newUserInitialBalanceYuan"
                type="number"
                min="0"
                step="0.01"
                class="h-10 min-w-0 flex-1 border-0 bg-transparent px-2 text-sm tabular-nums text-ink-900 outline-none focus:ring-0 dark:text-surface-50"
              />
            </div>
          </div>

          <div>
            <div class="flex items-start justify-between gap-3">
              <div>
                <h3 class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.newUserPolicy.starterPlans') }}</h3>
              </div>
              <button
                type="button"
                class="shrink-0 rounded-lg border border-surface-200 px-2.5 py-1 text-xs font-medium text-surface-600 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
                @click="starterPlanIDs = []"
              >
                {{ $t('admin.settings.newUserPolicy.allowAll') }}
              </button>
            </div>
            <div class="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              <label
                v-for="p in plans"
                :key="p.id"
                class="flex min-h-16 cursor-pointer items-start gap-2.5 rounded-xl border p-3 transition-all duration-150 ease-brand"
                :class="starterPlanIDs.includes(p.id)
                  ? 'border-accent-300 bg-accent-50 dark:border-accent-700 dark:bg-accent-950/40'
                  : 'border-surface-200 bg-surface-0 hover:border-surface-300 hover:bg-surface-50 dark:border-surface-700 dark:bg-surface-900 dark:hover:bg-surface-800'"
              >
                <input
                  v-model="starterPlanIDs"
                  type="checkbox"
                  :value="p.id"
                  class="mt-0.5 h-4 w-4 rounded border-surface-300 text-accent-600 focus:ring-accent-500/30"
                />
                <span class="min-w-0">
                  <span class="block truncate text-sm font-medium text-ink-900 dark:text-surface-50">{{ p.name }}</span>
                  <span class="mt-0.5 block text-xs text-surface-500">#{{ p.id }} · {{ formatYuan(p.price_cents) }} · {{ p.enabled ? $t('admin.settings.newUserPolicy.enabled') : $t('admin.settings.newUserPolicy.disabled') }}</span>
                </span>
              </label>
            </div>
          </div>
        </div>
        <footer class="flex justify-end border-t border-surface-200 px-4 py-3 dark:border-surface-800">
          <button
            class="inline-flex h-9 items-center rounded-xl bg-primary-600 px-4 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-60"
            :disabled="savingNewUserPolicy"
            @click="saveNewUserPolicy"
          >
            {{ savingNewUserPolicy ? $t('admin.settings.saving') : $t('admin.settings.newUserPolicy.save') }}
          </button>
        </footer>
      </section>

      <section
        v-show="tab === 'securityAuth'"
        class="overflow-hidden rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900"
      >
        <header class="border-b border-surface-200 px-4 py-4 dark:border-surface-800">
          <h2 class="text-base font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.registration.title') }}</h2>
          <p class="mt-1 text-xs text-surface-500">{{ $t('admin.settings.registration.subtitle') }}</p>
        </header>

        <div class="divide-y divide-surface-200 px-4 dark:divide-surface-800">
          <div class="flex items-center justify-between gap-4 py-4">
            <div>
              <h3 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.registration.publicTitle') }}</h3>
              <p class="mt-1 text-xs text-surface-500">{{ $t('admin.settings.registration.publicDesc') }}</p>
            </div>
            <button
              type="button"
              role="switch"
              :aria-checked="effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true)"
              :disabled="savingRegistrationKey === PUBLIC_REGISTRATION_KEY"
              class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors duration-150 ease-brand focus:outline-none focus:ring-4 focus:ring-accent-500/20 disabled:opacity-60"
              :class="effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true) ? 'bg-accent-600' : 'bg-surface-300 dark:bg-surface-700'"
              @click="setRegistrationBool(PUBLIC_REGISTRATION_KEY, !effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true))"
            >
              <span
                class="inline-block h-5 w-5 rounded-full bg-white shadow transition-transform duration-150 ease-brand"
                :class="effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true) ? 'translate-x-5' : 'translate-x-0.5'"
              ></span>
            </button>
          </div>

          <div class="flex items-center justify-between gap-4 py-4">
            <div>
              <h3 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.registration.verifyTitle') }}</h3>
              <p class="mt-1 text-xs text-surface-500">{{ $t('admin.settings.registration.verifyDesc') }}</p>
            </div>
            <button
              type="button"
              role="switch"
              :aria-checked="effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false)"
              :disabled="savingRegistrationKey === EMAIL_VERIFICATION_REQUIRED_KEY"
              class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors duration-150 ease-brand focus:outline-none focus:ring-4 focus:ring-accent-500/20 disabled:opacity-60"
              :class="effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false) ? 'bg-accent-600' : 'bg-surface-300 dark:bg-surface-700'"
              @click="setRegistrationBool(EMAIL_VERIFICATION_REQUIRED_KEY, !effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false))"
            >
              <span
                class="inline-block h-5 w-5 rounded-full bg-white shadow transition-transform duration-150 ease-brand"
                :class="effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false) ? 'translate-x-5' : 'translate-x-0.5'"
              ></span>
            </button>
          </div>

          <div class="py-4">
            <h3 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.registration.allowlistTitle') }}</h3>
            <p class="mt-1 text-xs text-surface-500">{{ $t('admin.settings.registration.allowlistDesc') }}</p>
            <div
              class="mt-3 flex min-h-12 flex-wrap items-center gap-2 rounded-xl border border-surface-300 bg-surface-50 px-3 py-2 transition-colors focus-within:border-accent-500 focus-within:ring-4 focus-within:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-800/60"
            >
              <span
                v-for="domain in emailDomainTags"
                :key="domain"
                class="inline-flex h-7 items-center gap-1 rounded-md bg-surface-200 px-2 font-mono text-xs text-surface-700 dark:bg-surface-700 dark:text-surface-100"
              >
                @{{ domain }}
                <button
                  type="button"
                  class="rounded text-surface-400 transition-colors hover:text-red-500 focus:outline-none focus:ring-2 focus:ring-red-500/20"
                  :aria-label="$t('admin.settings.registration.removeDomain', { domain })"
                  :disabled="savingRegistrationKey === EMAIL_DOMAIN_ALLOWLIST_KEY"
                  @click="removeDomain(domain)"
                >
                  ×
                </button>
              </span>
              <input
                v-model="domainInput"
                type="text"
                class="h-7 min-w-[180px] flex-1 border-0 bg-transparent p-0 text-sm text-ink-900 outline-none placeholder:text-surface-400 focus:ring-0 dark:text-surface-50"
                :placeholder="$t('admin.settings.registration.allowlistPlaceholder')"
                :disabled="savingRegistrationKey === EMAIL_DOMAIN_ALLOWLIST_KEY"
                @keydown="onDomainInputKeydown"
                @blur="addDomainInput"
              />
            </div>
            <p class="mt-2 text-xs text-surface-500">{{ $t('admin.settings.registration.allowlistHint') }}</p>
          </div>
        </div>
      </section>

      <section
        v-show="tab === 'securityAuth'"
        class="rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900"
      >
        <header class="flex flex-col gap-1 border-b border-surface-200 px-4 py-3 dark:border-surface-800 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.oidc.title') }}</h2>
          </div>
          <span
            class="rounded-full px-2.5 py-1 text-2xs font-medium ring-1 ring-inset"
            :class="oidcStatus === $t('admin.settings.oidc.enabled')
              ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
              : 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700'"
          >
            {{ oidcStatus }}
          </span>
        </header>

        <div class="grid gap-5 p-4 xl:grid-cols-[1fr,0.85fr]">
          <div class="space-y-4">
            <div class="grid gap-3 md:grid-cols-2">
              <label class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.issuer') }}</span>
                <input
                  v-model="oidcDrafts.oidc_issuer"
                  type="url"
                  placeholder="https://auth.example.com"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.redirectUrl') }}</span>
                <input
                  v-model="oidcDrafts.oidc_redirect_url"
                  type="url"
                  placeholder="https://panel.example.com/oidc/callback"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.clientId') }}</span>
                <input
                  v-model="oidcDrafts.oidc_client_id"
                  type="text"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.clientSecret') }}</span>
                <input
                  v-model="oidcDrafts.oidc_client_secret"
                  type="password"
                  autocomplete="new-password"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.scopes') }}</span>
                <input
                  v-model="oidcDrafts.oidc_scopes"
                  type="text"
                  placeholder="openid,profile,email"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
              <label class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.displayName') }}</span>
                <input
                  v-model="oidcDrafts.oidc_display_name"
                  type="text"
                  placeholder="Company SSO"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
              <label class="block md:col-span-2">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('admin.settings.oidc.iconUrl') }}</span>
                <input
                  v-model="oidcDrafts.oidc_icon_url"
                  type="url"
                  placeholder="https://cdn.example.com/sso.svg"
                  class="mt-2 h-10 w-full rounded-xl border border-surface-200 bg-surface-0 px-3 text-sm focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
            </div>
          </div>

          <div class="rounded-xl border border-surface-200 bg-surface-50/70 p-3 dark:border-surface-800 dark:bg-surface-950/30">
            <h3 class="text-xs font-semibold uppercase tracking-eyebrow text-surface-500">{{ $t('admin.settings.oidc.advancedTitle') }}</h3>
            <div class="mt-3 space-y-3">
              <label v-for="field in [
                ['oidc_auth_url', $t('admin.settings.oidc.authUrl')],
                ['oidc_token_url', $t('admin.settings.oidc.tokenUrl')],
                ['oidc_jwks_url', $t('admin.settings.oidc.jwksUrl')],
                ['oidc_userinfo_url', $t('admin.settings.oidc.userinfoUrl')],
              ]" :key="field[0]" class="block">
                <span class="text-xs font-medium text-surface-600 dark:text-surface-300">{{ field[1] }}</span>
                <input
                  v-model="oidcDrafts[field[0]]"
                  type="url"
                  class="mt-1.5 h-9 w-full rounded-lg border border-surface-200 bg-surface-0 px-2.5 text-xs focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
                />
              </label>
            </div>
          </div>
        </div>

        <footer class="flex justify-end border-t border-surface-200 px-4 py-3 dark:border-surface-800">
          <button
            class="inline-flex h-9 items-center rounded-xl bg-primary-600 px-4 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-60"
            :disabled="savingOIDC"
            @click="saveOIDCSettings"
          >
            {{ savingOIDC ? $t('admin.settings.saving') : $t('admin.settings.oidc.save') }}
          </button>
        </footer>
      </section>

      <section
        v-for="(rows, group) in generalGrouped"
        v-show="tab === 'general'"
        :key="group"
        class="rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900"
      >
        <header class="border-b border-surface-200 px-4 py-3 text-sm font-semibold dark:border-surface-800">
          {{ groupLabel(group as string) }}
        </header>
        <div class="divide-y divide-surface-200 dark:divide-surface-800">
          <div v-for="it in rows" :key="it.key" class="px-4 py-4">
            <div class="flex items-start gap-4">
              <div class="flex-1">
                <label class="block text-sm font-medium" :for="'setting-' + it.key">{{ locale === 'zh' && it.label_zh ? it.label_zh : it.label }}</label>
              </div>
              <div class="flex flex-col items-end gap-2">
                <select
                  v-if="it.type === 'bool'"
                  :id="'setting-' + it.key"
                  v-model="drafts[it.key]"
                  class="w-40 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
                >
                  <option value="">{{ $t('admin.settings.useDefaultEmpty') }}</option>
                  <option value="true">{{ $t('admin.settings.values.true') }}</option>
                  <option value="false">{{ $t('admin.settings.values.false') }}</option>
                </select>
                <input
                  v-else-if="it.type === 'int'"
                  :id="'setting-' + it.key"
                  v-model="drafts[it.key]"
                  type="number"
                  class="w-40 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
                />
                <input
                  v-else
                  :id="'setting-' + it.key"
                  v-model="drafts[it.key]"
                  type="text"
                  class="w-60 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
                />
                <div class="flex gap-2">
                  <button
                    class="rounded bg-primary-600 px-3 py-1 text-xs font-medium text-white hover:bg-primary-700 disabled:opacity-60"
                    :disabled="savingKey === it.key"
                    @click="save(it)"
                  >
                    {{ savingKey === it.key ? $t('admin.settings.saving') : $t('admin.settings.save') }}
                  </button>
                  <button
                    v-if="it.has_override"
                    class="rounded border border-surface-300 px-3 py-1 text-xs hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
                    :disabled="savingKey === it.key"
                    @click="clearOverride(it)"
                  >
                    {{ $t('admin.settings.reset') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section
        v-show="tab === 'messages'"
        class="rounded-lg border border-surface-200 bg-surface-0 p-4 dark:border-surface-700 dark:bg-surface-900"
      >
        <h3 class="text-base font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.messages.title') }}</h3>
        <div class="mt-4">
          <h4 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.smtpTestTitle') }}</h4>
          <p class="mt-1 text-xs text-surface-500">{{ $t('admin.settings.smtpHint') }}</p>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <input
              v-model="smtpTo"
              type="email"
              placeholder="admin@example.com"
              class="w-72 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
            />
            <button
              class="rounded bg-primary-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-primary-700 disabled:opacity-60"
              :disabled="smtpBusy || !smtpTo"
              @click="sendSMTPTest"
            >
              {{ smtpBusy ? $t('admin.settings.smtpSending') : $t('admin.settings.smtpSendBtn') }}
            </button>
            <span v-if="smtpFlash" class="text-xs" :class="smtpFlash.kind === 'ok' ? 'text-accent-600' : 'text-red-600'">{{ smtpFlash.text }}</span>
          </div>
        </div>
      </section>

      <section
        v-show="tab === 'notifications'"
        class="rounded-lg border border-surface-200 bg-surface-0 p-4 dark:border-surface-700 dark:bg-surface-900"
      >
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <h3 class="text-base font-semibold text-ink-900 dark:text-surface-50">{{ $t('admin.settings.notifications.title') }}</h3>
          <RouterLink
            to="/admin/webhooks"
            class="inline-flex h-9 items-center justify-center rounded-xl bg-primary-600 px-3 text-sm font-medium text-white transition-colors hover:bg-primary-700"
          >
            {{ $t('admin.settings.notifications.webhookLink') }}
          </RouterLink>
        </div>
      </section>
    </div>
  </div>
</template>
