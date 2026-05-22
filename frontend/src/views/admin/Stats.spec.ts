import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { AdminStats } from '@/api/admin/stats'
import type { AdminPlan } from '@/api/admin/plans'

const apiStubs = vi.hoisted(() => ({
  statsGet: vi.fn(),
  plansList: vi.fn(),
}))
vi.mock('@/api/admin/stats', () => ({
  adminStatsApi: { get: apiStubs.statsGet },
}))
vi.mock('@/api/admin/plans', () => ({
  adminPlansApi: { list: apiStubs.plansList },
}))

import Stats from './Stats.vue'

function makeStats(over: Partial<AdminStats> = {}): AdminStats {
  return {
    users: { total: 42, active: 38, suspended: 4, total_balance_cents: 50000, avg_balance_cents: 1190 },
    plans: { total: 3, enabled: 2, disabled: 1 },
    orders: {
      total: 100, completed: 90, failed: 5, refunded: 5,
      revenue_cents: 45000, month_count: 12, month_revenue_cents: 6000,
    },
    recent_orders: [
      {
        id: 100, user_id: 1, user_email: 'alice@example.com',
        plan_id: 1, plan_name: 'Pro 30d', price_cents: 500,
        status: 'completed', created_at: '2026-05-20T00:00:00Z',
      },
    ],
    ...over,
  }
}

async function mountStats() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/stats', component: { template: '<div/>' } }],
  })
  await router.push('/admin/stats')
  await router.isReady()
  const w = mount(Stats, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.statsGet.mockResolvedValue(makeStats())
  apiStubs.plansList.mockResolvedValue([
    { id: 1, name: 'Pro 30d', price_cents: 500, duration_days: 30, traffic_limit_bytes: 0, enabled: true } as AdminPlan,
  ])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Stats.vue smoke', () => {
  it('mounts and renders the page title', async () => {
    const w = await mountStats()
    expect(w.text()).toContain('统计')
  })

  it('renders the KPI cards from the aggregate endpoint', async () => {
    const w = await mountStats()
    // User KPI
    expect(w.text()).toContain('42')      // total users
    expect(w.text()).toContain('38 正常')
    expect(w.text()).toContain('4 封停')
    // Month revenue (¥60.00 from 6000 cents)
    expect(w.text()).toContain('¥60.00')
    // Order counts (✓ 90)
    expect(w.text()).toContain('90')
  })

  it('renders the recent-orders activity feed', async () => {
    const w = await mountStats()
    expect(w.text()).toContain('近期订单')
    expect(w.text()).toContain('alice@example.com → Pro 30d')
  })

  it('hits the single aggregate endpoint instead of fanning out', async () => {
    await mountStats()
    expect(apiStubs.statsGet).toHaveBeenCalledTimes(1)
    expect(apiStubs.plansList).toHaveBeenCalledTimes(1)
    // No other API stubs exist — this proves we're NOT hitting
    // orders.list / users.list with limit:1000 (the pre-P1e shape).
  })
})
