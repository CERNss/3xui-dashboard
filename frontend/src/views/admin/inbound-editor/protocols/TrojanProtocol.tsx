import { Alert } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function TrojanProtocol() {
  const { t } = useTranslation()
  return (
    <ProtocolClients
      title={t('admin.inboundEditor.clients.trojanTitle')}
      fields={[
        { name: 'password', label: 'Password', placeholder: t('admin.inboundEditor.clients.passwordPlaceholder') },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Alert type="info" showIcon message={t('admin.inboundEditor.trojanClientInfo')} />
    </ProtocolClients>
  )
}
