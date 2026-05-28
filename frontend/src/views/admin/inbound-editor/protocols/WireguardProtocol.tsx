import { Alert, Form, Input, InputNumber, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function WireguardProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.wireguardTitle')}
      addLabel={t('admin.inboundEditor.clients.addPeer')}
      fields={[
        { name: 'publicKey', label: t('admin.inboundEditor.stream.publicKey'), placeholder: t('admin.inboundEditor.clients.peerPublicKeyPlaceholder') },
        { name: 'allowedIPs', label: t('admin.inboundEditor.allowedIPs'), placeholder: '10.0.0.2/32' },
        { name: 'endpoint', label: 'Endpoint', placeholder: t('admin.inboundEditor.clients.endpointPlaceholder') },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Alert type="info" showIcon message={t('admin.inboundEditor.wireguardPeerInfo')} />
      <Space align="start" wrap>
        <Form.Item name="wireguardMtu" label="MTU">
          <InputNumber min={576} />
        </Form.Item>
        <Form.Item name="wireguardSecretKey" label={t('admin.inboundEditor.secretKey')}>
          <Input placeholder={t('admin.inboundEditor.serverSecretKeyPlaceholder')} />
        </Form.Item>
        <Form.Item name="wireguardNoKernelTun" label={t('admin.inboundEditor.noKernelTun')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
