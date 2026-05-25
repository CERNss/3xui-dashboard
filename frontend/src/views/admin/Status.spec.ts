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
    routes: [
      { path: '/admin/status', component: { template: '<div/>' } },
      { path: '/admin/nodes', component: { template: '<div/>' } },
    ],
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

  it('keeps aggregate traffic out of the realtime status tab', async () => {
    apiStubs.inboundsFleet.mockResolvedValue({
      inbounds: [
        {
          node_id: 1,
          node_name: 'tokyo-1',
          inbound: {
            id: 1,
            tag: 'vless-1',
            protocol: 'vless',
            port: 443,
            enable: true,
            up: 1024 ** 3,
            down: 2 * 1024 ** 3,
            allTime: 8 * 1024 ** 3,
          },
        },
      ],
    })
    const w = await mountStatus()
    expect(w.text()).toContain('需关注')
    expect(w.text()).not.toContain('总流量')
    expect(w.text()).not.toContain('所有时间总使用量')
    expect(w.text()).not.toContain('3.00 GiB')
  })

  it('distinguishes failed probes from disabled nodes in the table', async () => {
    apiStubs.nodesList.mockResolvedValue([
      {
        id: 1,
        name: 'offline-with-history',
        host: 'old.example.com',
        port: 443,
        enabled: true,
        status: 'offline',
        cpu_pct: 33.7,
        mem_pct: 54.2,
        xray_version: '25.1.0',
        last_seen_at: '2026-05-24T09:38:02Z',
        created_at: '',
        updated_at: '',
      },
      {
        id: 2,
        name: 'disabled-without-data',
        host: 'disabled.example.com',
        port: 443,
        enabled: false,
        status: 'offline',
        cpu_pct: 0,
        mem_pct: 0,
        xray_version: '',
        last_seen_at: null,
        created_at: '',
        updated_at: '',
      },
    ])
    const w = await mountStatus()
    expect(w.text()).toContain('探测失败，上次在线')
    expect(w.text()).toContain('离线前最后一次指标')
    expect(w.text()).toContain('33.7% · 54.2%')
    expect(w.text()).toContain('已停用')
    expect(w.text()).toContain('停用节点不采集')
    expect(w.text()).toContain('离线 1 · 未探测 0 · 停用 1')
  })
})
