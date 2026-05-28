import { Button, Form, Input, Modal, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

interface TLSConfigModalProps {
  open: boolean
  onClose: () => void
}

const FINGERPRINTS = ['', 'chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized']

export function TLSConfigModal({ open, onClose }: TLSConfigModalProps) {
  const { t } = useTranslation()
  return (
    <Modal
      title={t('admin.inboundEditor.stream.configureTLS')}
      open={open}
      onCancel={onClose}
      footer={<Button type="primary" onClick={onClose}>{t('admin.inboundEditor.stream.done')}</Button>}
      width={620}
      destroyOnClose={false}
      maskClosable
    >
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="tlsServerName" label={t('admin.inboundEditor.stream.serverName')}>
            <Input placeholder="example.com" style={{ width: 240 }} />
          </Form.Item>
          <Form.Item name="tlsAlpn" label="ALPN">
            <Select
              mode="multiple"
              style={{ minWidth: 220 }}
              options={['h2', 'http/1.1'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
        </Space>
        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="tlsFingerprint" label="Fingerprint">
            <Select
              style={{ width: 160 }}
              options={FINGERPRINTS.map((value) => ({
                label: value || t('admin.inboundEditor.stream.fingerprintNone'),
                value,
              }))}
            />
          </Form.Item>
          <Form.Item name="tlsAllowInsecure" label={t('admin.inboundEditor.stream.allowInsecure')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
        <Form.Item name="tlsCertificateFile" label={t('admin.inboundEditor.stream.certFile')}>
          <Input placeholder="/etc/letsencrypt/live/example.com/fullchain.pem" />
        </Form.Item>
        <Form.Item name="tlsKeyFile" label={t('admin.inboundEditor.stream.keyFile')}>
          <Input placeholder="/etc/letsencrypt/live/example.com/privkey.pem" />
        </Form.Item>
      </Space>
    </Modal>
  )
}
