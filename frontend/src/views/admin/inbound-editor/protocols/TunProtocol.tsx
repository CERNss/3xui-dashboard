import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import { Button, Form, Input, InputNumber, Space } from 'antd'
import { useTranslation } from 'react-i18next'

interface StringListProps {
  name: 'tunGateway' | 'tunDns' | 'tunAutoSystemRoutingTable'
  label: string
  addLabel: string
  placeholders: [string, string]
}

function StringListField({ name, label, addLabel, placeholders }: StringListProps) {
  return (
    <Form.List name={name}>
      {(items, { add, remove }) => (
        <Space direction="vertical" size={4} style={{ width: '100%' }}>
          <Space style={{ justifyContent: 'space-between', width: '100%' }}>
            <strong>{label}</strong>
            <Button size="small" icon={<PlusOutlined />} onClick={() => add('')}>
              {addLabel}
            </Button>
          </Space>
          {items.map((item, idx) => (
            <Space key={item.key} align="baseline" style={{ width: '100%' }}>
              <Form.Item name={[item.name]} noStyle>
                <Input style={{ width: 320 }} placeholder={idx === 0 ? placeholders[0] : placeholders[1]} />
              </Form.Item>
              <Button danger size="small" icon={<MinusCircleOutlined />} onClick={() => remove(item.name)} />
            </Space>
          ))}
        </Space>
      )}
    </Form.List>
  )
}

export function TunProtocol() {
  const { t } = useTranslation()
  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Space align="start" wrap>
        <Form.Item name="tunName" label={t('admin.inboundEditor.tun.interfaceName')}>
          <Input placeholder="xray0" style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="tunMtu" label="MTU">
          <InputNumber min={0} />
        </Form.Item>
        <Form.Item name="tunUserLevel" label={t('admin.inboundEditor.tun.userLevel')}>
          <InputNumber min={0} />
        </Form.Item>
        <Form.Item name="tunAutoOutboundsInterface" label={t('admin.inboundEditor.tun.autoOutboundsInterface')}>
          <Input placeholder="auto" />
        </Form.Item>
      </Space>
      <StringListField
        name="tunGateway"
        label={t('admin.inboundEditor.tun.gateway')}
        addLabel={t('admin.inboundEditor.tun.addGateway')}
        placeholders={['10.0.0.1/16', 'fc00::1/64']}
      />
      <StringListField
        name="tunDns"
        label="DNS"
        addLabel={t('admin.inboundEditor.tun.addDns')}
        placeholders={['1.1.1.1', '8.8.8.8']}
      />
      <StringListField
        name="tunAutoSystemRoutingTable"
        label={t('admin.inboundEditor.tun.autoSystemRoutes')}
        addLabel={t('admin.inboundEditor.tun.addRoute')}
        placeholders={['0.0.0.0/0', '::/0']}
      />
    </Space>
  )
}
