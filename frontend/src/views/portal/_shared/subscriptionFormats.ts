import type { TFunction } from 'i18next'

export type SubscriptionFormatKey =
  | 'base64'
  | 'clash'
  | 'singbox'
  | 'sip008'
  | 'wireguard'
  | 'wireguard-zip'
  | 'json'

export interface SubscriptionFormatInfo {
  key: SubscriptionFormatKey
  label: string
  hint: string
  apps: string
  downloadOnly?: boolean
}

const I18N_KEYS: Record<SubscriptionFormatKey, string> = {
  base64: 'base64',
  clash: 'clash',
  singbox: 'singbox',
  sip008: 'sip008',
  wireguard: 'wireguard',
  'wireguard-zip': 'wireguardZip',
  json: 'json',
}

export function subscriptionFormats(t: TFunction): SubscriptionFormatInfo[] {
  return [
    format('base64', 'Base64', t),
    format('clash', 'Clash', t),
    format('singbox', 'Sing-box', t),
    format('sip008', 'SIP008', t),
    format('wireguard', 'WireGuard', t),
    format('wireguard-zip', 'WG (ZIP)', t, true),
    format('json', 'JSON', t),
  ]
}

export function subscriptionUrl(origin: string, subId: string, formatKey: SubscriptionFormatKey): string {
  const base = `${origin}/sub/${subId}`
  return formatKey === 'base64' ? base : `${base}?format=${formatKey}`
}

function format(
  key: SubscriptionFormatKey,
  label: string,
  t: TFunction,
  downloadOnly = false,
): SubscriptionFormatInfo {
  const i18nKey = I18N_KEYS[key]
  return {
    key,
    label,
    hint: t(`portal.subscription.formats.${i18nKey}.hint`),
    apps: t(`portal.subscription.formats.${i18nKey}.apps`),
    downloadOnly,
  }
}
