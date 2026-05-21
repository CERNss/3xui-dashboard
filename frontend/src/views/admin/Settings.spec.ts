import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

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
  // RouterLink in the 通知 tab needs a router instance to resolve.
  // A memory-history stub is enough — tests don't exercise navigation.
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/webhooks', component: { template: '<div/>' } }],
  })
  await router.push('/admin/settings')
  await router.isReady()
  const w = mount(Settings, {
    global: { plugins: [router], mocks: { $t: (k: string) => k } },
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
