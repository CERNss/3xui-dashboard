import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { AdminPlan } from '@/api/admin/plans'

const apiStubs = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}))
vi.mock('@/api/admin/plans', () => ({
  adminPlansApi: apiStubs,
}))

import Plans from './Plans.vue'

function makePlan(over: Partial<AdminPlan> = {}): AdminPlan {
  return {
    id: 1,
    name: 'Pro 30d',
    description: '',
    duration_days: 30,
    traffic_limit_bytes: 100 * 1024 * 1024 * 1024,
    price_cents: 500,
    ip_limit: 0,
    enabled: true,
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...over,
  } as AdminPlan
}

async function mountPlans() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/plans', component: { template: '<div/>' } }],
  })
  await router.push('/admin/plans')
  await router.isReady()
  const w = mount(Plans, {
    global: { plugins: [router], mocks: { $t: (k: string) => k } },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.list.mockResolvedValue([
    makePlan(),
    makePlan({ id: 2, name: 'Basic 7d', price_cents: 100, duration_days: 7, enabled: false }),
  ])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Plans.vue smoke', () => {
  it('mounts and renders the page title', async () => {
    const w = await mountPlans()
    expect(w.text()).toContain('套餐管理')
  })

  it('renders plan rows from the API with formatted prices + traffic', async () => {
    const w = await mountPlans()
    expect(w.text()).toContain('Pro 30d')
    expect(w.text()).toContain('Basic 7d')
    expect(w.text()).toContain('¥5.00')   // 500 cents
    expect(w.text()).toContain('¥1.00')   // 100 cents
    expect(w.text()).toContain('100 GB')  // traffic_limit_bytes formatted
  })

  it('opens the create modal when "新建套餐" is clicked', async () => {
    const w = await mountPlans()
    const newBtn = w.findAll('button').find((b) => b.text().includes('新建套餐'))
    expect(newBtn).toBeDefined()
    await newBtn!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('新建套餐')
    // Form fields visible
    expect(w.html()).toContain('placeholder="例如：基础 30 天"')
  })
})
