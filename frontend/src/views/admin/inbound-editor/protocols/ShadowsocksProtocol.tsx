import { Form, Input, Select, Space } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function ShadowsocksProtocol() {
  const { t } = useTranslation()
  return (
    <ProtocolClients
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
            options={[
              'chacha20-ietf-poly1305',
              'aes-256-gcm',
              'aes-128-gcm',
              '2022-blake3-aes-128-gcm',
              '2022-blake3-aes-256-gcm',
              '2022-blake3-chacha20-poly1305',
            ].map((value) => ({ label: value, value }))}
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
        <Form.Item name="ssPassword" label={t('admin.inboundEditor.globalPassword')}>
          <Input placeholder={t('admin.inboundEditor.globalPasswordPlaceholder')} />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
