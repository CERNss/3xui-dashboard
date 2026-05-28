import { Form, Input, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

const SS_METHODS = [
  'chacha20-ietf-poly1305',
  'chacha20-poly1305',
  'xchacha20-ietf-poly1305',
  'aes-256-gcm',
  'aes-128-gcm',
  '2022-blake3-aes-128-gcm',
  '2022-blake3-aes-256-gcm',
  '2022-blake3-chacha20-poly1305',
]

function isSS2022(method: string) {
  return method?.startsWith('2022-')
}

export function ShadowsocksProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  const ssMethod = Form.useWatch<string>('ssMethod')
  const ss2022 = isSS2022(ssMethod ?? '')
  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.shadowsocksTitle')}
      fields={[
        { name: 'password', label: 'Password', placeholder: t('admin.inboundEditor.clients.passwordPlaceholder') },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Space align="start" wrap>
        <Form.Item name="ssMethod" label={t('admin.inboundEditor.ssMethod')} rules={[{ required: true }]}>
          <Select
            style={{ width: 260 }}
            options={SS_METHODS.map((value) => ({ label: value, value }))}
          />
        </Form.Item>
        <Form.Item name="ssNetwork" label={t('admin.inboundEditor.ssNetwork')}>
          <Select
            style={{ width: 140 }}
            options={[
              { label: 'tcp+udp', value: 'tcp,udp' },
              { label: t('admin.inboundEditor.networkTcpOnly'), value: 'tcp' },
              { label: t('admin.inboundEditor.networkUdpOnly'), value: 'udp' },
            ]}
          />
        </Form.Item>
        <Form.Item name="ssIvCheck" label="ivCheck" valuePropName="checked">
          <Switch />
        </Form.Item>
        {ss2022 ? (
          <Form.Item
            name="ssPassword"
            label={t('admin.inboundEditor.globalPassword')}
            tooltip={t('admin.inboundEditor.ssPasswordTooltip')}
            rules={[{ required: true, message: t('admin.inboundEditor.ssPasswordRequired') }]}
          >
            <Input placeholder={t('admin.inboundEditor.globalPasswordPlaceholder')} />
          </Form.Item>
        ) : null}
      </Space>
    </ProtocolClients>
  )
}
