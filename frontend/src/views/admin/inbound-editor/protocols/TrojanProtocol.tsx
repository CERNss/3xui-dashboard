import { SettingOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Space, Typography } from 'antd'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { FallbacksConfigModal } from '../FallbacksConfigModal'
import { ProtocolClients } from '../ProtocolClients'

export function TrojanProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const fallbacks = Form.useWatch<Array<unknown>>('fallbacks', form) ?? []
  const [fallbacksOpen, setFallbacksOpen] = useState(false)
  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.trojanTitle')}
      fields={[
        { name: 'password', label: 'Password', placeholder: t('admin.inboundEditor.clients.passwordPlaceholder') },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Alert type="info" showIcon message={t('admin.inboundEditor.trojanClientInfo')} />
      <Space size={12} align="center">
        <Button icon={<SettingOutlined />} onClick={() => setFallbacksOpen(true)}>
          {t('admin.inboundEditor.fallbacks.configure')}
        </Button>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {fallbacks.length === 0
            ? t('admin.inboundEditor.fallbacks.summaryEmpty')
            : t('admin.inboundEditor.fallbacks.summaryCount', { n: fallbacks.length })}
        </Typography.Text>
      </Space>
      <FallbacksConfigModal open={fallbacksOpen} onClose={() => setFallbacksOpen(false)} />
    </ProtocolClients>
  )
}
