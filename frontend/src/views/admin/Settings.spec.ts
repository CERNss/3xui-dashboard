import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const apiStubs = vi.hoisted(() => ({
  list: vi.fn(),
  set: vi.fn(),
  clear: vi.fn(),
}))
vi.mock('@/api/admin/settings', () => ({
  settingsApi: {
    list: apiStubs.list,
    set: apiStubs.set,
    clear: apiStubs.clear,
  },
}))

import Settings from './Settings.vue'

beforeEach(() => {
  apiStubs.list.mockResolvedValue([
    { key: 'public_registration', label: '允许注册', group: 'registration', value: 'true', has_override: true },
    { key: 'expiry_warn_days', label: '到期提醒天数', group: 'other', value: '3', has_override: false },
  ])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

async function mountSettings() {
  const w = mount(Settings, {
    global: { mocks: { $t: (k: string) => k } },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

describe('admin/Settings.vue smoke', () => {
  it('mounts and fetches settings once', async () => {
    const w = await mountSettings()
    expect(w.exists()).toBe(true)
    expect(apiStubs.list).toHaveBeenCalledTimes(1)
  })

  it('renders each setting key from the API', async () => {
    const w = await mountSettings()
    expect(w.text()).toContain('允许注册')
    expect(w.text()).toContain('到期提醒天数')
  })
})
