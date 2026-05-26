import { Alert, Checkbox, Form, Input, Space } from 'antd'
import { useTranslation } from 'react-i18next'

export function AdvancedJsonForm() {
  const { t } = useTranslation()
  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert type="info" showIcon message={t('admin.inboundEditor.advanced.info')} />
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
  )
}
