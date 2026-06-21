import { Button, Select, Space, Switch, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { itemValue, localizedLabel } from './settingHelpers'
import type { Drafts } from './types'

export interface PreferencePanelProps {
  id: string
  title: string
  description: string
  children: ReactNode
  footer?: ReactNode
}

export function PreferencePanel({ children, description, footer, id, title }: PreferencePanelProps) {
  return (
    <section className="settings-preference-panel" aria-labelledby={`settings-preference-${id}`}>
      <header className="settings-preference-header">
        <Typography.Title id={`settings-preference-${id}`} level={3}>
          {title}
        </Typography.Title>
        <Typography.Text>{description}</Typography.Text>
      </header>
      <div className="settings-preference-rows">{children}</div>
      {footer ? <footer className="settings-preference-footer">{footer}</footer> : null}
    </section>
  )
}

export function PreferenceSwitchRow({
  description,
  drafts,
  item,
  offText,
  onDraftChange,
  onText,
  title,
}: {
  description: string
  drafts: Drafts
  item?: SettingItem
  offText: string
  onDraftChange: (key: string, value: string) => void
  onText: string
  title: string
}) {
  const { i18n } = useTranslation()
  if (!item) return null
  const label = localizedLabel(item, i18n.language)
  const checked = effectiveBool(item, drafts)

  return (
    <div className="settings-preference-row" data-setting-key={item.key}>
      <div className="settings-preference-copy">
        <Typography.Text strong>{title}</Typography.Text>
        <Typography.Text>{description}</Typography.Text>
      </div>
      <div className="settings-preference-control settings-preference-control--switch">
        <Switch aria-label={label} checked={checked} onChange={(next) => onDraftChange(item.key, next ? 'true' : 'false')} />
        <span className="settings-preference-state" data-active={checked}>
          {checked ? onText : offText}
        </span>
      </div>
    </div>
  )
}

export function PreferenceTagsRow({
  description,
  drafts,
  hint,
  item,
  onDraftChange,
  placeholder,
  title,
}: {
  description: string
  drafts: Drafts
  hint: string
  item?: SettingItem
  onDraftChange: (key: string, value: string) => void
  placeholder: string
  title: string
}) {
  const { i18n } = useTranslation()
  if (!item) return null
  const label = localizedLabel(item, i18n.language)
  const values = splitList(draftValue(item, drafts))

  return (
    <div className="settings-preference-row settings-preference-row--stacked" data-setting-key={item.key}>
      <div className="settings-preference-copy">
        <Typography.Text strong>{title}</Typography.Text>
        <Typography.Text>{description}</Typography.Text>
      </div>
      <Select
        aria-label={label}
        className="settings-preference-tags"
        mode="tags"
        placeholder={placeholder}
        tokenSeparators={[',', ' ']}
        value={values}
        onChange={(next) => onDraftChange(item.key, joinList(next))}
      />
      <Typography.Text className="settings-preference-hint">{hint}</Typography.Text>
    </div>
  )
}

export function PreferenceNumberRow({
  description,
  drafts,
  item,
  min,
  onDraftChange,
  title,
  unit,
}: {
  description: string
  drafts: Drafts
  item?: SettingItem
  min?: number
  onDraftChange: (key: string, value: string) => void
  title: string
  unit: string
}) {
  const { i18n } = useTranslation()
  if (!item) return null
  const label = localizedLabel(item, i18n.language)

  return (
    <div className="settings-preference-row" data-setting-key={item.key}>
      <div className="settings-preference-copy">
        <Typography.Text strong>{title}</Typography.Text>
        <Typography.Text>{description}</Typography.Text>
      </div>
      <label className="settings-preference-number">
        <input
          aria-label={label}
          min={min}
          type="number"
          value={draftValue(item, drafts)}
          onChange={(event) => onDraftChange(item.key, event.target.value)}
        />
        <span>{unit}</span>
      </label>
    </div>
  )
}

export function PreferenceActions({
  resetLabel,
  saveLabel,
  saving,
  canReset,
  canSave,
  onReset,
  onSave,
}: {
  resetLabel: string
  saveLabel: string
  saving?: boolean
  canReset: boolean
  canSave: boolean
  onReset: () => void
  onSave: () => void
}) {
  return (
    <Space size={10} wrap>
      {canReset ? (
        <Button loading={saving} onClick={onReset}>
          {resetLabel}
        </Button>
      ) : null}
      <Button type="primary" loading={saving} disabled={!canSave} onClick={onSave}>
        {saveLabel}
      </Button>
    </Space>
  )
}

export function draftValue(item: SettingItem | undefined, drafts: Drafts) {
  if (!item) return ''
  return drafts[item.key] ?? itemValue(item)
}

export function changedItems(items: SettingItem[], drafts: Drafts) {
  return items.filter((item) => draftValue(item, drafts) !== itemValue(item))
}

function effectiveBool(item: SettingItem, drafts: Drafts) {
  const raw = draftValue(item, drafts) || item.env_fallback || item.default
  return ['true', '1', 'yes', 'on'].includes(raw.trim().toLowerCase())
}

function splitList(value: string) {
  return value
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
}

function joinList(values: string[]) {
  return values
    .map((value) => value.trim())
    .filter(Boolean)
    .join(',')
}
