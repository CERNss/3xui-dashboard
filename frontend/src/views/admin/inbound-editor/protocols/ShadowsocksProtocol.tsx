import { Form, Input, Select, Space } from 'antd'
import { ProtocolClients } from '../ProtocolClients'

export function ShadowsocksProtocol() {
  return (
    <ProtocolClients
      title="Shadowsocks clients"
      fields={[
        { name: 'password', label: 'Password', placeholder: 'client password' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: 'Expiry time', numeric: true },
        { name: 'enable', label: 'Enable', switch: true, defaultValue: true },
      ]}
    >
      <Space align="start" wrap>
        <Form.Item name="ssMethod" label="Method" rules={[{ required: true }]}>
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
        <Form.Item name="ssNetwork" label="Network">
          <Select
            style={{ width: 140 }}
            options={[
              { label: 'tcp+udp', value: 'tcp,udp' },
              { label: 'tcp only', value: 'tcp' },
              { label: 'udp only', value: 'udp' },
            ]}
          />
        </Form.Item>
        <Form.Item name="ssPassword" label="Global password">
          <Input placeholder="optional global password" />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
