import { Card, Empty, Space, Typography } from 'antd'
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
}

export function SettingsSection({
  title,
  description,
  extra,
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

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card>
        <Typography.Title level={4} style={{ marginTop: 0 }}>
          {title}
        </Typography.Title>
        <Typography.Text type="secondary">{description}</Typography.Text>
      </Card>
      {extra}
      {Object.keys(grouped).length === 0 && !extra ? (
        <Empty description={t('admin.settings.emptySection')} />
      ) : Object.keys(grouped).length > 0 ? (
        Object.entries(grouped).map(([group, rows]) => {
          const titleKey = groupTitleKey(group)
          return (
            <Card key={group} title={titleKey ? t(titleKey) : group}>
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
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
              </Space>
            </Card>
          )
        })
      ) : null}
    </Space>
  )
}
