<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { dump as dumpYAML, load as loadYAML } from 'js-yaml'
import { formatError } from '@/utils/format'

import { settingsApi, type SettingItem } from '@/api/admin/settings'
import { adminPlansApi, type AdminPlan } from '@/api/admin/plans'
import { useBrandingStore } from '@/stores/branding'
import Webhooks from '@/views/admin/Webhooks.vue'

const { t, locale } = useI18n()
const branding = useBrandingStore()
const route = useRoute()
const router = useRouter()

const PUBLIC_REGISTRATION_KEY = 'public_registration_enabled'
const EMAIL_VERIFICATION_REQUIRED_KEY = 'email_verification_required'
const EMAIL_DOMAIN_ALLOWLIST_KEY = 'email_domain_allowlist'
const NEW_USER_INITIAL_BALANCE_KEY = 'new_user_initial_balance_cents'
const NEW_USER_PLAN_IDS_KEY = 'new_user_plan_ids'
const BRAND_ICON_KEY = 'brand_icon_url'
const BRAND_TITLE_KEY = 'brand_title'
const BRAND_SUBTITLE_KEY = 'brand_subtitle'
const BRAND_DESCRIPTION_KEY = 'brand_description'
const BRAND_FOOTER_KEY = 'brand_footer'
const BRAND_INFO_KEYS = [BRAND_TITLE_KEY, BRAND_SUBTITLE_KEY, BRAND_DESCRIPTION_KEY, BRAND_FOOTER_KEY] as const
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
const MULTILINE_SETTING_KEYS = ['clash_template_yaml', 'singbox_template_json'] as const
const TEMPLATE_EDITOR_FORMATS: Record<string, 'yaml' | 'json'> = {
  clash_template_yaml: 'yaml',
  singbox_template_json: 'json',
}
const TEMPLATE_PLACEHOLDERS = [
  { raw: '${proxies}', token: '__THREEXUI_TEMPLATE_PROXIES__' },
  { raw: '${proxy_names}', token: '__THREEXUI_TEMPLATE_PROXY_NAMES__' },
  { raw: '${proxy_groups}', token: '__THREEXUI_TEMPLATE_PROXY_GROUPS__' },
] as const
const SUBSCRIPTION_GROUP = 'subscription'
const ALERT_GROUP = 'traffic'
const settingHelpPaths: Record<string, string> = {
  subscription_remark_model: 'admin.settings.settingHelp.subscriptionRemark',
  traffic_warn_pct: 'admin.settings.settingHelp.trafficWarn',
  traffic_critical_pct: 'admin.settings.settingHelp.trafficCritical',
  expiry_warn_days: 'admin.settings.settingHelp.expiryWarn',
  clash_template_yaml: 'admin.settings.settingHelp.clashTemplate',
  singbox_template_json: 'admin.settings.settingHelp.singboxTemplate',
  proxy_group_strategy: 'admin.settings.settingHelp.proxyGroupStrategy',
  rule_providers_enabled: 'admin.settings.settingHelp.ruleProviders',
}

const items = ref<SettingItem[]>([])
const plans = ref<AdminPlan[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const savingKey = ref<string | null>(null)
const savingNewUserPolicy = ref(false)
const savingBrandInfo = ref(false)
const savingOIDC = ref(false)
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
const iconFile = ref<File | null>(null)
const iconPreview = ref('')
const iconBusy = ref(false)
const iconFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
const newUserInitialBalanceYuan = ref('0.00')
const starterPlanIDs = ref<number[]>([])
const brandDrafts = ref<Record<(typeof BRAND_INFO_KEYS)[number], string>>({
  [BRAND_TITLE_KEY]: '',
  [BRAND_SUBTITLE_KEY]: '',
  [BRAND_DESCRIPTION_KEY]: '',
  [BRAND_FOOTER_KEY]: '',
})
const oidcDrafts = ref<Record<string, string>>({})
const domainInput = ref('')
const savingRegistrationKey = ref<string | null>(null)
const formatEditorFlash = ref<Record<string, { kind: 'ok' | 'err'; text: string }>>({})

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
    BRAND_INFO_KEYS.forEach((key) => {
      brandDrafts.value[key] = drafts.value[key] || settingFallbackValue(settings, key)
    })
    OIDC_KEYS.forEach((key) => {
      oidcDrafts.value[key] = drafts.value[key] || ''
    })
  } catch (e: any) {
    error.value = formatError(e, t('admin.settings.loadFailed'))
  } finally {
    loading.value = false
  }
}

function settingFallbackValue(settings: SettingItem[], key: string): string {
  const it = settings.find((row) => row.key === key)
  return it?.env_fallback || it?.default || ''
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

async function saveBrandInfo() {
  savingBrandInfo.value = true
  flash.value = null
  try {
    await Promise.all(BRAND_INFO_KEYS.map((key) => settingsApi.set(key, brandDrafts.value[key]?.trim() || '')))
    branding.setInfo({
      title: brandDrafts.value[BRAND_TITLE_KEY],
      subtitle: brandDrafts.value[BRAND_SUBTITLE_KEY],
      description: brandDrafts.value[BRAND_DESCRIPTION_KEY],
      footer: brandDrafts.value[BRAND_FOOTER_KEY],
    })
    flash.value = { kind: 'ok', text: t('admin.settings.branding.saved') }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, t('admin.settings.saveFailed')) }
  } finally {
    savingBrandInfo.value = false
  }
}

// Prefer the localized label in flash text so users in zh locale
// see "余额阈值 已保存" instead of "Low balance threshold 已保存".
function localizedLabel(it: SettingItem): string {
  return locale.value === 'zh' && it.label_zh ? it.label_zh : it.label
}

function settingHelp(it: SettingItem): string {
  const path = settingHelpPaths[it.key]
  return path ? t(path) : ''
}

function isMultilineSetting(key: string): boolean {
  return (MULTILINE_SETTING_KEYS as readonly string[]).includes(key)
}

function editorFormat(key: string): 'yaml' | 'json' {
  return TEMPLATE_EDITOR_FORMATS[key] || 'yaml'
}

