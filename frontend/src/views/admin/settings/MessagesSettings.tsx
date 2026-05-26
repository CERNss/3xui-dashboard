import { SendOutlined } from '@ant-design/icons'
import { Button, Card, Input, Space, Typography, message } from 'antd'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSmtpTest } from '@/hooks/queries/admin/settings'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function MessagesSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  const [to, setTo] = useState('')
  const smtpTest = useSmtpTest()

  const sendTest = async () => {
    await smtpTest.mutateAsync(to)
    message.success(t('admin.settings.smtpSendOk', { to }))
  }

  return (
    <SettingsSection
      {...props}
      title={t('admin.settings.messages.title')}
      description={t('admin.settings.messages.desc')}
      extra={
        <Card title={t('admin.settings.smtpTestTitle')}>
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text type="secondary">{t('admin.settings.smtpHint')}</Typography.Text>
            <Space.Compact style={{ width: '100%' }}>
              <Input aria-label={t('admin.settings.smtpRecipient')} type="email" value={to} onChange={(event) => setTo(event.target.value)} />
              <Button type="primary" icon={<SendOutlined />} disabled={!to} loading={smtpTest.isPending} onClick={sendTest}>
                {t('admin.settings.smtpSendBtn')}
              </Button>
            </Space.Compact>
          </Space>
        </Card>
      }
    />
  )
}
