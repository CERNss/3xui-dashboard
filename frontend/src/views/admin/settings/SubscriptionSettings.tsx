import { useTranslation } from 'react-i18next'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function SubscriptionSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return <SettingsSection {...props} title={t('admin.settings.subscriptionTitle')} description={t('admin.settings.subscriptionDesc')} />
}
