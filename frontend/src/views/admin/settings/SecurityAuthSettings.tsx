import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function SecurityAuthSettings(props: SettingsSectionProps) {
  return <SettingsSection {...props} title="Security & auth" description="Registration and OIDC settings." />
}
