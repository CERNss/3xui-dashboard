import { useTranslation } from 'react-i18next'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function DataCollectionSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return (
    <SettingsSection
      {...props}
      title={t('admin.settings.dataCollectionTitle')}
      description={t('admin.settings.dataCollectionDesc')}
    />
  )
}
