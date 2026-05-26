import { SendOutlined } from '@ant-design/icons'
import { Button, Card, Input, Space, Typography, message } from 'antd'
import { useState } from 'react'
import { useSmtpTest } from '@/hooks/queries/admin/settings'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function MessagesSettings(props: SettingsSectionProps) {
  const [to, setTo] = useState('')
  const smtpTest = useSmtpTest()

  const sendTest = async () => {
    await smtpTest.mutateAsync(to)
    message.success(`SMTP test sent to ${to}`)
  }

  return (
    <SettingsSection
      {...props}
      title="Messages"
      description="SMTP and message-template settings."
      extra={
        <Card title="SMTP test">
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text type="secondary">Send a one-shot test email without creating a setting row.</Typography.Text>
            <Space.Compact style={{ width: '100%' }}>
              <Input aria-label="SMTP test recipient" type="email" value={to} onChange={(event) => setTo(event.target.value)} />
              <Button type="primary" icon={<SendOutlined />} disabled={!to} loading={smtpTest.isPending} onClick={sendTest}>
                Send test
              </Button>
            </Space.Compact>
          </Space>
        </Card>
      }
    />
  )
}
