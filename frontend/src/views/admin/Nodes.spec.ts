import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { Node } from '@/api/admin/nodes'

const apiStubs = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  enable: vi.fn(),
  disable: vi.fn(),
  remove: vi.fn(),
  probe: vi.fn(),
  fleet: vi.fn(),
}))
vi.mock('@/api/admin/nodes', () => ({
  nodesApi: apiStubs,
}))
vi.mock('@/api/admin/inbounds', () => ({
  inboundsApi: {
    fleet: apiStubs.fleet,
  },
}))

import Nodes from './Nodes.vue'

function makeNode(over: Partial<Node> = {}): Node {
  return {
    id: 1,
    name: 'tokyo-1',
    area: 'jp',
    province: 'Tokyo',
    scheme: 'https',
    host: 'node1.example.com',
    port: 2053,
    base_path: '',
    enabled: true,
    status: 'online',
    cpu_pct: 12.5,
    mem_pct: 34.5,
    xray_version: '1.8.13',
    uptime_s: 86400,
    last_seen_at: '2026-05-20T10:00:00Z',
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-20T10:00:00Z',
    ...over,
  } as Node
}

async function mountNodes() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/nodes', component: { template: '<div/>' } }],
  })
  await router.push('/admin/nodes')
  await router.isReady()
  const w = mount(Nodes, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  vi.useFakeTimers()
  apiStubs.list.mockResolvedValue([
    makeNode(),
    makeNode({ id: 2, name: 'sg-1', area: 'sg', province: 'unknown', status: 'offline', host: 'node2.example.com' }),
  ])
  apiStubs.fleet.mockResolvedValue({
    inbounds: [
      { node_id: 1, node_name: 'tokyo-1', inbound: { clientStats: [{ email: 'a@example.com' }] } },
      { node_id: 2, node_name: 'sg-1', inbound: { clientStats: [] } },
    ],
  })
})
afterEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Nodes.vue smoke', () => {
  it('mounts and renders node actions', async () => {
    const w = await mountNodes()
    expect(w.text()).toContain('添加节点')
    expect(w.find('button[title="编辑"]').exists()).toBe(true)
  })

  it('renders nodes from the API with connection strings', async () => {
    const w = await mountNodes()
    expect(w.text()).toContain('tokyo-1')
    expect(w.text()).toContain('sg-1')
    expect(w.text()).toContain('node1.example.com')
    expect(w.text()).toContain('node2.example.com')
  })

  it('sends backend search filters', async () => {
    const w = await mountNodes()
    apiStubs.list.mockClear()
    apiStubs.list.mockResolvedValueOnce([
      makeNode({ id: 2, name: 'sg-1', area: 'sg', province: 'unknown', status: 'offline', host: 'node2.example.com' }),
    ])

    await w.find('input[placeholder="搜索代理..."]').setValue('node2')
    await vi.advanceTimersByTimeAsync(260)
    await flushPromises()

    expect(apiStubs.list).toHaveBeenLastCalledWith({ query: 'node2', area: undefined, province: undefined, scheme: undefined, status: undefined })
    expect(w.text()).toContain('sg-1')
    expect(w.text()).toContain('显示 1 个，共 1 个节点')
    expect(w.findAll('tbody tr')).toHaveLength(1)
    expect(w.find('tbody tr').text()).not.toContain('tokyo-1')
  })

  it('sends backend area and province filters', async () => {
    const w = await mountNodes()
    apiStubs.list.mockClear()
    apiStubs.list.mockResolvedValueOnce([
      makeNode({ id: 2, name: 'sg-1', area: 'sg', province: 'unknown', status: 'offline', host: 'node2.example.com' }),
    ])

    await w.find('select[aria-label="按区域筛选"]').setValue('sg')
    await vi.advanceTimersByTimeAsync(260)
    await flushPromises()

    expect(apiStubs.list).toHaveBeenLastCalledWith({ query: undefined, area: 'sg', province: undefined, scheme: undefined, status: undefined })
    expect(w.text()).toContain('新加坡 · 未知')
    expect(w.text()).toContain('显示 1 个，共 1 个节点')
    expect(w.findAll('tbody tr')).toHaveLength(1)
    expect(w.find('tbody tr').text()).toContain('sg-1')
    expect(w.find('tbody tr').text()).not.toContain('tokyo-1')

    apiStubs.list.mockResolvedValueOnce([
      makeNode({ id: 1, name: 'tokyo-1', area: 'jp', province: 'Tokyo' }),
    ])
    await w.find('input[aria-label="按地区筛选"]').setValue('Tokyo')
    await vi.advanceTimersByTimeAsync(260)
    await flushPromises()

    expect(apiStubs.list).toHaveBeenLastCalledWith({ query: undefined, area: 'sg', province: 'Tokyo', scheme: undefined, status: undefined })
  })

  it('sends backend protocol and status filters', async () => {
    const w = await mountNodes()
    apiStubs.list.mockClear()
    apiStubs.list.mockResolvedValueOnce([
      makeNode({ id: 2, name: 'sg-1', area: 'sg', province: 'unknown', scheme: 'http', status: 'offline', host: 'node2.example.com' }),
    ])

    await w.find('select[aria-label="按协议筛选"]').setValue('http')
    await vi.advanceTimersByTimeAsync(260)
    await flushPromises()
    expect(apiStubs.list).toHaveBeenLastCalledWith({ query: undefined, area: undefined, province: undefined, scheme: 'http', status: undefined })

    apiStubs.list.mockResolvedValueOnce([
      makeNode({ id: 2, name: 'sg-1', area: 'sg', province: 'unknown', scheme: 'http', status: 'offline', host: 'node2.example.com' }),
    ])
    await w.find('select[aria-label="按状态筛选"]').setValue('offline')
    await vi.advanceTimersByTimeAsync(260)
    await flushPromises()

    expect(apiStubs.list).toHaveBeenLastCalledWith({ query: undefined, area: undefined, province: undefined, scheme: 'http', status: 'offline' })
  })

  it('shows xray version', async () => {
    const w = await mountNodes()
    expect(w.text()).toContain('1.8.13')
  })

  it('shows disabled nodes as disabled instead of offline', async () => {
    apiStubs.list.mockResolvedValue([
      makeNode({ id: 1, name: 'failed-probe', enabled: true, status: 'offline' }),
      makeNode({ id: 2, name: 'manual-disabled', enabled: false, status: 'offline' }),
    ])
    const w = await mountNodes()
    const rows = w.findAll('tbody tr')

    expect(rows[0].text()).toContain('离线')
    expect(rows[0].text()).not.toContain('已停用')
    expect(rows[1].text()).toContain('已停用')
    expect(rows[1].text()).not.toContain('离线')
  })

  it('opens the add-node modal when "添加节点" clicked', async () => {
    const w = await mountNodes()
    const addBtn = w.findAll('button').find((b) => b.text().includes('添加节点'))
    expect(addBtn).toBeDefined()
    await addBtn!.trigger('click')
    await flushPromises()
    // Modal header
    expect(w.text()).toContain('填好面板地址和这个面板的 API 密钥后，控制台会自动探测')
  })

  it('fills connection fields from a pasted panel URL', async () => {
    const w = await mountNodes()
    const addBtn = w.findAll('button').find((b) => b.text().includes('添加节点'))
    expect(addBtn).toBeDefined()
    await addBtn!.trigger('click')
    await flushPromises()

    await w.find('input[placeholder="https://tokyo-edge.example.net:2053/panel/"]').setValue('https://tokyo-edge.example.net:2053/panel/')

    const formSelects = w.find('form').findAll('select')
    expect((formSelects[0].element as HTMLSelectElement).value).toBe('unknown')
    expect((formSelects[1].element as HTMLSelectElement).value).toBe('https')
    expect(w.find('input[placeholder="node1.example.com"]').element).toHaveProperty('value', 'tokyo-edge.example.net')
    expect(w.find('input[type="number"]').element).toHaveProperty('value', '2053')
    expect(w.find('input[placeholder="/panel/ 或空"]').element).toHaveProperty('value', '/panel/')
  })

  it('opens an edit modal with the selected node and saves through update', async () => {
    apiStubs.update.mockResolvedValue(makeNode({ name: 'tokyo-edit' }))
    const w = await mountNodes()
    const editBtn = w.findAll('button[title="编辑"]').find((button) =>
      button.element.closest('article, tr')?.textContent?.includes('tokyo-1'),
    )
    expect(editBtn).toBeDefined()

    await editBtn!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('编辑节点 · tokyo-1')
    expect(w.find('input[placeholder="tokyo-1"]').element).toHaveProperty('value', 'tokyo-1')
    expect(w.find('input[placeholder="node1.example.com"]').element).toHaveProperty('value', 'node1.example.com')
    expect(w.find('input[placeholder="留空则保留当前面板 API 密钥"]').exists()).toBe(true)

    await w.find('input[placeholder="tokyo-1"]').setValue('tokyo-renamed')
    await w.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(apiStubs.update).toHaveBeenCalledWith(1, expect.objectContaining({
      name: 'tokyo-renamed',
      area: 'jp',
      province: 'Tokyo',
      host: 'node1.example.com',
      api_token: '',
    }))
    expect(apiStubs.create).not.toHaveBeenCalled()
  })
})
