import { CopyOutlined, DownloadOutlined } from '@ant-design/icons'
import { Alert, Button, Checkbox, Form, Input, Space, Switch, Tabs, Typography, message } from 'antd'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { blankInboundValues, valuesToInboundBody } from './model'
import type { InboundEditorValues } from './types'

function prettyParse(jsonStr: string): string {
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2)
  } catch {
    return jsonStr
  }
}

function downloadBlob(filename: string, content: string) {
  const blob = new Blob([content], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export function AdvancedJsonForm() {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const [editMode, setEditMode] = useState(false)
  const rawValues = Form.useWatch(() => form.getFieldsValue(true), form) ?? {}
  // The form may not be fully populated on first render (e.g. when the
  // editor opens in create mode and AdvancedJsonForm mounts before
  // setFieldsValue runs). Merge with blank defaults so the serializer
  // never trips on undefined arrays / numbers.
  const values = { ...blankInboundValues(null), ...(rawValues as object) } as InboundEditorValues

  const body = valuesToInboundBody(values)
  const settingsJSON = prettyParse(body.settings ?? '{}')
  const streamJSON = prettyParse(body.streamSettings ?? '{}')
  const sniffJSON = prettyParse(body.sniffing ?? '{}')
  const fullObj = {
    listen: body.listen ?? '',
    port: body.port ?? 0,
    protocol: body.protocol ?? '',
    remark: body.remark ?? '',
    settings: tryParseOrRaw(body.settings),
    streamSettings: tryParseOrRaw(body.streamSettings),
    sniffing: tryParseOrRaw(body.sniffing),
    tag: body.tag ?? '',
  }
  const fullJSON = JSON.stringify(fullObj, null, 2)

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      message.success(t('admin.inboundEditor.advanced.copied'))
    } catch {
      message.error(t('admin.inboundEditor.advanced.copyFailed'))
    }
  }

  const filenameFor = (key: string) => {
    const slug = (body.tag || body.remark || 'inbound').toString().replace(/[^a-z0-9_-]+/gi, '-').toLowerCase()
    return key === 'full' ? `${slug}.json` : `${slug}.${key}.json`
  }

  const viewTab = (label: string, key: string, payload: string) => ({
    key,
    label,
    children: (
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Space size={8}>
          <Button size="small" icon={<CopyOutlined />} onClick={() => copy(payload)}>
            {t('admin.inboundEditor.advanced.copy')}
          </Button>
          <Button size="small" icon={<DownloadOutlined />} onClick={() => downloadBlob(filenameFor(key), payload)}>
            {t('admin.inboundEditor.advanced.download')}
          </Button>
        </Space>
        <pre
          style={{
            margin: 0,
            padding: 12,
            background: 'rgba(0,0,0,0.45)',
            border: '1px solid rgba(255,255,255,0.06)',
            borderRadius: 6,
            maxHeight: 380,
            overflow: 'auto',
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
            fontSize: 12,
            lineHeight: 1.5,
            color: '#d1d5db',
            whiteSpace: 'pre',
          }}
        >
          {payload}
        </pre>
      </Space>
    ),
  })

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Space align="center">
        <Switch checked={editMode} onChange={setEditMode} />
        <Typography.Text>{t('admin.inboundEditor.advanced.editMode')}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {editMode
            ? t('admin.inboundEditor.advanced.editModeHint')
            : t('admin.inboundEditor.advanced.viewModeHint')}
        </Typography.Text>
      </Space>

      {!editMode ? (
        <Tabs
          items={[
            viewTab(t('admin.inboundEditor.advanced.tabAll'), 'full', fullJSON),
            viewTab('settings', 'settings', settingsJSON),
            viewTab('sniffing', 'sniffing', sniffJSON),
            viewTab('streamSettings', 'streamSettings', streamJSON),
          ]}
        />
      ) : (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert type="warning" showIcon message={t('admin.inboundEditor.advanced.info')} />
          <Form.Item name="advSettingsOverride" valuePropName="checked">
            <Checkbox>{t('admin.inboundEditor.advanced.overrideSettings')}</Checkbox>
          </Form.Item>
          <Form.Item name="advSettings" label="settings">
            <Input.TextArea rows={8} spellCheck={false} />
          </Form.Item>
          <Form.Item name="advStreamOverride" valuePropName="checked">
            <Checkbox>{t('admin.inboundEditor.advanced.overrideStream')}</Checkbox>
          </Form.Item>
          <Form.Item name="advStream" label="streamSettings">
            <Input.TextArea rows={8} spellCheck={false} />
          </Form.Item>
          <Form.Item name="advSniffingOverride" valuePropName="checked">
            <Checkbox>{t('admin.inboundEditor.advanced.overrideSniffing')}</Checkbox>
          </Form.Item>
          <Form.Item name="advSniffing" label="sniffing">
            <Input.TextArea rows={8} spellCheck={false} />
          </Form.Item>
        </Space>
      )}
    </Space>
  )
}

function tryParseOrRaw(s?: string): unknown {
  if (!s) return {}
  try {
    return JSON.parse(s)
  } catch {
    return s
  }
}
