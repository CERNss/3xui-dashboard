import { Alert, Form, Input, InputNumber, Space, Switch } from 'antd'
import { ProtocolClients } from '../ProtocolClients'

export function WireguardProtocol() {
  return (
    <ProtocolClients
      title="WireGuard peers"
      addLabel="Add peer"
      fields={[
        { name: 'publicKey', label: 'Public key', placeholder: 'peer public key' },
        { name: 'allowedIPs', label: 'Allowed IPs', placeholder: '10.0.0.2/32' },
        { name: 'endpoint', label: 'Endpoint', placeholder: 'optional endpoint' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'enable', label: 'Enable', switch: true, defaultValue: true },
      ]}
    >
      <Alert type="info" showIcon message="WireGuard uses peer rows and server secret-key settings instead of stream/sniffing." />
      <Space align="start" wrap>
        <Form.Item name="wireguardMtu" label="MTU">
          <InputNumber min={576} />
        </Form.Item>
        <Form.Item name="wireguardSecretKey" label="Secret key">
          <Input placeholder="server secret key" />
        </Form.Item>
        <Form.Item name="wireguardNoKernelTun" label="No kernel TUN" valuePropName="checked">
          <Switch />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
