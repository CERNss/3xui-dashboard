import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const apiStubs = vi.hoisted(() => ({
  nodesList: vi.fn(),
  nodeMetrics: vi.fn(),
  inboundsFleet: vi.fn(),
}))

vi.mock('@/api/admin/nodes', () => ({
  nodesApi: {
    list: apiStubs.nodesList,
    metrics: apiStubs.nodeMetrics,
  },
}))

vi.mock('@/api/admin/inbounds', () => ({
  inboundsApi: { fleet: apiStubs.inboundsFleet },
}))

import OpsMonitor from './OpsMonitor.vue'

async function mountOpsMonitor() {
  const w = mount(OpsMonitor, { attachTo: document.body })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.nodesList.mockResolvedValue([
    {
      id: 1,
      name: 'tokyo-1',
      scheme: 'https',
      host: 'tokyo.example.com',
      port: 443,
      base_path: '',
      enabled: true,
      status: 'online',
      cpu_pct: 20,
      mem_pct: 40,
      xray_version: '25.1.0',
      uptime_s: 3600,
      last_seen_at: '2026-05-24T09:38:02Z',
      created_at: '',
      updated_at: '',
    },
    {
      id: 2,
      name: 'disabled-1',
      scheme: 'https',
      host: 'disabled.example.com',
      port: 443,
      base_path: '',
      enabled: false,
      status: 'offline',
      cpu_pct: 0,
      mem_pct: 0,
      xray_version: '',
      uptime_s: 0,
      last_seen_at: null,
      created_at: '',
      updated_at: '',
    },
  ])
  apiStubs.nodeMetrics.mockResolvedValue({
    id: 1,
    from: 0,
    to: 0,
    bucket: '10m',
    points: [
      { time: '2026-05-24T09:00:00Z', cpu: 12, mem: 35 },
      { time: '2026-05-24T09:10:00Z', cpu: 24, mem: 45 },
    ],
  })
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
          clientStats: [
            { id: 1, inboundId: 1, enable: true, email: 'a@example.com' },
            { id: 2, inboundId: 1, enable: false, email: 'b@example.com' },
          ],
        },
      },
    ],
    node_errors: { 9: 'timeout' },
  })
})

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/OpsMonitor.vue', () => {
  it('renders real node and fleet data without fake business metrics', async () => {
    const w = await mountOpsMonitor()

    expect(w.text()).toContain('需要关注')
    expect(w.text()).toContain('100')
    expect(w.text()).toContain('1/1')
    expect(w.text()).toContain('启用入站')
    expect(w.text()).toContain('1/1')
    expect(w.text()).toContain('启用客户端')
    expect(w.text()).toContain('共 2 个客户端')
    expect(w.text()).toContain('tokyo-1')
    expect(w.text()).toContain('节点 9')
    expect(w.text()).toContain('timeout')
    expect(w.text()).toContain('业务指标未接入')
    expect(w.text()).toContain('未接入')
  })

  it('loads per-node metrics for enabled nodes only', async () => {
    await mountOpsMonitor()
    expect(apiStubs.nodeMetrics).toHaveBeenCalledTimes(1)
    expect(apiStubs.nodeMetrics).toHaveBeenCalledWith(
      1,
      expect.objectContaining({ bucket: '10m' }),
    )
  })

  it('surfaces partial metric failures while keeping the page usable', async () => {
    apiStubs.nodeMetrics.mockRejectedValueOnce(new Error('metrics down'))
    const w = await mountOpsMonitor()
    expect(w.text()).toContain('1 个节点指标加载失败')
    expect(w.text()).toContain('启用入站')
  })
})
