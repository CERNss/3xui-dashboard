import type { SettingItem } from '@/api/admin/settings'
import type { SettingsTab } from './types'

export const SETTINGS_TABS: SettingsTab[] = [
  'general',
  'subscription',
  'alerts',
  'dataCollection',
  'securityAuth',
  'userDefaults',
  'messages',
  'notifications',
]

export const tabI18nKeys: Record<SettingsTab, string> = {
  general: 'admin.settings.generalTab',
  subscription: 'admin.settings.subscriptionTab',
  alerts: 'admin.settings.alertsTab',
  dataCollection: 'admin.settings.dataCollectionTab',
  securityAuth: 'admin.settings.securityAuthTab',
  userDefaults: 'admin.settings.userDefaultsTab',
  messages: 'admin.settings.messagesTab',
  notifications: 'admin.settings.notificationsTab',
}

export const BRAND_ICON_KEY = 'brand_icon_url'
export const BRAND_DOCS_URL_KEY = 'brand_docs_url'
export const BRAND_HOMEPAGE_CONTENT_KEY = 'brand_homepage_content'
export const REGISTRATION_KEYS = new Set([
  'public_registration_enabled',
  'email_verification_required',
  'email_domain_allowlist',
])
export const NEW_USER_KEYS = new Set(['new_user_initial_balance_cents', 'new_user_plan_ids'])
export const BRAND_INFO_KEYS = new Set([
  'site_name',
  'brand_title',
  'brand_subtitle',
  'brand_description',
  'brand_footer',
  BRAND_DOCS_URL_KEY,
  BRAND_HOMEPAGE_CONTENT_KEY,
])
export const OIDC_KEYS = new Set([
  'oidc_enabled',
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
])

export function itemValue(item: SettingItem) {
  return item.value ?? ''
}

export function localizedLabel(item: SettingItem, language?: string) {
  return language?.startsWith('zh') && item.label_zh ? item.label_zh : item.label
}

export function localizedDescription(item: SettingItem, language?: string) {
  return language?.startsWith('zh') && item.description_zh ? item.description_zh : item.description
}

export function filterSettings(items: SettingItem[], tab: SettingsTab) {
  return items.filter((item) => {
    if (item.key === BRAND_ICON_KEY) return false
    if (tab === 'general') {
      return item.group === 'other' && !BRAND_INFO_KEYS.has(item.key) && !OIDC_KEYS.has(item.key)
    }
    if (tab === 'subscription') return item.group === 'subscription'
    if (tab === 'alerts') return item.group === 'traffic'
    if (tab === 'dataCollection') return item.group === 'data_collection'
    if (tab === 'securityAuth') return item.group === 'registration' || OIDC_KEYS.has(item.key)
    if (tab === 'userDefaults') return NEW_USER_KEYS.has(item.key)
    if (tab === 'messages') return item.group === 'other' && !BRAND_INFO_KEYS.has(item.key) && !OIDC_KEYS.has(item.key)
    return false
  })
}

export function groupTitleKey(group: string) {
  return {
    data_collection: 'admin.settings.groupDataCollection',
    other: 'admin.settings.groupOther',
    registration: 'admin.settings.groupRegistration',
    subscription: 'admin.settings.groupSubscription',
    traffic: 'admin.settings.groupTraffic',
  }[group]
}

export function inputMin(key: string) {
  if (key.endsWith('_interval_seconds')) return 5
  if (key.endsWith('_concurrency') || key.endsWith('_timeout_seconds')) return 1
  return 0
}

export function inputMax(key: string, drafts: Record<string, string>) {
  if (key.endsWith('_concurrency')) return 64
  if (key.endsWith('_timeout_seconds')) return Math.min(300, Number(drafts[intervalKeyForTimeout(key)] || 300))
  if (key.endsWith('_retry_attempts')) return 5
  return undefined
}

function intervalKeyForTimeout(key: string) {
  if (key.startsWith('ops_collect_')) return 'ops_collect_interval_seconds'
  if (key.startsWith('traffic_collect_')) return 'traffic_collect_interval_seconds'
  return key
}
