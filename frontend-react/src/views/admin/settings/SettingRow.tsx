import { Button, Card, Input, Space, Typography } from 'antd'
import type { SettingItem } from '@/api/admin/settings'
import { inputMax, inputMin, itemValue, localizedLabel } from './settingHelpers'
import type { Drafts } from './types'

export interface SettingRowProps {
  item: SettingItem
  drafts: Drafts
  saving?: boolean
  onDraftChange: (key: string, value: string) => void
  onSave: (item: SettingItem) => void
  onReset: (item: SettingItem) => void
}

export function SettingRow({ item, drafts, saving, onDraftChange, onSave, onReset }: SettingRowProps) {
  const draft = drafts[item.key] ?? itemValue(item)
  const changed = draft !== itemValue(item)
  const controlID = `setting-${item.key}`

  return (
    <Card data-setting-key={item.key} size="small" styles={{ body: { padding: 16 } }}>
      <div style={{ display: 'grid', gap: 16, gridTemplateColumns: 'minmax(220px, 0.42fr) minmax(0, 1fr)' }}>
        <Space direction="vertical" size={4}>
          <label htmlFor={controlID} style={{ fontWeight: 600 }}>
            {localizedLabel(item)}
          </label>
          {item.description ? <Typography.Text type="secondary">{item.description}</Typography.Text> : null}
          {item.env_fallback ? <Typography.Text type="secondary">Fallback: {item.env_fallback}</Typography.Text> : null}
        </Space>
        <Space direction="vertical" size={8} style={{ alignItems: 'flex-end', width: '100%' }}>
          {item.type === 'bool' ? (
            <select
              aria-label={localizedLabel(item)}
              id={controlID}
              style={{ border: '1px solid #d9d9d9', borderRadius: 6, height: 32, padding: '0 11px', width: 200 }}
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            >
              <option value="">Use default</option>
              <option value="true">true</option>
              <option value="false">false</option>
            </select>
          ) : item.type === 'int' ? (
            <input
              aria-label={localizedLabel(item)}
              id={controlID}
              max={inputMax(item.key, drafts)}
              min={inputMin(item.key)}
              style={{ border: '1px solid #d9d9d9', borderRadius: 6, height: 32, padding: '0 11px', width: 200 }}
              type="number"
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            />
          ) : item.key.includes('template_') ? (
            <Input.TextArea
              aria-label={localizedLabel(item)}
              id={controlID}
              rows={8}
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            />
          ) : (
            <Input
              aria-label={localizedLabel(item)}
              id={controlID}
              style={{ maxWidth: 420 }}
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            />
          )}
          <Space wrap>
            <Button type="primary" size="small" disabled={!changed} loading={saving} onClick={() => onSave(item)}>
              Save
            </Button>
            {item.has_override ? (
              <Button size="small" loading={saving} onClick={() => onReset(item)}>
                Reset
              </Button>
            ) : null}
          </Space>
        </Space>
      </div>
    </Card>
  )
}
