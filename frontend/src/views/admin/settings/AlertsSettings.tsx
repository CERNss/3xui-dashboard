import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function AlertsSettings(props: SettingsSectionProps) {
  return <SettingsSection {...props} title="Alerts" description="Traffic and expiry alert thresholds." />
}
