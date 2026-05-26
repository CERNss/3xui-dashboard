import { Card, Typography } from 'antd'
import { useTranslation } from 'react-i18next'
import Webhooks from '../Webhooks'

export function NotificationsSettings() {
  const { t } = useTranslation()
  return (
    <Card>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {t('admin.settings.notifications.title')}
      </Typography.Title>
      <Typography.Text type="secondary">{t('admin.settings.notifications.desc')}</Typography.Text>
      <div style={{ marginTop: 16 }}>
        <Webhooks embedded />
      </div>
    </Card>
  )
}
