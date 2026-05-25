import { Button, Drawer, Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import type { FormInstance } from 'antd'
import { AREA_OPTIONS, parsePanelURL, type NodeFormValues } from './utils'

interface NodeDrawerProps {
  editingName?: string
  form: FormInstance<NodeFormValues>
  open: boolean
  saving?: boolean
  onClose: () => void
  onSubmit: () => void
}

export function NodeDrawer({ editingName, form, open, saving, onClose, onSubmit }: NodeDrawerProps) {
  const isEditing = Boolean(editingName)

  const applyQuickURL = (value: string) => {
    const parsed = parsePanelURL(value)
    if (!parsed) return
    form.setFieldsValue(parsed)
  }

  return (
    <Drawer
      title={isEditing ? `Edit ${editingName}` : 'New Node'}
      open={open}
      width={560}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>
            Cancel
          </Button>
          <Button type="primary" loading={saving} onClick={onSubmit}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
        </Space>
      }
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item label="Quick panel URL" tooltip="Paste a 3x-ui panel URL to fill protocol, host, port, and base path.">
          <Input
            aria-label="Quick panel URL"
            placeholder="https://node.example.com:2053/panel/"
            onChange={(event) => applyQuickURL(event.target.value)}
          />
        </Form.Item>

        <Form.Item name="name" label="Name" rules={[{ required: true, whitespace: true, message: 'Name is required' }]}>
          <Input placeholder="Tokyo edge 1" />
        </Form.Item>

        <Space align="start" style={{ width: '100%' }} wrap>
          <Form.Item name="area" label="Area" rules={[{ required: true, message: 'Area is required' }]}>
            <Select
              style={{ minWidth: 180 }}
              options={AREA_OPTIONS.map((area) => ({ label: area.label, value: area.key }))}
            />
          </Form.Item>
          <Form.Item
            name="province"
            label="Province"
            rules={[{ required: true, whitespace: true, message: 'Province is required' }]}
          >
            <Input placeholder="Tokyo" />
          </Form.Item>
        </Space>

        <Space align="start" style={{ width: '100%' }} wrap>
          <Form.Item name="scheme" label="Scheme" rules={[{ required: true, message: 'Scheme is required' }]}>
            <Select
              style={{ width: 120 }}
              options={[
                { label: 'https', value: 'https' },
                { label: 'http', value: 'http' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="port"
            label="Port"
            rules={[
              { required: true, type: 'number', message: 'Port is required' },
              { type: 'number', min: 1, max: 65535, message: 'Port must be between 1 and 65535' },
            ]}
          >
            <InputNumber precision={0} />
          </Form.Item>
        </Space>

        <Form.Item name="host" label="Host" rules={[{ required: true, whitespace: true, message: 'Host is required' }]}>
          <Input placeholder="node.example.com" />
        </Form.Item>

        <Form.Item name="base_path" label="Base path">
          <Input placeholder="/panel/" />
        </Form.Item>

        <Form.Item
          name="api_token"
          label="API token"
          rules={isEditing ? [] : [{ required: true, whitespace: true, message: 'API token is required' }]}
          extra={isEditing ? 'Leave blank to keep the current token.' : undefined}
        >
          <Input placeholder={isEditing ? 'Keep current token' : 'Bearer token'} />
        </Form.Item>

        <Form.Item name="enabled" label="Enabled" valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </Drawer>
  )
}
