import { BrandingUpload } from './BrandingUpload'
import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function GeneralSettings(props: SettingsSectionProps) {
  return (
    <SettingsSection
      {...props}
      title="Site settings"
      description="General server-driven settings and branding composites."
      extra={<BrandingUpload />}
    />
  )
}
