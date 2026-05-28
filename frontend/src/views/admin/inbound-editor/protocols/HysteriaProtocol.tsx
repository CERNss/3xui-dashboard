import { SettingOutlined } from '@ant-design/icons'
import { Button, Form, Input, InputNumber, Select, Space, Switch, Typography } from 'antd'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { HysteriaMasqueradeModal } from '../HysteriaMasqueradeModal'
import { ProtocolClients } from '../ProtocolClients'

export function HysteriaProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const masqType = Form.useWatch<string>('hysteriaMasqueradeType', form) ?? ''
  const [masqOpen, setMasqOpen] = useState(false)
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
        <Form.Item name="hysteriaVersion" label={t('admin.inboundEditor.hysteriaVersion')}>
          <Select
            style={{ width: 100 }}
            options={[
              { label: 'v1', value: 1 },
              { label: 'v2', value: 2 },
            ]}
          />
        </Form.Item>
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
        <Form.Item name="hysteriaUdpIdleTimeout" label={t('admin.inboundEditor.hysteriaUdpIdleTimeout')}>
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
      <Space size={12} align="center">
        <Button icon={<SettingOutlined />} onClick={() => setMasqOpen(true)}>
          {t('admin.inboundEditor.hysteriaMasquerade.configure')}
        </Button>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {masqType ? t('admin.inboundEditor.hysteriaMasquerade.summary', { type: masqType }) : t('admin.inboundEditor.hysteriaMasquerade.summaryNone')}
        </Typography.Text>
      </Space>
      <HysteriaMasqueradeModal open={masqOpen} onClose={() => setMasqOpen(false)} />
    </ProtocolClients>
  )
}
