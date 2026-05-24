import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'

import type { AdminStats } from '@/api/admin/stats'
import type { AdminPlan } from '@/api/admin/plans'

// Stub every API the embedded Status + Stats panels call, since
// Overview transitively mounts them.
const apiStubs = vi.hoisted(() => ({
  nodesList: vi.fn(),
  inboundsFleet: vi.fn(),
  statsGet: vi.fn(),
  plansList: vi.fn(),
}))
vi.mock('@/api/admin/nodes', () => ({
  nodesApi: { list: apiStubs.nodesList },
}))
vi.mock('@/api/admin/inbounds', () => ({
  inboundsApi: { fleet: apiStubs.inboundsFleet },
}))
vi.mock('@/api/admin/stats', () => ({
  adminStatsApi: { get: apiStubs.statsGet },
}))
vi.mock('@/api/admin/plans', () => ({
  adminPlansApi: { list: apiStubs.plansList },
}))

import Overview from './Overview.vue'

function makeStats(): AdminStats {
  return {
    users: {
      total: 42, active: 38, suspended: 4,
      month_new: 6, prev_month_new: 3,
      total_balance_cents: 50000, avg_balance_cents: 1190,
    },
    plans: { total: 3, enabled: 2, disabled: 1 },
    orders: {
      total: 100, completed: 90, failed: 5, refunded: 5,
      revenue_cents: 45000, month_count: 12, month_revenue_cents: 6000,
    },
    traffic: {
      month_up_bytes: 0, month_down_bytes: 0,
      today_up_bytes: 0, today_down_bytes: 0,
    },
    top_nodes: [],
    top_users: [],
    audit: { info: 0, warn: 0, err: 0 },
    recent_orders: [],
  }
}

async function mountOverview(initialPath = '/admin/status'): Promise<{ w: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/status', component: Overview },
      { path: '/admin/stats', component: Overview },
      { path: '/admin/nodes', component: { template: '<div/>' } },
      { path: '/admin/audit', component: { template: '<div/>' } },
    ],
  })
  await router.push(initialPath)
  await router.isReady()
  const w = mount(Overview, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return { w, router }
}

beforeEach(() => {
  apiStubs.nodesList.mockResolvedValue([])
  apiStubs.inboundsFleet.mockResolvedValue({ inbounds: [] })
  apiStubs.statsGet.mockResolvedValue(makeStats())
  apiStubs.plansList.mockResolvedValue([{ id: 1, name: 'P', price_cents: 0, duration_days: 30, traffic_limit_bytes: 0, enabled: true } as AdminPlan])
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Overview.vue', () => {
  it('renders the status header on /admin/status and only mounts the Status panel', async () => {
    const { w } = await mountOverview('/admin/status')
    // Header derives from active tab — title comes from i18n
    // (admin.status.title = "系统状态"). Tab labels also come from
    // nav.status / nav.stats (same string), so we look for both
    // tab labels but only the status subtitle.
    expect(w.text()).toContain('系统状态')
    expect(w.text()).toContain('集群总览')
    // Stats panel must NOT have fetched (lazy mount).
    expect(apiStubs.statsGet).not.toHaveBeenCalled()
    expect(apiStubs.nodesList).toHaveBeenCalledTimes(1)
  })

  it('switches active tab when the route changes to /admin/stats', async () => {
    const { w, router } = await mountOverview('/admin/status')
    await router.push('/admin/stats')
    await flushPromises()
    // Stats panel mounts on activation and fires its aggregate fetch.
    expect(apiStubs.statsGet).toHaveBeenCalledTimes(1)
    // Stats subtitle is now in the DOM (Status's subtitle no longer in header).
    expect(w.text()).toContain('运营概览')
  })

  it('clicking the refresh button only refetches the active panel', async () => {
    const { w, router } = await mountOverview('/admin/status')
    apiStubs.nodesList.mockClear()
    apiStubs.inboundsFleet.mockClear()
    apiStubs.statsGet.mockClear()
    await w.find('button[type="button"]').trigger('click')
    await flushPromises()
    expect(apiStubs.nodesList).toHaveBeenCalledTimes(1)
    expect(apiStubs.inboundsFleet).toHaveBeenCalledTimes(1)
    expect(apiStubs.statsGet).not.toHaveBeenCalled()

    // Switch to Stats tab, refresh should hit Stats APIs only.
    await router.push('/admin/stats')
    await flushPromises()
    apiStubs.nodesList.mockClear()
    apiStubs.inboundsFleet.mockClear()
    apiStubs.statsGet.mockClear()
    apiStubs.plansList.mockClear()
    await w.find('button[type="button"]').trigger('click')
    await flushPromises()
    expect(apiStubs.statsGet).toHaveBeenCalledTimes(1)
    expect(apiStubs.plansList).toHaveBeenCalledTimes(1)
    expect(apiStubs.nodesList).not.toHaveBeenCalled()
  })
})
