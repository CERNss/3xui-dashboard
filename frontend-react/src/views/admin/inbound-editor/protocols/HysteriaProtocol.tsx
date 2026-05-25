import { Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import { ProtocolClients } from '../ProtocolClients'

export function HysteriaProtocol() {
  return (
    <ProtocolClients
      title="Hysteria auth clients"
      fields={[
        { name: 'auth', label: 'Auth', placeholder: 'auth secret' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: 'Expiry time', numeric: true },
        { name: 'enable', label: 'Enable', switch: true, defaultValue: true },
      ]}
    >
      <Space align="start" wrap>
        <Form.Item name="hysteriaSni" label="SNI">
          <Input placeholder="vpn.example.com" />
        </Form.Item>
        <Form.Item name="hysteriaAuth" label="Auth string">
          <Input placeholder="shared auth string" />
        </Form.Item>
        <Form.Item name="hysteriaObfs" label="Obfuscation">
          <Input placeholder="optional obfs password" />
        </Form.Item>
        <Form.Item name="hysteriaUpMbps" label="Up Mbps">
          <InputNumber min={0} />
        </Form.Item>
        <Form.Item name="hysteriaDownMbps" label="Down Mbps">
          <InputNumber min={0} />
        </Form.Item>
        <Form.Item name="tlsFingerprint" label="Fingerprint">
          <Select
            style={{ width: 160 }}
            options={['', 'chrome', 'firefox', 'safari', 'ios', 'android', 'randomized'].map((value) => ({
              label: value || 'none',
              value,
            }))}
          />
        </Form.Item>
        <Form.Item name="tlsAllowInsecure" label="Allow insecure" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="tlsCertificateFile" label="Certificate file">
          <Input placeholder="/etc/letsencrypt/live/example.com/fullchain.pem" />
        </Form.Item>
        <Form.Item name="tlsKeyFile" label="Key file">
          <Input placeholder="/etc/letsencrypt/live/example.com/privkey.pem" />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
