import { Alert, Checkbox, Form, Input, Space } from 'antd'

export function AdvancedJsonForm() {
  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert type="info" showIcon message="Enable an override to submit raw JSON instead of the generated settings." />
      <Form.Item name="advSettingsOverride" valuePropName="checked">
        <Checkbox>Override settings JSON</Checkbox>
      </Form.Item>
      <Form.Item name="advSettings" label="settings">
        <Input.TextArea rows={8} spellCheck={false} />
      </Form.Item>
      <Form.Item name="advStreamOverride" valuePropName="checked">
        <Checkbox>Override streamSettings JSON</Checkbox>
      </Form.Item>
      <Form.Item name="advStream" label="streamSettings">
        <Input.TextArea rows={8} spellCheck={false} />
      </Form.Item>
      <Form.Item name="advSniffingOverride" valuePropName="checked">
        <Checkbox>Override sniffing JSON</Checkbox>
      </Form.Item>
      <Form.Item name="advSniffing" label="sniffing">
        <Input.TextArea rows={8} spellCheck={false} />
      </Form.Item>
    </Space>
  )
}
