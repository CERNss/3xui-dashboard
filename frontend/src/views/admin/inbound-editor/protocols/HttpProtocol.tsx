import { Form, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

export function HttpProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.httpTitle')}
      addLabel={t('admin.inboundEditor.clients.addAccount')}
      fields={[
        { name: 'user', label: t('admin.inboundEditor.clients.user'), placeholder: 'alice' },
        { name: 'pass', label: t('admin.inboundEditor.clients.pass'), placeholder: '••••••' },
      ]}
    >
      <Space align="start" wrap>
        <Form.Item name="httpAllowTransparent" label={t('admin.inboundEditor.allowTransparent')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Space>
    </ProtocolClients>
  )
}
