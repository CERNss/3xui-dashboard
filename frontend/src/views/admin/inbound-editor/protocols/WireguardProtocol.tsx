import { Alert, Button, Form, Input, InputNumber, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function WireguardProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const generateServerKeypair = Form.useWatch('wireguardGenerateKeypair', form)

  const clearServerKeypair = () => {
    form.setFieldsValue({
      wireguardSecretKey: '',
      wireguardPublicKey: '',
      wireguardGenerateKeypair: false,
    })
  }

  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.wireguardTitle')}
      addLabel={t('admin.inboundEditor.clients.addPeer')}
      fields={[
        { name: 'publicKey', label: t('admin.inboundEditor.stream.publicKey'), placeholder: t('admin.inboundEditor.clients.peerPublicKeyPlaceholder') },
        { name: 'allowedIPs', label: t('admin.inboundEditor.allowedIPs'), placeholder: '10.0.0.2/32' },
        { name: 'endpoint', label: 'Endpoint', placeholder: t('admin.inboundEditor.clients.endpointPlaceholder') },
        { name: 'preSharedKey', label: t('admin.inboundEditor.preSharedKey'), placeholder: t('admin.inboundEditor.preSharedKeyPlaceholder') },
        { name: 'keepAlive', label: t('admin.inboundEditor.keepAlive'), numeric: true },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Alert type="info" showIcon message={t('admin.inboundEditor.wireguardPeerInfo')} />
      <Space align="start" wrap>
        <Form.Item name="wireguardMtu" label="MTU">
          <InputNumber min={576} />
        </Form.Item>
        <Form.Item name="wireguardNoKernelTun" label={t('admin.inboundEditor.noKernelTun')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Space>
      <Space align="start" wrap>
        <Form.Item name="wireguardSecretKey" label={t('admin.inboundEditor.secretKey')}>
          <Input style={{ width: 320 }} disabled={generateServerKeypair} placeholder={t('admin.inboundEditor.serverSecretKeyPlaceholder')} />
        </Form.Item>
        <Form.Item name="wireguardPublicKey" label={t('admin.inboundEditor.stream.publicKey')}>
          <Input style={{ width: 320 }} disabled placeholder={t('admin.inboundEditor.wgPublicKeyPlaceholder')} />
        </Form.Item>
      </Space>
      <Space size={12} align="center">
        <Form.Item name="wireguardGenerateKeypair" valuePropName="checked" noStyle>
          <Switch />
        </Form.Item>
        <span style={{ color: '#888' }}>{t('admin.inboundEditor.wgGenerateKeypair')}</span>
        <Button size="small" onClick={clearServerKeypair}>{t('admin.inboundEditor.stream.clear')}</Button>
      </Space>
    </ProtocolClients>
  )
}
