import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import { Button, Card, Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

export function TunnelProtocol() {
  const { t } = useTranslation()
  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Space align="start" wrap>
        <Form.Item
          name="tunnelRewriteAddress"
          label={t('admin.inboundEditor.tunnel.rewriteAddress')}
          rules={[{ required: true }]}
        >
          <Input placeholder="example.com" style={{ width: 240 }} />
        </Form.Item>
        <Form.Item
          name="tunnelRewritePort"
          label={t('admin.inboundEditor.tunnel.rewritePort')}
          rules={[{ required: true, type: 'number', min: 0, max: 65535 }]}
        >
          <InputNumber min={0} max={65535} />
        </Form.Item>
        <Form.Item name="tunnelAllowedNetwork" label={t('admin.inboundEditor.tunnel.allowedNetwork')}>
          <Select
            style={{ width: 160 }}
            options={[
              { label: 'TCP + UDP', value: 'tcp,udp' },
              { label: 'TCP', value: 'tcp' },
              { label: 'UDP', value: 'udp' },
            ]}
          />
        </Form.Item>
        <Form.Item name="tunnelFollowRedirect" label={t('admin.inboundEditor.tunnel.followRedirect')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Space>

      <Form.List name="tunnelPortMap">
        {(items, { add, remove }) => (
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Space style={{ justifyContent: 'space-between', width: '100%' }}>
              <strong>{t('admin.inboundEditor.tunnel.portMap')}</strong>
              <Button size="small" icon={<PlusOutlined />} onClick={() => add({ name: '', value: '' })}>
                {t('admin.inboundEditor.tunnel.addPortMap')}
              </Button>
            </Space>
            {items.length === 0 ? <Card size="small">{t('admin.inboundEditor.tunnel.portMapEmpty')}</Card> : null}
            {items.map((item) => (
              <Space key={item.key} align="baseline" wrap>
                <Form.Item name={[item.name, 'name']} label={t('admin.inboundEditor.tunnel.portMapName')}>
                  <Input placeholder="5555" style={{ width: 120 }} />
                </Form.Item>
                <Form.Item name={[item.name, 'value']} label={t('admin.inboundEditor.tunnel.portMapValue')}>
                  <Input placeholder="1.1.1.1:7777" style={{ width: 240 }} />
                </Form.Item>
                <Button danger size="small" icon={<MinusCircleOutlined />} onClick={() => remove(item.name)} />
              </Space>
            ))}
          </Space>
        )}
      </Form.List>
    </Space>
  )
}
