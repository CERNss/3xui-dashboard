import { Form, Input, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'
import { randomLowerAlnum } from '../random'

export function MixedProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  const udp = Form.useWatch('mixedUdp')
  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.mixedTitle')}
      addLabel={t('admin.inboundEditor.clients.addAccount')}
      fields={[
        { name: 'user', label: t('admin.inboundEditor.clients.user'), placeholder: 'alice', defaultValue: () => randomLowerAlnum(8) },
        { name: 'pass', label: t('admin.inboundEditor.clients.pass'), placeholder: '••••••', defaultValue: () => randomLowerAlnum(12) },
      ]}
    >
      <Space align="start" wrap>
        <Form.Item name="mixedAuth" label={t('admin.inboundEditor.auth')}>
          <Select
            style={{ width: 160 }}
            options={[
              { label: 'noauth', value: 'noauth' },
              { label: 'password', value: 'password' },
            ]}
          />
        </Form.Item>
        <Form.Item name="mixedUdp" label="UDP" valuePropName="checked">
          <Switch />
        </Form.Item>
        {udp ? (
          <Form.Item name="mixedUdpIP" label={t('admin.inboundEditor.udpIP')}>
            <Input placeholder="127.0.0.1" />
          </Form.Item>
        ) : null}
      </Space>
    </ProtocolClients>
  )
}
