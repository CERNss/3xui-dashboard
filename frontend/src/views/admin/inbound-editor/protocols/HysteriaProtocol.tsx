import { Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function HysteriaProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.hysteriaTitle')}
      fields={[
        { name: 'auth', label: 'Auth', placeholder: t('admin.inboundEditor.clients.authPlaceholder') },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Space align="start" wrap>
        <Form.Item name="hysteriaSni" label="SNI">
          <Input placeholder="vpn.example.com" />
        </Form.Item>
        <Form.Item name="hysteriaAuth" label={t('admin.inboundEditor.authString')}>
          <Input placeholder={t('admin.inboundEditor.authStringPlaceholder')} />
        </Form.Item>
        <Form.Item name="hysteriaObfs" label={t('admin.inboundEditor.obfuscation')}>
          <Input placeholder={t('admin.inboundEditor.obfuscationPlaceholder')} />
        </Form.Item>
        <Form.Item name="hysteriaUpMbps" label={t('admin.inboundEditor.upMbps')}>
          <InputNumber min={0} />
        </Form.Item>
        <Form.Item name="hysteriaDownMbps" label={t('admin.inboundEditor.downMbps')}>
          <InputNumber min={0} />
        </Form.Item>
        <Form.Item name="tlsFingerprint" label="Fingerprint">
          <Select
            style={{ width: 160 }}
            options={['', 'chrome', 'firefox', 'safari', 'ios', 'android', 'randomized'].map((value) => ({
              label: value || t('admin.inboundEditor.stream.fingerprintNone'),
              value,
            }))}
          />
        </Form.Item>
        <Form.Item name="tlsAllowInsecure" label={t('admin.inboundEditor.stream.allowInsecure')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="tlsCertificateFile" label={t('admin.inboundEditor.stream.certFile')}>
          <Input placeholder="/etc/letsencrypt/live/example.com/fullchain.pem" />
        </Form.Item>
        <Form.Item name="tlsKeyFile" label={t('admin.inboundEditor.stream.keyFile')}>
          <Input placeholder="/etc/letsencrypt/live/example.com/privkey.pem" />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