function editorFormatLabel(key: string): string {
  return editorFormat(key).toUpperCase()
}

function editorDisplayLabel(it: SettingItem): string {
  return localizedLabel(it).replace(/\s*[（(]\s*(YAML|JSON)\s*[）)]\s*$/i, '')
}

function normalizeTemplateText(raw: string): string {
  const text = raw
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((line) => line.replace(/[ \t]+$/g, ''))
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
  return text ? text + '\n' : ''
}

function protectJSONTemplate(raw: string): string {
  return TEMPLATE_PLACEHOLDERS.reduce(
    (text, placeholder) => text.split(placeholder.raw).join(JSON.stringify(placeholder.token)),
    raw,
  )
}

function restoreJSONTemplate(raw: string): string {
  return TEMPLATE_PLACEHOLDERS.reduce(
    (text, placeholder) => text.split(JSON.stringify(placeholder.token)).join(placeholder.raw),
    raw,
  )
}

function protectYAMLTemplate(raw: string): string {
  return raw
    .split('\n')
    .map((line) => {
      const trimmed = line.trim()
      const indent = line.match(/^\s*/)?.[0] || ''
      if (trimmed === '${proxies}') return `${indent || '  '}[]`
      if (trimmed === '${proxy_groups}') return `${indent}proxy-groups: []`
      return line
        .split('${proxies}').join('__THREEXUI_TEMPLATE_PROXIES__')
        .split('${proxy_names}').join('__THREEXUI_TEMPLATE_PROXY_NAMES__')
        .split('${proxy_groups}').join('__THREEXUI_TEMPLATE_PROXY_GROUPS__')
    })
    .join('\n')
}

function formatJSONTemplate(raw: string): string {
  const parsed = JSON.parse(protectJSONTemplate(raw))
  return restoreJSONTemplate(JSON.stringify(parsed, null, 2)).trimEnd() + '\n'
}

function formatYAMLTemplate(raw: string): string {
  const normalized = normalizeTemplateText(raw)
  if (TEMPLATE_PLACEHOLDERS.some((placeholder) => normalized.includes(placeholder.raw))) {
    loadYAML(protectYAMLTemplate(normalized))
    return normalized
  }
  const parsed = loadYAML(normalized)
  return dumpYAML(parsed, { indent: 2, lineWidth: 120, noRefs: true }).trimEnd() + '\n'
}

function shortFormatError(e: unknown): string {
  const text = e instanceof Error ? e.message : String(e)
  return text.split('\n')[0] || text
}

function setFormatEditorFlash(key: string, value: { kind: 'ok' | 'err'; text: string }) {
  formatEditorFlash.value = { ...formatEditorFlash.value, [key]: value }
}

function clearFormatEditorFlash(key: string) {
  if (!formatEditorFlash.value[key]) return
  const next = { ...formatEditorFlash.value }
  delete next[key]
  formatEditorFlash.value = next
}

function formatEditorValue(key: string) {
  const format = editorFormat(key)
  const raw = drafts.value[key] || ''
  if (!raw.trim()) {
    setFormatEditorFlash(key, {
      kind: 'ok',
      text: t('admin.settings.formatEditor.empty'),
    })
    return
  }
  try {
    drafts.value[key] = format === 'json' ? formatJSONTemplate(raw) : formatYAMLTemplate(raw)
    setFormatEditorFlash(key, {
      kind: 'ok',
      text: t('admin.settings.formatEditor.formatted', { format: format.toUpperCase() }),
    })
  } catch (e) {
    setFormatEditorFlash(key, {
      kind: 'err',
      text: t('admin.settings.formatEditor.invalid', {
        format: format.toUpperCase(),
        message: shortFormatError(e),
      }),
    })
  }
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
    if (it.key === BRAND_ICON_KEY) continue
    if ((BRAND_INFO_KEYS as readonly string[]).includes(it.key)) continue
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
    if (group === 'registration' || group === SUBSCRIPTION_GROUP || group === ALERT_GROUP) continue
    buckets[group] = rows
  }
  return buckets
})

const subscriptionGrouped = computed(() => {
  const rows = grouped.value[SUBSCRIPTION_GROUP] || []
  return rows.length > 0 ? { [SUBSCRIPTION_GROUP]: rows } : {}
})

