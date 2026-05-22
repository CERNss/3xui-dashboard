import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

const apiStubs = vi.hoisted(() => ({
  nodesList: vi.fn(),
  inboundsFleet: vi.fn(),
}))
vi.mock('@/api/admin/nodes', () => ({
  nodesApi: { list: apiStubs.nodesList },
}))
vi.mock('@/api/admin/inbounds', () => ({
  inboundsApi: { fleet: apiStubs.inboundsFleet },
}))

import Status from './Status.vue'

async function mountStatus() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/status', component: { template: '<div/>' } }],
  })
  await router.push('/admin/status')
  await router.isReady()
  const w = mount(Status, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.nodesList.mockResolvedValue([
    { id: 1, name: 'tokyo-1', host: 'tk.example.com', port: 54321, enabled: true, status: 'online', cpu_pct: 12, mem_pct: 35, uptime_secs: 3600, latency_ms: 42, xray_version: '25.0.0', created_at: '', updated_at: '' },
  ])
  apiStubs.inboundsFleet.mockResolvedValue({
    inbounds: [
      { node_id: 1, node_name: 'tokyo-1', inbound: { id: 1, tag: 'vless-1', protocol: 'vless', port: 443, enable: true } },
    ],
  })
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Status.vue smoke', () => {
  it('mounts without throwing', async () => {
    const w = await mountStatus()
    expect(w.exists()).toBe(true)
  })

  it('fetches nodes + fleet on mount', async () => {
    await mountStatus()
    expect(apiStubs.nodesList).toHaveBeenCalledTimes(1)
    expect(apiStubs.inboundsFleet).toHaveBeenCalledTimes(1)
  })

  it('renders the node name from the API response', async () => {
    const w = await mountStatus()
    expect(w.text()).toContain('tokyo-1')
  })
})
