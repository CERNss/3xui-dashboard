import { useTranslation } from 'react-i18next'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function UserDefaultsSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return <SettingsSection {...props} title={t('admin.settings.userDefaultsTitle')} description={t('admin.settings.userDefaultsDesc')} />
}
