import { Form, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function VmessProtocol() {
  const { t } = useTranslation()
  return (
    <ProtocolClients
      title={t('admin.inboundEditor.clients.vmessTitle')}
      fields={[
        { name: 'id', label: 'UUID', placeholder: t('admin.inboundEditor.clients.uuidPlaceholder') },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Form.Item name="disableInsecureEncryption" label={t('admin.inboundEditor.vmessDisableInsecure')} valuePropName="checked">
        <Switch />
      </Form.Item>
    </ProtocolClients>
  )
}
