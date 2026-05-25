import { adminClient } from '../client/admin'

export interface SettingItem {
  key: string
  label: string
  type: 'bool' | 'int' | 'string'
  group: 'registration' | 'subscription' | 'traffic' | 'data_collection' | 'other'
  default: string
  description: string
  value: string
  has_override: boolean
  env_fallback: string
  // Optional Chinese variants. Backend started populating these so
  // the admin UI can render localized labels/descriptions for the
  // setting definitions themselves (which are defined server-side).
  // Fallback to label/description if the backend hasn't shipped yet.
  label_zh?: string
  description_zh?: string
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

  uploadBrandIcon: (file: File) => {
    const body = new FormData()
    body.append('file', file)
    return adminClient
      .post<{ key: string; value: string; url: string; content_type: string; size: number }>(
        '/settings/branding/icon',
        body,
        { headers: { 'Content-Type': 'multipart/form-data' } },
      )
      .then((r) => r.data)
  },

  smtpTest: (to: string) =>
    adminClient
      .post<{ status: string; to: string }>('/settings/smtp-test', { to })
      .then((r) => r.data),
}
