import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function UserDefaultsSettings(props: SettingsSectionProps) {
  return <SettingsSection {...props} title="User defaults" description="Initial balance and starter plan defaults for new users." />
}
