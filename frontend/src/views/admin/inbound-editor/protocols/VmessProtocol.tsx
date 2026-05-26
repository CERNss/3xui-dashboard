import { Form, Switch } from 'antd'
import { ProtocolClients } from '../ProtocolClients'

export function VmessProtocol() {
  return (
    <ProtocolClients
      title="VMess clients"
      fields={[
        { name: 'id', label: 'UUID', placeholder: 'client uuid' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: 'Expiry time', numeric: true },
        { name: 'enable', label: 'Enable', switch: true, defaultValue: true },
      ]}
    >
      <Form.Item name="disableInsecureEncryption" label="Disable insecure encryption" valuePropName="checked">
        <Switch />
      </Form.Item>
    </ProtocolClients>
  )
}
