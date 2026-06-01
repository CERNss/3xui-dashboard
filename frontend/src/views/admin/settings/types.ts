import type { SettingItem } from '@/api/admin/settings'

export type SettingsTab =
  | 'general'
  | 'subscription'
  | 'payment'
  | 'alerts'
  | 'dataCollection'
  | 'securityAuth'
  | 'userDefaults'
  | 'messages'
  | 'notifications'

export type Drafts = Record<string, string>

export interface SettingsSectionProps {
  items: SettingItem[]
  drafts: Drafts
  savingKey?: string | null
  onDraftChange: (key: string, value: string) => void
  onSave: (item: SettingItem) => void
  onReset: (item: SettingItem) => void
}
