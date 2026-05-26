import { useTranslation } from 'react-i18next'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function AlertsSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return <SettingsSection {...props} title={t('admin.settings.alertsTitle')} description={t('admin.settings.alertsDesc')} />
}
