import { Alert } from 'antd'
import { ProtocolClients } from '../ProtocolClients'

export function TrojanProtocol() {
  return (
    <ProtocolClients
      title="Trojan passwords"
      fields={[
        { name: 'password', label: 'Password', placeholder: 'client password' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: 'Expiry time', numeric: true },
        { name: 'enable', label: 'Enable', switch: true, defaultValue: true },
      ]}
    >
      <Alert type="info" showIcon message="Trojan clients authenticate with per-client passwords." />
    </ProtocolClients>
  )
}
