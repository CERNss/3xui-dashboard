import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia } from 'pinia'

import AdminLayout from './AdminLayout.vue'

beforeAll(() => {
  const mem: Record<string, string> = {}
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => (key in mem ? mem[key] : null),
    setItem: (key: string, value: string) => { mem[key] = value },
    removeItem: (key: string) => { delete mem[key] },
    clear: () => { for (const key of Object.keys(mem)) delete mem[key] },
  })
})

describe('components/layout/AdminLayout.vue', () => {
  beforeEach(() => {
    localStorage.setItem('dashboard.admin.token', 'layout-test-token')
  })

  afterEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
    localStorage.clear()
  })

  it('shows the current admin page title and subtitle in the app header', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/admin',
          component: AdminLayout,
          meta: { requiresAdmin: true },
          children: [
            { path: 'status', component: { template: '<div />' } },
            { path: 'ops-monitor', component: { template: '<div />' } },
            {
              path: 'nodes',
              component: { template: '<div data-test-content>nodes body</div>' },
              meta: {
                requiresAdmin: true,
                titleKey: 'admin.nodes.title',
                subtitleKey: 'admin.nodes.subtitle',
              },
            },
            { path: 'inbounds', component: { template: '<div />' } },
            { path: 'users', component: { template: '<div />' } },
            { path: 'plans', component: { template: '<div />' } },
            { path: 'provisioning-pools', component: { template: '<div />' } },
            { path: 'orders', component: { template: '<div />' } },
            { path: 'audit-log', component: { template: '<div />' } },
            { path: 'settings', component: { template: '<div />' } },
          ],
        },
      ],
    })

    await router.push('/admin/nodes')
    await router.isReady()

    const wrapper = mount(AdminLayout, {
      global: {
        plugins: [router, createPinia()],
      },
      attachTo: document.body,
    })
    await flushPromises()

    const desktopHeader = wrapper.get('main > header')
    expect(desktopHeader.text()).toContain('节点列表')
    expect(desktopHeader.text()).toContain('管理已接入的 3x-ui 面板，自动同步状态和流量')
    expect(wrapper.text()).toContain('nodes body')
  })
})
