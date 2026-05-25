import { Form, Select } from 'antd'
import { ProtocolClients } from '../ProtocolClients'

export function VlessProtocol() {
  return (
    <ProtocolClients
      title="VLESS clients"
      fields={[
        { name: 'id', label: 'UUID', placeholder: 'client uuid' },
        { name: 'flow', label: 'Flow', placeholder: 'xtls-rprx-vision' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: 'Expiry time', numeric: true },
        { name: 'enable', label: 'Enable', switch: true, defaultValue: true },
      ]}
    >
      <Form.Item name="decryption" label="Decryption" rules={[{ required: true }]}>
        <Select options={[{ label: 'none', value: 'none' }]} />
      </Form.Item>
    </ProtocolClients>
  )
}
