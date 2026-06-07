import { Empty, Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { groupTitleKey } from './settingHelpers'
import { SettingRow } from './SettingRow'
import type { SettingsSectionProps } from './types'

export interface GenericSettingsSectionProps extends SettingsSectionProps {
  title: string
  description: string
  extra?: ReactNode
  /** Whether `extra` renders above the grouped item Cards (default) or
   * below them. SecurityAuth puts the OIDC panel at the bottom so the
   * primary "Registration" Cards land directly under the title. */
  extraPosition?: 'top' | 'bottom'
}

export function SettingsSection({
  title,
  description,
  extra,
  extraPosition = 'top',
  items,
  drafts,
  savingKey,
  onDraftChange,
  onSave,
  onReset,
}: GenericSettingsSectionProps) {
  const { t } = useTranslation()
  const grouped = items.reduce<Record<string, SettingItem[]>>((buckets, item) => {
    const group = item.group || 'other'
    buckets[group] = buckets[group] ?? []
    buckets[group].push(item)
    return buckets
  }, {})

  const groupEntries = Object.entries(grouped)
  const mergesSingleGroup = groupEntries.length === 1

  const itemsBlock = groupEntries.length === 0 && !extra ? (
    <section className="settings-empty-panel">
      <Empty description={t('admin.settings.emptySection')} />
    </section>
  ) : groupEntries.length > 0 ? (
    groupEntries.map(([group, rows]) => {
      const titleKey = groupTitleKey(group)
      return (
        <section className="settings-group-panel" key={group} aria-labelledby={`settings-group-${group}`}>
          <header className="settings-group-header">
            <Typography.Title id={`settings-group-${group}`} level={3}>
              {mergesSingleGroup ? title : titleKey ? t(titleKey) : group}
            </Typography.Title>
            {mergesSingleGroup ? <Typography.Text>{description}</Typography.Text> : null}
          </header>
          <div className="settings-group-rows">
            {rows.map((item) => (
              <SettingRow
                key={item.key}
                item={item}
                drafts={drafts}
                saving={savingKey === item.key}
                onDraftChange={onDraftChange}
                onSave={onSave}
                onReset={onReset}
              />
            ))}
          </div>
        </section>
      )
    })
  ) : null

  return (
    <div className="settings-section-stack">
      {mergesSingleGroup ? null : (
        <header className="settings-section-intro">
          <Typography.Title level={2}>{title}</Typography.Title>
          <Typography.Text>{description}</Typography.Text>
        </header>
      )}
      {extraPosition === 'top' ? extra : null}
      {itemsBlock}
      {extraPosition === 'bottom' ? extra : null}
    </div>
  )
}
