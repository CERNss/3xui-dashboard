import { SettingsSection } from './SettingsSection'
import type { SettingsSectionProps } from './types'

export function SubscriptionSettings(props: SettingsSectionProps) {
  return <SettingsSection {...props} title="Subscription" description="Subscription URL and client template settings." />
}
