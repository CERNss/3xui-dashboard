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
}))
vi.mock('@/api/admin/nodes', () => ({
  nodesApi: apiStubs,
}))

import Nodes from './Nodes.vue'

function makeNode(over: Partial<Node> = {}): Node {
  return {
    id: 1,
    name: 'tokyo-1',
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
  apiStubs.list.mockResolvedValue([
    makeNode(),
    makeNode({ id: 2, name: 'sg-1', status: 'offline', host: 'node2.example.com' }),
  ])
})
afterEach(() => {
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

  it('opens an edit modal with the selected node and saves through update', async () => {
    apiStubs.update.mockResolvedValue(makeNode({ name: 'tokyo-edit' }))
    const w = await mountNodes()
    const editBtn = w.findAll('button[title="编辑"]')[0]
    expect(editBtn).toBeDefined()

    await editBtn.trigger('click')
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
      host: 'node1.example.com',
      api_token: '',
    }))
    expect(apiStubs.create).not.toHaveBeenCalled()
  })
})
