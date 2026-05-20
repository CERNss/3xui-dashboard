import { adminClient } from '../client/admin'

export interface SettingItem {
  key: string
  label: string
  type: 'bool' | 'int' | 'string'
  group: 'registration' | 'subscription' | 'traffic' | 'other'
  default: string
  description: string
  value: string
  has_override: boolean
  env_fallback: string
}

export const settingsApi = {
  list: () =>
    adminClient
      .get<{ settings: SettingItem[] }>('/settings')
      .then((r) => r.data.settings),

  set: (key: string, value: string) =>
    adminClient.put<{ key: string; value: string }>(`/settings/${encodeURIComponent(key)}`, { value }),

  clear: (key: string) =>
    adminClient.delete<void>(`/settings/${encodeURIComponent(key)}`),
}
