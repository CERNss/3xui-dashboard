import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { Webhook } from '@/api/admin/webhooks'

const apiStubs = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  test: vi.fn(),
  deliveries: vi.fn(),
  replay: vi.fn(),
}))
vi.mock('@/api/admin/webhooks', () => ({
  adminWebhooksApi: {
    list: apiStubs.list,
    create: apiStubs.create,
    update: apiStubs.update,
    remove: apiStubs.remove,
    test: apiStubs.test,
    deliveries: apiStubs.deliveries,
    replay: apiStubs.replay,
  },
}))

import Webhooks from './Webhooks.vue'

function makeWebhook(over: Partial<Webhook> = {}): Webhook {
  return {
    id: 1,
    name: 'ops-slack',
    url: 'https://hooks.slack.com/services/T/B/X',
    events: ['order.*', 'node.offline'],
    enabled: true,
    allow_private: false,
    method: 'POST',
    headers: {},
    body_template: '',
    template_format: 'json',
    created_at: '2026-05-21T10:00:00Z',
    updated_at: '2026-05-21T10:00:00Z',
    ...over,
  }
}

async function mountWebhooks() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/webhooks', component: { template: '<div/>' } }],
  })
  await router.push('/admin/webhooks')
  await router.isReady()
  const w = mount(Webhooks, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.deliveries.mockResolvedValue([])
  apiStubs.list.mockResolvedValue([
    makeWebhook(),
    makeWebhook({
      id: 2,
      name: 'analytics-get',
      url: 'https://analytics.example.com/track',
      method: 'GET',
      events: ['*'],
      enabled: false,
      body_template: 'event={{.Event}}',
      template_format: 'text',
    }),
  ])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Webhooks.vue smoke', () => {
  it('mounts and renders the page header', async () => {
    const w = await mountWebhooks()
    expect(w.text()).toContain('Webhooks')
  })

  it('renders each webhook row with method chip and template info', async () => {
    const w = await mountWebhooks()
    // Both names visible
    expect(w.text()).toContain('ops-slack')
    expect(w.text()).toContain('analytics-get')
    // Method labels rendered
    expect(w.text()).toContain('POST')
    expect(w.text()).toContain('GET')
    // Template summary differentiates default vs custom
    expect(w.text()).toMatch(/default \(json\)/)
    expect(w.text()).toMatch(/text •/)
  })

  it('shows enabled / disabled chip per row', async () => {
    const w = await mountWebhooks()
    expect(w.text()).toContain('启用')
    expect(w.text()).toContain('禁用')
  })

  it('opens the editor modal when "新建 webhook" is clicked', async () => {
    const w = await mountWebhooks()
    const newBtn = w.findAll('button').find((b) => b.text().includes('新建 webhook'))
    expect(newBtn).toBeDefined()
    await newBtn!.trigger('click')
    await flushPromises()
    // Modal-specific heading appears (Chinese title "新建 webhook"
    // also appears as the open trigger, so check for an
    // editor-only label).
    expect(w.text()).toMatch(/请求内容/)
  })

  it('triggers the test API when "test" is clicked', async () => {
    apiStubs.test.mockResolvedValue({
      id: 99,
      webhook_id: 1,
      event_type: 'webhook.test',
      status: 'pending',
      attempt: 0,
      scheduled_at: '2026-05-21T10:00:00Z',
      next_attempt_at: '2026-05-21T10:00:00Z',
    })
    const w = await mountWebhooks()
    const testBtn = w.findAll('button').find((b) => b.text().trim() === 'test')
    expect(testBtn).toBeDefined()
    await testBtn!.trigger('click')
    await flushPromises()
    expect(apiStubs.test).toHaveBeenCalledTimes(1)
    expect(apiStubs.test).toHaveBeenCalledWith(1)
  })

  it('hits the list endpoint once on mount', async () => {
    await mountWebhooks()
    expect(apiStubs.list).toHaveBeenCalledTimes(1)
  })
})
