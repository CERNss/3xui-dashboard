import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

const apiStubs = vi.hoisted(() => ({
  poolsList: vi.fn(),
  poolsCreate: vi.fn(),
  addTarget: vi.fn(),
  updateTarget: vi.fn(),
  removeTarget: vi.fn(),
  removePool: vi.fn(),
  fleet: vi.fn(),
}))

vi.mock('@/api/admin/provisioningPools', () => ({
  provisioningPoolsApi: {
    list: apiStubs.poolsList,
    create: apiStubs.poolsCreate,
    addTarget: apiStubs.addTarget,
    updateTarget: apiStubs.updateTarget,
    removeTarget: apiStubs.removeTarget,
    remove: apiStubs.removePool,
  },
}))

vi.mock('@/api/admin/inbounds', () => ({
  inboundsApi: { fleet: apiStubs.fleet },
}))

import ProvisioningPools from './ProvisioningPools.vue'

async function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/provisioning-pools', component: { template: '<div/>' } }],
  })
  await router.push('/admin/provisioning-pools')
  await router.isReady()
  const w = mount(ProvisioningPools, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.poolsList.mockResolvedValue([
    {
      id: 1,
      name: 'basic-pool',
      description: 'Basic users',
      enabled: true,
      auto_create: false,
      allowed_protocols: ['vless'],
      targets: [
        {
          id: 10,
          pool_id: 1,
          node_id: 1,
          node_name: 'node-a',
          inbound_tag: 'vless-1',
          protocol: 'vless',
          max_clients: 100,
          used_clients: 2,
          priority: 100,
          enabled: true,
        },
      ],
    },
  ])
  apiStubs.fleet.mockResolvedValue({
    inbounds: [
      {
        node_id: 1,
        node_name: 'node-a',
        inbound: { tag: 'vless-1', remark: 'main', enable: true, port: 443, protocol: 'vless' },
      },
    ],
  })
})

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/ProvisioningPools.vue smoke', () => {
  it('renders pools and target capacity', async () => {
    const w = await mountPage()
    expect(w.text()).toContain('分配池')
    expect(w.text()).toContain('basic-pool')
    expect(w.text()).toContain('node-a')
    expect(w.text()).toContain('2 / 100')
  })

  it('opens the create modal', async () => {
    const w = await mountPage()
    const btn = w.findAll('button').find(b => b.text().includes('新建分配池'))
    expect(btn).toBeDefined()
    await btn!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('新建分配池')
    expect(w.html()).toContain('basic-vless')
  })
})
