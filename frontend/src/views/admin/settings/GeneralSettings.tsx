import { useTranslation } from 'react-i18next'
import { BrandingUpload } from './BrandingUpload'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function GeneralSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return (
    <SettingsSection
      {...props}
      title={t('admin.settings.siteSettingsTitle')}
      description={t('admin.settings.generalDesc')}
      extra={<BrandingUpload {...props} />}
    />
  )
}
