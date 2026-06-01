import { describe, expect, it } from 'vitest'
import type { SettingItem } from '@/api/admin/settings'
import { dashboardAutoRefreshIntervalMs } from './settings'

function item(key: string, value: string): SettingItem {
  return {
    key,
    label: key,
    type: 'int',
    group: 'data_collection',
    default: '0',
    description: '',
    value,
    has_override: false,
    env_fallback: '',
  }
}

describe('dashboardAutoRefreshIntervalMs', () => {
  it('treats missing, disabled, and invalid values as off', () => {
    expect(dashboardAutoRefreshIntervalMs()).toBe(false)
    expect(dashboardAutoRefreshIntervalMs([item('dashboard_auto_refresh_interval_seconds', '0')])).toBe(false)
    expect(dashboardAutoRefreshIntervalMs([item('dashboard_auto_refresh_interval_seconds', '4')])).toBe(false)
    expect(dashboardAutoRefreshIntervalMs([item('dashboard_auto_refresh_interval_seconds', 'nope')])).toBe(false)
  })

  it('converts enabled seconds to milliseconds and clamps large values', () => {
    expect(dashboardAutoRefreshIntervalMs([item('dashboard_auto_refresh_interval_seconds', '5')])).toBe(5000)
    expect(dashboardAutoRefreshIntervalMs([item('dashboard_auto_refresh_interval_seconds', '3601')])).toBe(3_600_000)
  })
})
