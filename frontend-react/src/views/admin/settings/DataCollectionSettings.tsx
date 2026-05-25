import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function DataCollectionSettings(props: SettingsSectionProps) {
  return (
    <SettingsSection
      {...props}
      title="Data collection"
      description="Configure background collection jobs for node health and traffic telemetry."
    />
  )
}
