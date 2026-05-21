import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { Node } from '@/api/admin/nodes'

const apiStubs = vi.hoisted(() => ({
  create: vi.fn(),
  update: vi.fn(),
}))
vi.mock('@/api/admin/inbounds', () => ({
  inboundsApi: {
    create: apiStubs.create,
    update: apiStubs.update,
  },
}))

import InboundEditorModal from './InboundEditorModal.vue'

function fakeNode(id: number, name = `node-${id}`): Node {
  return {
    id,
    name,
    scheme: 'https',
    host: '127.0.0.1',
    port: 54321,
    base_path: '/',
    enabled: true,
    status: 'online',
    last_seen_at: '2026-05-21T00:00:00Z',
    cpu_pct: 0,
    mem_pct: 0,
    uptime_secs: 0,
    latency_ms: 0,
    xray_version: '25.0.0',
    created_at: '',
    updated_at: '',
  } as unknown as Node
}

function mountEditor(over: { mode?: 'create' | 'edit'; open?: boolean } = {}) {
  return mount(InboundEditorModal, {
    props: {
      open: over.open ?? true,
      mode: over.mode ?? 'create',
      nodeID: 1,
      tag: '',
      source: null,
      nodes: [fakeNode(1)],
    },
    global: { mocks: { $t: (k: string) => k } },
    attachTo: document.body,
  })
}

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/InboundEditorModal smoke', () => {
  it('renders nothing when open is false', () => {
    const w = mountEditor({ open: false })
    // Modal body markers absent — modal is closed.
    expect(w.text()).not.toContain('基础配置')
  })

  it('renders the tab nav with the 5 tabs when open', async () => {
    const w = mountEditor()
    await flushPromises()
    const text = w.text()
    expect(text).toContain('基础配置')
    expect(text).toContain('协议')
    expect(text).toContain('Stream')
    expect(text).toContain('Sniffing')
    expect(text).toContain('高级配置')
  })

  it('exposes all 6 protocol options including wireguard + hysteria', async () => {
    const w = mountEditor()
    await flushPromises()
    // Multiple selects on the Basic tab (node + protocol + traffic
    // reset). Find the one whose options include 'vless' rather
    // than relying on document order.
    const protocolSelect = w.findAll('select').find((s) =>
      s.findAll('option').some((o) => o.attributes('value') === 'vless'),
    )
    expect(protocolSelect).toBeDefined()
    const options = protocolSelect!.findAll('option').map((o) => o.attributes('value'))
    expect(options).toContain('vless')
    expect(options).toContain('vmess')
    expect(options).toContain('trojan')
    expect(options).toContain('shadowsocks')
    expect(options).toContain('wireguard')
    expect(options).toContain('hysteria')
  })

  it('hides Stream + Sniffing tabs for wireguard protocol', async () => {
    const w = mountEditor()
    await flushPromises()
    // Switch protocol via the basic-tab select
    const selects = w.findAll('select')
    const protoSelect = selects.find((s) =>
      s.findAll('option').some((o) => o.attributes('value') === 'vless'),
    )
    expect(protoSelect).toBeDefined()
    await protoSelect!.setValue('wireguard')
    await flushPromises()
    // Tab labels: the WG branch should NOT render Stream / Sniffing in the visible nav.
    // findAll('button') includes both tab buttons and form buttons; filter to tab nav.
    const tabButtons = w.findAll('nav button').map((b) => b.text().trim())
    expect(tabButtons).toContain('基础配置')
    expect(tabButtons).toContain('协议')
    expect(tabButtons).not.toContain('Stream')
    expect(tabButtons).not.toContain('Sniffing')
  })

  it('hides Stream + Sniffing tabs for hysteria protocol', async () => {
    const w = mountEditor()
    await flushPromises()
    const selects = w.findAll('select')
    const protoSelect = selects.find((s) => s.element.value === 'vless')
    await protoSelect!.setValue('hysteria')
    await flushPromises()
    const tabButtons = w.findAll('nav button').map((b) => b.text().trim())
    expect(tabButtons).not.toContain('Stream')
    expect(tabButtons).not.toContain('Sniffing')
  })

  it('keeps Stream + Sniffing visible for vless (the default case)', async () => {
    const w = mountEditor()
    await flushPromises()
    const tabButtons = w.findAll('nav button').map((b) => b.text().trim())
    expect(tabButtons).toContain('Stream')
    expect(tabButtons).toContain('Sniffing')
  })

  it('emits close when the X button is clicked', async () => {
    const w = mountEditor()
    await flushPromises()
    // Header X button — first svg button in the modal header.
    const closeBtn = w.find('header button')
    expect(closeBtn.exists()).toBe(true)
    await closeBtn.trigger('click')
    expect(w.emitted('close')).toBeTruthy()
  })
})
