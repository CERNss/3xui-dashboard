import { Form, Select } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function VlessProtocol() {
  const { t } = useTranslation()
  return (
    <ProtocolClients
      title={t('admin.inboundEditor.clients.vlessTitle')}
      fields={[
        { name: 'id', label: 'UUID', placeholder: t('admin.inboundEditor.clients.uuidPlaceholder') },
        { name: 'flow', label: 'Flow', placeholder: 'xtls-rprx-vision' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Form.Item name="decryption" label={t('admin.inboundEditor.decryption')} rules={[{ required: true }]}>
        <Select options={[{ label: 'none', value: 'none' }]} />
      </Form.Item>
    </ProtocolClients>
  )
}