const alertGrouped = computed(() => {
  const rows = grouped.value[ALERT_GROUP] || []
  return rows.length > 0 ? { [ALERT_GROUP]: rows } : {}
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
type Tab = 'general' | 'subscription' | 'alerts' | 'securityAuth' | 'userDefaults' | 'messages' | 'notifications'
const tabs: Tab[] = ['general', 'subscription', 'alerts', 'securityAuth', 'userDefaults', 'messages', 'notifications']
const visibleTabs = computed<Tab[]>(() => tabs)
const defaultTab = computed<Tab>(() => visibleTabs.value[0] || 'general')
const tab = ref<Tab>('general')

function tabFromQuery(): Tab {
  const raw = typeof route.query.tab === 'string' ? route.query.tab : ''
  return visibleTabs.value.includes(raw as Tab) ? (raw as Tab) : defaultTab.value
}

watch(
  [() => route.query.tab, visibleTabs],
  () => {
    tab.value = tabFromQuery()
  },
  { immediate: true },
)

function selectTab(target: Tab) {
  tab.value = target
  void router.replace({
    query: {
      ...route.query,
      tab: target === defaultTab.value ? undefined : target,
    },
  })
}

function tabLabel(target: Tab): string {
  return ({
    general: t('admin.settings.generalTab'),
    subscription: t('admin.settings.subscriptionTab'),
    alerts: t('admin.settings.alertsTab'),
    securityAuth: t('admin.settings.securityAuthTab'),
    userDefaults: t('admin.settings.userDefaultsTab'),
    messages: t('admin.settings.messagesTab'),
    notifications: t('admin.settings.notificationsTab'),
  } as Record<Tab, string>)[target]
}

onMounted(load)
</script>

<template>
  <div class="space-y-5">
    <header class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">
          {{ $t('admin.settings.title') }}
        </h1>
      </div>
      <button class="settings-secondary-button" type="button" @click="load">
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12a9 9 0 0 1-15.4 6.4L3 16" />
          <path d="M3 21v-5h5" />
          <path d="M3 12a9 9 0 0 1 15.4-6.4L21 8" />
          <path d="M21 3v5h-5" />
        </svg>
        {{ $t('admin.settings.refresh') }}
      </button>
    </header>

    <p v-if="error" class="settings-alert settings-alert-error">{{ error }}</p>
    <p
      v-if="flash"
      class="settings-alert"
      :class="flash.kind === 'ok' ? 'settings-alert-ok' : 'settings-alert-error'"
    >
      {{ flash.text }}
    </p>

    <div v-if="loading" class="rounded-xl border border-surface-100 bg-surface-0 p-5 text-sm text-surface-500 dark:border-surface-800 dark:bg-surface-900">
      {{ $t('app.loading') }}
    </div>

    <div v-else class="space-y-5">
      <nav class="settings-tabbar" role="tablist" aria-label="Settings surfaces">
        <button
          v-for="t in visibleTabs"
          :key="t"
          type="button"
          role="tab"
          :aria-selected="tab === t"
          :class="[
            'settings-tab-button',
            tab === t
              ? 'bg-ink-900 text-white shadow-card dark:bg-surface-50 dark:text-ink-900'
              : 'text-surface-500 hover:bg-surface-100 hover:text-ink-900 dark:text-surface-400 dark:hover:bg-surface-800 dark:hover:text-surface-50',
          ]"
          @click="selectTab(t)"
        >
          <span
            :class="[
              'settings-tab-icon',
              tab === t ? 'bg-white/15 text-white dark:bg-ink-900/10 dark:text-ink-900' : 'bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-300',
            ]"
          >
            <svg v-if="t === 'general'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
            <svg v-else-if="t === 'subscription'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 5h16" /><path d="M4 12h16" /><path d="M4 19h10" /><path d="m16 17 2 2 4-5" /></svg>
            <svg v-else-if="t === 'alerts'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10.3 3.9 2.9 18a2 2 0 0 0 1.8 3h14.6a2 2 0 0 0 1.8-3L13.7 3.9a2 2 0 0 0-3.4 0z" /><path d="M12 9v4" /><path d="M12 17h.01" /></svg>
            <svg v-else-if="t === 'securityAuth'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="m9 12 2 2 4-5" /></svg>
            <svg v-else-if="t === 'userDefaults'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21a8 8 0 0 0-16 0" /><circle cx="12" cy="7" r="4" /><path d="M17 11h4M19 9v4" /></svg>
            <svg v-else-if="t === 'messages'" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" /><path d="m22 6-10 7L2 6" /></svg>
            <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" /></svg>
          </span>
          {{ tabLabel(t) }}
        </button>
      </nav>

      <section v-show="tab === 'general'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.siteSettingsTitle') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.generalDesc') }}</p>
          </div>
        </header>
        <div class="settings-brand-section">
          <div class="settings-brand-heading">
            <div>
              <h3>{{ $t('admin.settings.branding.infoTitle') }}</h3>
              <p>{{ $t('admin.settings.branding.desc') }}</p>
            </div>
            <button class="settings-primary-button settings-brand-save" type="button" :disabled="savingBrandInfo" @click="saveBrandInfo">
              {{ savingBrandInfo ? $t('admin.settings.saving') : $t('admin.settings.branding.save') }}
            </button>
          </div>
          <div class="settings-brand-main">
            <div class="settings-brand-card">
              <div class="settings-brand-preview">
                <div class="settings-brand-icon">
                  <img
                    v-if="iconPreview || branding.iconUrl"
                    :src="iconPreview || branding.iconUrl"
                    alt=""
                    class="h-12 w-12 rounded-lg object-cover"
                  />
                  <svg v-else class="h-8 w-8" viewBox="0 0 24 24" fill="currentColor" stroke="none">
                    <path d="M13 2 3 14h7l-1 8 11-13h-7l0-7z" />
                  </svg>
                </div>
                <div class="min-w-0">
                  <p class="text-eyebrow font-semibold uppercase tracking-eyebrow text-surface-400">
                    {{ $t('admin.settings.branding.current') }}
                  </p>
                  <h3 class="mt-1 truncate text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50">
                    {{ brandDrafts[BRAND_TITLE_KEY] || branding.title }}
                  </h3>
                  <p class="mt-1 truncate text-sm font-medium text-surface-600 dark:text-surface-300">
                    {{ brandDrafts[BRAND_SUBTITLE_KEY] || branding.subtitle }}
                  </p>
                  <p class="settings-brand-description-preview">
                    {{ brandDrafts[BRAND_DESCRIPTION_KEY] || branding.description }}
                  </p>
                </div>
              </div>
              <div class="settings-brand-upload">
                <div class="settings-brand-action-title">{{ $t('admin.settings.branding.icon') }}</div>
                <div class="settings-brand-upload-actions">
                  <label class="settings-secondary-button cursor-pointer justify-center">
                    <input class="sr-only" type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" @change="onIconPicked" />
                    <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M12 3v12" />
                      <path d="m7 8 5-5 5 5" />
                      <path d="M5 21h14" />
                    </svg>
                    {{ $t('admin.settings.branding.choose') }}
                  </label>
                  <button
                    class="settings-secondary-button justify-center"
                    type="button"
                    :disabled="!iconFile || iconBusy"
                    @click="uploadIcon"
                  >
                    {{ iconBusy ? $t('admin.settings.branding.uploading') : $t('admin.settings.branding.upload') }}
                  </button>
                </div>
              </div>
            </div>

            <div class="settings-brand-form">
              <label class="settings-stack-field">
                <span>{{ $t('admin.settings.branding.brandTitle') }}</span>
                <input v-model="brandDrafts[BRAND_TITLE_KEY]" type="text" maxlength="80" class="settings-input" placeholder="3xui Central" />
              </label>
              <label class="settings-stack-field">
                <span>{{ $t('admin.settings.branding.subtitle') }}</span>
                <input v-model="brandDrafts[BRAND_SUBTITLE_KEY]" type="text" maxlength="120" class="settings-input" :placeholder="$t('brand.centralPanel')" />
              </label>
              <label class="settings-stack-field md:col-span-2">
                <span>{{ $t('admin.settings.branding.description') }}</span>
                <input v-model="brandDrafts[BRAND_DESCRIPTION_KEY]" type="text" maxlength="240" class="settings-input" :placeholder="$t('brand.slogan')" />
              </label>
              <label class="settings-stack-field md:col-span-2">
                <span>{{ $t('admin.settings.branding.footer') }}</span>
                <input v-model="brandDrafts[BRAND_FOOTER_KEY]" type="text" maxlength="240" class="settings-input" :placeholder="$t('brand.footer')" />
              </label>
            </div>
          </div>
        </div>
        <p
          v-if="iconFlash"
          class="settings-alert mx-4 mb-4 lg:mx-5"
          :class="iconFlash.kind === 'ok' ? 'settings-alert-ok' : 'settings-alert-error'"
        >
          {{ iconFlash.text }}
        </p>
        <div v-for="(rows, group) in generalGrouped" :key="group" class="settings-group">
          <div class="settings-group-title">{{ groupLabel(group as string) }}</div>
          <div
            v-for="it in rows"
            :key="it.key"
            :data-setting-key="it.key"
            :class="[
              'border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:px-5',
              isMultilineSetting(it.key)
                ? 'space-y-3'
                : 'grid gap-3 lg:grid-cols-[minmax(220px,0.36fr),minmax(0,1fr)] lg:items-start',
            ]"
          >
            <div :class="isMultilineSetting(it.key) ? 'max-w-none' : 'max-w-2xl'">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0">
                  <label class="settings-label" :for="'setting-' + it.key">
                    {{ isMultilineSetting(it.key) ? editorDisplayLabel(it) : localizedLabel(it) }}
                  </label>
                  <p v-if="settingHelp(it)" class="settings-help">{{ settingHelp(it) }}</p>
                </div>
                <div v-if="isMultilineSetting(it.key)" class="settings-editor-toolbar">
                  <span class="settings-format-chip">{{ editorFormatLabel(it.key) }}</span>
                  <button class="settings-small-secondary" type="button" @click="formatEditorValue(it.key)">
                    {{ $t('admin.settings.formatEditor.format') }}
                  </button>
                </div>
              </div>
            </div>
            <div :class="['flex min-w-0 flex-col gap-2', isMultilineSetting(it.key) ? 'w-full' : 'lg:items-end']">
              <template v-if="isMultilineSetting(it.key)">
                <div class="settings-code-editor">
                  <textarea
                    :id="'setting-' + it.key"
                    v-model="drafts[it.key]"
                    rows="10"
                    spellcheck="false"
                    class="settings-code-input"
                    @input="clearFormatEditorFlash(it.key)"
                  />
                </div>
                <p
                  v-if="formatEditorFlash[it.key]"
                  class="settings-editor-flash"
                  :class="formatEditorFlash[it.key].kind === 'ok' ? 'text-accent-600 dark:text-accent-300' : 'text-red-600 dark:text-red-300'"
                >
                  {{ formatEditorFlash[it.key].text }}
                </p>
              </template>
              <select
                v-else-if="it.type === 'bool'"
                :id="'setting-' + it.key"
                v-model="drafts[it.key]"
                class="settings-input lg:w-44"
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
                class="settings-input lg:w-44"
              />
              <input
                v-else
                :id="'setting-' + it.key"
                v-model="drafts[it.key]"
                type="text"
                class="settings-input lg:w-80"
              />
              <div class="flex flex-wrap justify-end gap-2">
                <button class="settings-small-primary" type="button" :disabled="savingKey === it.key" @click="save(it)">
                  {{ savingKey === it.key ? $t('admin.settings.saving') : $t('admin.settings.save') }}
                </button>
                <button
                  v-if="it.has_override"
                  class="settings-small-secondary"
                  type="button"
                  :disabled="savingKey === it.key"
                  @click="clearOverride(it)"
                >
                  {{ $t('admin.settings.reset') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-show="tab === 'subscription'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.subscriptionTitle') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.subscriptionDesc') }}</p>
          </div>
        </header>
        <div v-for="(rows, group) in subscriptionGrouped" :key="group" class="settings-group">
          <div class="settings-group-title">{{ groupLabel(group as string) }}</div>
          <div
            v-for="it in rows"
            :key="it.key"
            :data-setting-key="it.key"
            :class="[
              'border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:px-5',
              isMultilineSetting(it.key)
                ? 'space-y-3'
                : 'grid gap-3 lg:grid-cols-[minmax(220px,0.36fr),minmax(0,1fr)] lg:items-start',
            ]"
          >
            <div :class="isMultilineSetting(it.key) ? 'max-w-none' : 'max-w-2xl'">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0">
                  <label class="settings-label" :for="'setting-' + it.key">
                    {{ isMultilineSetting(it.key) ? editorDisplayLabel(it) : localizedLabel(it) }}
                  </label>
                  <p v-if="settingHelp(it)" class="settings-help">{{ settingHelp(it) }}</p>
                </div>
                <div v-if="isMultilineSetting(it.key)" class="settings-editor-toolbar">
                  <span class="settings-format-chip">{{ editorFormatLabel(it.key) }}</span>
                  <button class="settings-small-secondary" type="button" @click="formatEditorValue(it.key)">
                    {{ $t('admin.settings.formatEditor.format') }}
                  </button>
                </div>
              </div>
            </div>
            <div :class="['flex min-w-0 flex-col gap-2', isMultilineSetting(it.key) ? 'w-full' : 'lg:items-end']">
              <template v-if="isMultilineSetting(it.key)">
                <div class="settings-code-editor">
                  <textarea
                    :id="'setting-' + it.key"
                    v-model="drafts[it.key]"
                    rows="10"
                    spellcheck="false"
                    class="settings-code-input"
                    @input="clearFormatEditorFlash(it.key)"
                  />
                </div>
                <p
                  v-if="formatEditorFlash[it.key]"
                  class="settings-editor-flash"
                  :class="formatEditorFlash[it.key].kind === 'ok' ? 'text-accent-600 dark:text-accent-300' : 'text-red-600 dark:text-red-300'"
                >
                  {{ formatEditorFlash[it.key].text }}
                </p>
              </template>
              <select
                v-else-if="it.type === 'bool'"
                :id="'setting-' + it.key"
                v-model="drafts[it.key]"
                class="settings-input lg:w-44"
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
                class="settings-input lg:w-44"
              />
              <input
                v-else
                :id="'setting-' + it.key"
                v-model="drafts[it.key]"
                type="text"
                class="settings-input lg:w-80"
              />
              <div class="flex flex-wrap justify-end gap-2">
                <button class="settings-small-primary" type="button" :disabled="savingKey === it.key" @click="save(it)">
                  {{ savingKey === it.key ? $t('admin.settings.saving') : $t('admin.settings.save') }}
                </button>
                <button
                  v-if="it.has_override"
                  class="settings-small-secondary"
                  type="button"
                  :disabled="savingKey === it.key"
                  @click="clearOverride(it)"
                >
                  {{ $t('admin.settings.reset') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-show="tab === 'alerts'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.alertsTitle') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.alertsDesc') }}</p>
          </div>
        </header>
        <div v-for="(rows, group) in alertGrouped" :key="group" class="settings-group">
          <div class="settings-group-title">{{ groupLabel(group as string) }}</div>
          <div
            v-for="it in rows"
            :key="it.key"
            :data-setting-key="it.key"
            :class="[
              'border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:px-5',
              isMultilineSetting(it.key)
                ? 'space-y-3'
                : 'grid gap-3 lg:grid-cols-[minmax(220px,0.36fr),minmax(0,1fr)] lg:items-start',
            ]"
          >
            <div :class="isMultilineSetting(it.key) ? 'max-w-none' : 'max-w-2xl'">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0">
                  <label class="settings-label" :for="'setting-' + it.key">
                    {{ isMultilineSetting(it.key) ? editorDisplayLabel(it) : localizedLabel(it) }}
                  </label>
                  <p v-if="settingHelp(it)" class="settings-help">{{ settingHelp(it) }}</p>
                </div>
                <div v-if="isMultilineSetting(it.key)" class="settings-editor-toolbar">
                  <span class="settings-format-chip">{{ editorFormatLabel(it.key) }}</span>
                  <button class="settings-small-secondary" type="button" @click="formatEditorValue(it.key)">
                    {{ $t('admin.settings.formatEditor.format') }}
                  </button>
                </div>
              </div>
            </div>
            <div :class="['flex min-w-0 flex-col gap-2', isMultilineSetting(it.key) ? 'w-full' : 'lg:items-end']">
              <template v-if="isMultilineSetting(it.key)">
                <div class="settings-code-editor">
                  <textarea
                    :id="'setting-' + it.key"
                    v-model="drafts[it.key]"
                    rows="10"
                    spellcheck="false"
                    class="settings-code-input"
                    @input="clearFormatEditorFlash(it.key)"
                  />
                </div>
                <p
                  v-if="formatEditorFlash[it.key]"
                  class="settings-editor-flash"
                  :class="formatEditorFlash[it.key].kind === 'ok' ? 'text-accent-600 dark:text-accent-300' : 'text-red-600 dark:text-red-300'"
                >
                  {{ formatEditorFlash[it.key].text }}
                </p>
              </template>
              <select
                v-else-if="it.type === 'bool'"
                :id="'setting-' + it.key"
                v-model="drafts[it.key]"
                class="settings-input lg:w-44"
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
                class="settings-input lg:w-44"
              />
              <input
                v-else
                :id="'setting-' + it.key"
                v-model="drafts[it.key]"
                type="text"
                class="settings-input lg:w-80"
              />
              <div class="flex flex-wrap justify-end gap-2">
                <button class="settings-small-primary" type="button" :disabled="savingKey === it.key" @click="save(it)">
                  {{ savingKey === it.key ? $t('admin.settings.saving') : $t('admin.settings.save') }}
                </button>
                <button
                  v-if="it.has_override"
                  class="settings-small-secondary"
                  type="button"
                  :disabled="savingKey === it.key"
                  @click="clearOverride(it)"
                >
                  {{ $t('admin.settings.reset') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-show="tab === 'securityAuth'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.securityAuthTab') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.registration.subtitle') }}</p>
          </div>
        </header>

        <div class="settings-group">
          <div class="settings-group-title">{{ $t('admin.settings.registration.title') }}</div>
          <div class="settings-registration-list">
            <div class="settings-registration-row">
              <div>
                <h3 class="settings-label">{{ $t('admin.settings.registration.publicTitle') }}</h3>
                <p class="settings-help">{{ $t('admin.settings.registration.publicDesc') }}</p>
              </div>
              <button
                type="button"
                role="switch"
                :aria-checked="effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true)"
                :disabled="savingRegistrationKey === PUBLIC_REGISTRATION_KEY"
                class="settings-switch"
                :class="effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true) ? 'bg-accent-600' : 'bg-surface-300 dark:bg-surface-700'"
                @click="setRegistrationBool(PUBLIC_REGISTRATION_KEY, !effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true))"
              >
                <span
                  class="settings-switch-knob"
                  :class="effectiveSettingBool(PUBLIC_REGISTRATION_KEY, true) ? 'translate-x-5' : 'translate-x-0.5'"
                ></span>
              </button>
            </div>
            <div class="settings-registration-row">
              <div>
                <h3 class="settings-label">{{ $t('admin.settings.registration.verifyTitle') }}</h3>
                <p class="settings-help">{{ $t('admin.settings.registration.verifyDesc') }}</p>
              </div>
              <button
                type="button"
                role="switch"
                :aria-checked="effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false)"
                :disabled="savingRegistrationKey === EMAIL_VERIFICATION_REQUIRED_KEY"
                class="settings-switch"
                :class="effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false) ? 'bg-accent-600' : 'bg-surface-300 dark:bg-surface-700'"
                @click="setRegistrationBool(EMAIL_VERIFICATION_REQUIRED_KEY, !effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false))"
              >
                <span
                  class="settings-switch-knob"
                  :class="effectiveSettingBool(EMAIL_VERIFICATION_REQUIRED_KEY, false) ? 'translate-x-5' : 'translate-x-0.5'"
                ></span>
              </button>
            </div>
            <div class="settings-registration-row settings-registration-row-input">
              <div>
                <h3 class="settings-label">{{ $t('admin.settings.registration.allowlistTitle') }}</h3>
                <p class="settings-help">{{ $t('admin.settings.registration.allowlistDesc') }}</p>
              </div>
              <div class="settings-registration-control">
                <div class="settings-token-input">
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
                <p class="mt-2 text-sm text-surface-500">{{ $t('admin.settings.registration.allowlistHint') }}</p>
              </div>
            </div>
          </div>
        </div>

        <div class="settings-group">
          <div class="settings-group-title flex items-center justify-between gap-3">
            <span>{{ $t('admin.settings.oidc.title') }}</span>
            <span
              class="rounded-full px-2.5 py-1 text-2xs font-medium ring-1 ring-inset"
              :class="oidcStatus === $t('admin.settings.oidc.enabled')
                ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                : 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700'"
            >
              {{ oidcStatus }}
            </span>
          </div>
          <div class="settings-oidc-islands">
            <div class="settings-oidc-main">
              <section class="settings-oidc-island">
                <header>
                  <h3 class="settings-label">{{ $t('admin.settings.oidc.connectionTitle') }}</h3>
                  <p class="settings-help">{{ $t('admin.settings.oidc.connectionDesc') }}</p>
                </header>
                <div class="mt-4 grid gap-3 md:grid-cols-2">
                  <label class="settings-stack-field">
                    <span>{{ $t('admin.settings.oidc.issuer') }}</span>
                    <input v-model="oidcDrafts.oidc_issuer" type="url" placeholder="https://auth.example.com" class="settings-input" />
                  </label>
                  <label class="settings-stack-field">
                    <span>{{ $t('admin.settings.oidc.redirectUrl') }}</span>
                    <input v-model="oidcDrafts.oidc_redirect_url" type="url" placeholder="https://panel.example.com/oidc/callback" class="settings-input" />
                  </label>
                  <label class="settings-stack-field">
                    <span>{{ $t('admin.settings.oidc.clientId') }}</span>
                    <input v-model="oidcDrafts.oidc_client_id" type="text" class="settings-input" />
                  </label>
                  <label class="settings-stack-field">
                    <span>{{ $t('admin.settings.oidc.clientSecret') }}</span>
                    <input v-model="oidcDrafts.oidc_client_secret" type="password" autocomplete="new-password" class="settings-input" />
                  </label>
                </div>
              </section>

              <section class="settings-oidc-island">
                <header>
                  <h3 class="settings-label">{{ $t('admin.settings.oidc.loginDisplayTitle') }}</h3>
                  <p class="settings-help">{{ $t('admin.settings.oidc.loginDisplayDesc') }}</p>
                </header>
                <div class="mt-4 grid gap-3 md:grid-cols-2">
                  <label class="settings-stack-field">
                    <span>{{ $t('admin.settings.oidc.scopes') }}</span>
                    <input v-model="oidcDrafts.oidc_scopes" type="text" placeholder="openid,profile,email" class="settings-input" />
                  </label>
                  <label class="settings-stack-field">
                    <span>{{ $t('admin.settings.oidc.displayName') }}</span>
                    <input v-model="oidcDrafts.oidc_display_name" type="text" placeholder="Company SSO" class="settings-input" />
                  </label>
                  <label class="settings-stack-field md:col-span-2">
                    <span>{{ $t('admin.settings.oidc.iconUrl') }}</span>
                    <input v-model="oidcDrafts.oidc_icon_url" type="url" placeholder="https://cdn.example.com/sso.svg" class="settings-input" />
                  </label>
                </div>
              </section>
            </div>

            <section class="settings-oidc-island settings-oidc-advanced">
              <header>
                <h3 class="settings-label">{{ $t('admin.settings.oidc.advancedTitle') }}</h3>
                <p class="settings-help">{{ $t('admin.settings.oidc.advancedDesc') }}</p>
              </header>
              <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                <label v-for="field in [
                  ['oidc_auth_url', $t('admin.settings.oidc.authUrl')],
                  ['oidc_token_url', $t('admin.settings.oidc.tokenUrl')],
                  ['oidc_jwks_url', $t('admin.settings.oidc.jwksUrl')],
                  ['oidc_userinfo_url', $t('admin.settings.oidc.userinfoUrl')],
                ]" :key="field[0]" class="settings-stack-field">
                  <span>{{ field[1] }}</span>
                  <input v-model="oidcDrafts[field[0]]" type="url" class="settings-input h-9 text-xs" />
                </label>
              </div>
            </section>
          </div>
          <footer class="settings-panel-footer">
            <button class="settings-primary-button" type="button" :disabled="savingOIDC" @click="saveOIDCSettings">
              {{ savingOIDC ? $t('admin.settings.saving') : $t('admin.settings.oidc.save') }}
            </button>
          </footer>
        </div>
      </section>

      <section v-show="tab === 'userDefaults'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.newUserPolicy.title') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.newUserPolicy.subtitle') }}</p>
          </div>
        </header>
        <div class="settings-group">
          <div class="grid gap-3 border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:grid-cols-[minmax(220px,0.36fr),minmax(0,1fr)] lg:px-5">
            <div>
              <label class="settings-label" for="new-user-initial-balance">{{ $t('admin.settings.newUserPolicy.initialBalance') }}</label>
              <p class="settings-help">{{ $t('admin.settings.newUserPolicy.initialBalanceDesc') }}</p>
            </div>
            <div class="flex max-w-xs items-center rounded-lg border border-surface-200 bg-surface-50 px-3 focus-within:border-accent-500 focus-within:ring-2 focus-within:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70">
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

          <div class="border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:px-5">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="settings-label">{{ $t('admin.settings.newUserPolicy.starterPlans') }}</h3>
                <p class="settings-help">{{ $t('admin.settings.newUserPolicy.starterPlansDesc') }}</p>
              </div>
              <div class="settings-user-defaults-actions">
                <span class="settings-summary-pill">
                  {{ starterPlanSummary }}
                </span>
                <button type="button" class="settings-small-secondary" @click="starterPlanIDs = []">
                  {{ $t('admin.settings.newUserPolicy.allowAll') }}
                </button>
              </div>
            </div>
            <div class="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              <label
                v-for="p in plans"
                :key="p.id"
                class="flex min-h-14 cursor-pointer items-start gap-2.5 rounded-lg border px-3 py-2.5 transition-all duration-150 ease-brand"
                :class="starterPlanIDs.includes(p.id)
                  ? 'border-accent-300 bg-accent-50 dark:border-accent-700 dark:bg-accent-950/40'
                  : 'border-surface-100 bg-surface-50/80 hover:border-surface-300 hover:bg-surface-50 dark:border-surface-800 dark:bg-surface-950/30 dark:hover:bg-surface-800'"
              >
                <input v-model="starterPlanIDs" type="checkbox" :value="p.id" class="mt-0.5 h-4 w-4 rounded border-surface-300 text-accent-600 focus:ring-accent-500/30" />
                <span class="min-w-0">
                  <span class="block truncate text-sm font-medium text-ink-900 dark:text-surface-50">{{ p.name }}</span>
                  <span class="mt-0.5 block text-xs text-surface-500">#{{ p.id }} · {{ formatYuan(p.price_cents) }} · {{ p.enabled ? $t('admin.settings.newUserPolicy.enabled') : $t('admin.settings.newUserPolicy.disabled') }}</span>
                </span>
              </label>
            </div>
          </div>
        </div>
        <footer class="settings-panel-footer">
          <button class="settings-primary-button" type="button" :disabled="savingNewUserPolicy" @click="saveNewUserPolicy">
            {{ savingNewUserPolicy ? $t('admin.settings.saving') : $t('admin.settings.newUserPolicy.save') }}
          </button>
        </footer>
      </section>

      <section v-show="tab === 'messages'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.messages.title') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.messages.desc') }}</p>
          </div>
        </header>
        <div class="grid gap-3 border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:grid-cols-[minmax(220px,0.36fr),minmax(0,1fr)] lg:px-5">
          <div>
            <h3 class="settings-label">{{ $t('admin.settings.smtpTestTitle') }}</h3>
            <p class="settings-help">{{ $t('admin.settings.smtpHint') }}</p>
          </div>
          <div class="settings-smtp-test-form">
            <input v-model="smtpTo" type="email" placeholder="admin@example.com" class="settings-input" />
            <button class="settings-primary-button settings-smtp-test-button" type="button" :disabled="smtpBusy || !smtpTo" @click="sendSMTPTest">
              {{ smtpBusy ? $t('admin.settings.smtpSending') : $t('admin.settings.smtpSendBtn') }}
            </button>
            <span v-if="smtpFlash" class="settings-smtp-test-flash" :class="smtpFlash.kind === 'ok' ? 'text-accent-600' : 'text-red-600'">{{ smtpFlash.text }}</span>
          </div>
        </div>
      </section>

      <section v-show="tab === 'notifications'" class="settings-panel">
        <header class="settings-panel-header">
          <div>
            <h2 class="settings-panel-title">{{ $t('admin.settings.notifications.title') }}</h2>
            <p class="settings-panel-desc">{{ $t('admin.settings.notifications.desc') }}</p>
          </div>
        </header>
        <Webhooks v-if="tab === 'notifications'" embedded class="px-4 py-4 lg:px-5" />
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings-brand-section {
  @apply space-y-4 border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:px-5;
}

.settings-brand-heading {
  @apply flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between;
}

.settings-brand-heading h3 {
  @apply text-sm font-semibold text-ink-900 dark:text-surface-50;
}

.settings-brand-heading p {
  @apply mt-1 text-sm text-surface-500 dark:text-surface-400;
}

.settings-brand-main {
  @apply grid gap-4 xl:grid-cols-[minmax(270px,320px),minmax(0,1fr)] xl:items-start;
}

.settings-brand-card {
  @apply space-y-3 rounded-lg border border-surface-100 bg-surface-50/70 p-3 dark:border-surface-800 dark:bg-surface-950/25;
}

.settings-brand-preview {
  @apply flex min-w-0 items-start gap-4;
}

.settings-brand-description-preview {
  @apply mt-2 overflow-hidden text-sm leading-6 text-surface-500 dark:text-surface-400;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.settings-brand-icon {
  @apply flex h-16 w-16 shrink-0 items-center justify-center rounded-xl bg-accent-500 text-white shadow-card ring-1 ring-accent-700/30;
}

.settings-brand-form {
  @apply grid min-w-0 gap-3 md:grid-cols-2;
}

.settings-brand-upload {
  @apply border-t border-surface-100 pt-3 dark:border-surface-800;
}

.settings-brand-upload-actions {
  @apply mt-2 grid grid-cols-2 gap-2;
}

.settings-brand-action-title {
  @apply text-xs font-semibold uppercase tracking-caps text-surface-500 dark:text-surface-400;
}

.settings-brand-save {
  @apply shrink-0 justify-center sm:mt-0;
}

.settings-tabbar {
  @apply flex max-w-full flex-wrap gap-1 rounded-xl border border-surface-100 bg-surface-0 p-1 shadow-card dark:border-surface-800 dark:bg-surface-900;
}

.settings-tab-button {
  @apply inline-flex h-11 shrink-0 items-center gap-2 rounded-lg px-3.5 text-sm font-semibold transition-all ease-brand active:scale-[0.98];
}

.settings-tab-icon {
  @apply flex h-7 w-7 shrink-0 items-center justify-center rounded-md transition-colors;
}

.settings-panel {
  @apply overflow-hidden rounded-xl border border-surface-100 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900;
}

.settings-panel-header {
  @apply flex flex-col gap-2 px-4 py-4 lg:px-5;
}

.settings-panel-title {
  @apply text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50;
}

.settings-panel-desc {
  @apply mt-1 text-sm text-surface-500 dark:text-surface-400;
}

.settings-summary-pill {
  @apply inline-flex h-8 max-w-full items-center truncate rounded-full bg-accent-50 px-2.5 text-xs font-medium text-accent-700 ring-1 ring-inset ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800;
}

.settings-user-defaults-actions {
  @apply flex w-full min-w-0 flex-col gap-2 sm:w-auto sm:flex-row sm:items-center sm:justify-end;
}

.settings-group {
  @apply border-t border-surface-100 dark:border-surface-800;
}

.settings-group-title {
  @apply px-4 py-3 text-xs font-semibold uppercase tracking-caps text-surface-500 dark:text-surface-400 lg:px-5;
}

.settings-label {
  @apply text-sm font-semibold text-ink-900 dark:text-surface-50;
}

.settings-help {
  @apply mt-1 text-sm text-surface-500 dark:text-surface-400;
}

.settings-oidc-islands {
  @apply grid gap-4 border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:px-5 xl:grid-cols-[minmax(0,1fr),minmax(320px,0.46fr)] xl:items-start;
}

.settings-oidc-main {
  @apply grid gap-4;
}

.settings-oidc-island {
  @apply rounded-lg border border-surface-100 bg-surface-50/60 p-4 dark:border-surface-800 dark:bg-surface-950/35;
}

.settings-oidc-advanced {
  @apply xl:sticky xl:top-4;
}

.settings-input {
  @apply h-10 w-full rounded-lg border border-surface-200 bg-surface-50 px-3 text-sm text-ink-900 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70 dark:text-surface-50;
}

.settings-textarea {
  @apply min-h-32 w-full rounded-lg border border-surface-200 bg-surface-50 px-3 py-2 font-mono text-xs leading-5 text-ink-900 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70 dark:text-surface-50;
}

.settings-textarea-wide {
  @apply min-h-36 resize-y;
}

.settings-editor-toolbar {
  @apply flex shrink-0 items-center gap-2 self-start;
}

.settings-format-chip {
  @apply inline-flex h-8 items-center rounded-md border border-surface-200 bg-surface-50 px-2.5 font-mono text-2xs font-semibold uppercase tracking-caps text-surface-500 dark:border-surface-700 dark:bg-surface-800/70 dark:text-surface-300;
}

.settings-code-editor {
  @apply w-full overflow-hidden rounded-lg border border-surface-200 bg-surface-950 shadow-inner transition-colors focus-within:border-accent-500 focus-within:ring-2 focus-within:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-950;
}

.settings-code-input {
  @apply min-h-64 w-full resize-y border-0 bg-transparent px-3.5 py-3 font-mono text-xs leading-5 text-surface-100 outline-none placeholder:text-surface-500 focus:ring-0;
  tab-size: 2;
}

.settings-editor-flash {
  @apply text-xs font-medium;
}

.settings-stack-field {
  @apply block text-xs font-semibold text-surface-600 dark:text-surface-300;
}

.settings-stack-field > input {
  @apply mt-1.5;
}

.settings-secondary-button {
  @apply inline-flex h-9 items-center gap-1.5 rounded-lg border border-surface-200 bg-surface-0 px-3 text-sm font-medium text-surface-700 transition-colors hover:bg-surface-50 disabled:opacity-60 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-200 dark:hover:bg-surface-800;
}

.settings-primary-button {
  @apply inline-flex h-9 items-center whitespace-nowrap rounded-lg bg-ink-900 px-4 text-sm font-semibold text-white shadow-card transition-colors hover:bg-ink-800 disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500;
}

.settings-small-primary {
  @apply inline-flex h-8 items-center rounded-lg bg-ink-900 px-3 text-xs font-semibold text-white transition-colors hover:bg-ink-800 disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500;
}

.settings-smtp-test-form {
  @apply grid min-w-0 gap-2 sm:grid-cols-[minmax(0,1fr),auto];
}

.settings-smtp-test-button {
  @apply justify-center px-5;
}

.settings-smtp-test-flash {
  @apply text-xs sm:col-span-2;
}

.settings-small-secondary {
  @apply inline-flex h-8 items-center rounded-lg border border-surface-200 px-3 text-xs font-semibold text-surface-700 transition-colors hover:bg-surface-50 disabled:opacity-60 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800;
}

.settings-alert {
  @apply rounded-lg border px-3 py-2 text-sm;
}

.settings-alert-ok {
  @apply border-accent-100 bg-accent-50 text-accent-800 dark:border-accent-800 dark:bg-accent-950/40 dark:text-accent-200;
}

.settings-alert-error {
  @apply border-red-100 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200;
}

.settings-registration-list {
  @apply border-t border-surface-100 dark:border-surface-800;
}

.settings-registration-row {
  @apply flex min-h-20 items-center justify-between gap-4 px-4 py-4 dark:border-surface-800 lg:px-5;
}

.settings-registration-row + .settings-registration-row {
  @apply border-t border-surface-100 dark:border-surface-800;
}

.settings-registration-row-input {
  @apply items-start;
}

.settings-registration-control {
  @apply w-full min-w-0 lg:max-w-[72%];
}

.settings-switch {
  @apply relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors duration-150 ease-brand focus:outline-none focus:ring-2 focus:ring-accent-500/40 disabled:opacity-60;
}

.settings-switch-knob {
  @apply inline-block h-5 w-5 rounded-full bg-white shadow transition-transform duration-150 ease-brand;
}

.settings-token-input {
  @apply flex min-h-11 flex-wrap items-center gap-2 rounded-lg border border-surface-200 bg-surface-50 px-3 py-2 transition-colors focus-within:border-accent-500 focus-within:ring-2 focus-within:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70;
}

.settings-panel-footer {
  @apply flex justify-end border-t border-surface-100 px-4 py-3 dark:border-surface-800 lg:px-5;
}
</style>
