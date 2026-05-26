import { useTranslation } from 'react-i18next'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function SecurityAuthSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return <SettingsSection {...props} title={t('admin.settings.securityAuthTitle')} description={t('admin.settings.securityAuthDesc')} />
}
