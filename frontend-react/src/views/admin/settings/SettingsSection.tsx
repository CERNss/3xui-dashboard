import { Card, Empty, Space, Typography } from 'antd'
import type { ReactNode } from 'react'
import type { SettingItem } from '@/api/admin/settings'
import { groupTitle } from './settingHelpers'
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
      {Object.keys(grouped).length === 0 ? (
        <Empty description="No settings in this section" />
      ) : (
        Object.entries(grouped).map(([group, rows]) => (
          <Card key={group} title={groupTitle(group)}>
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
        ))
      )}
    </Space>
  )
}
