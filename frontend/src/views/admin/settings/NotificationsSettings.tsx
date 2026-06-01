import { Card } from 'antd'
import { useTranslation } from 'react-i18next'
import Webhooks from '../Webhooks'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function NotificationsSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return (
    <SettingsSection
      {...props}
      title={t('admin.settings.notifications.title')}
      description={t('admin.settings.notifications.desc')}
      extraPosition="bottom"
      extra={
        <Card title={t('admin.settings.notifications.webhooksTitle')}>
          <Webhooks embedded />
        </Card>
      }
    />
  )
}
