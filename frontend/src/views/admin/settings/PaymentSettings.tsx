import { useTranslation } from 'react-i18next'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function PaymentSettings(props: SettingsSectionProps) {
  const { t } = useTranslation()
  return (
    <SettingsSection
      {...props}
      title={t('admin.settings.payment.title')}
      description={t('admin.settings.payment.desc')}
    />
  )
}
