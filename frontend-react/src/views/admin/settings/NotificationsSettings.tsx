import { Card, Typography } from 'antd'
import Webhooks from '../Webhooks'

export function NotificationsSettings() {
  return (
    <Card>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        Notifications
      </Typography.Title>
      <Typography.Text type="secondary">Manage outbound webhook notifications for admin events.</Typography.Text>
      <div style={{ marginTop: 16 }}>
        <Webhooks embedded />
      </div>
    </Card>
  )
}
