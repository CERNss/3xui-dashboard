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
      month_up_bytes: 9.71 * 1024 ** 3,
      month_down_bytes: 106.56 * 1024 ** 3,
      today_up_bytes: 205.51 * 1024 ** 2,
      today_down_bytes: 2.51 * 1024 ** 3,
    },
    top_nodes: [
      { key: 'UK-HY2', up_bytes: 1024 ** 3, down_bytes: 1.68 * 1024 ** 3, bytes: 2.68 * 1024 ** 3 },
      { key: 'JP-HY2', up_bytes: 0, down_bytes: 0.03 * 1024 ** 3, bytes: 0.03 * 1024 ** 3 },
    ],
    top_users: [
      { key: 'kazami@kazami.tech', up_bytes: 1024 ** 3, down_bytes: 1.56 * 1024 ** 3, bytes: 2.56 * 1024 ** 3 },
      { key: 'kris@vtb.live', up_bytes: 0.05 * 1024 ** 3, down_bytes: 0.08 * 1024 ** 3, bytes: 0.13 * 1024 ** 3 },
    ],
    audit: { info: 143, warn: 0, err: 0 },
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
    routes: [
      { path: '/admin/stats', component: { template: '<div/>' } },
      { path: '/admin/audit', component: { template: '<div/>' } },
    ],
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
  it('mounts without throwing', async () => {
    const w = await mountStats()
    expect(w.exists()).toBe(true)
  })

  it('renders the 4 KPI cards aligned with the dashboard screenshot', async () => {
    const w = await mountStats()
    // Card 1: 月新增用户
    expect(w.text()).toContain('月新增用户')
    expect(w.text()).toContain('6')
    // Delta chip: cur=6, prev=3 → +100%
    expect(w.text()).toContain('+100% 对比上月')
    // Card 2: 总用户 + 活跃用户 subtitle
    expect(w.text()).toContain('总用户')
    expect(w.text()).toContain('42')
    expect(w.text()).toContain('活跃用户: 38')
    // Card 3 + 4: traffic
    expect(w.text()).toContain('月上传')
    expect(w.text()).toContain('月下载')
    expect(w.text()).toContain('9.71 GB')
    expect(w.text()).toContain('106.56 GB')
    expect(w.text()).toContain('今日: 2.51 GB')
  })

  it('hides the month-delta chip when both months are zero', async () => {
    apiStubs.statsGet.mockResolvedValue(
      makeStats({ users: { total: 0, active: 0, suspended: 0, month_new: 0, prev_month_new: 0, total_balance_cents: 0, avg_balance_cents: 0 } }),
    )
    const w = await mountStats()
    expect(w.text()).not.toContain('对比上月')
  })

  it('renders the node + user traffic rankings with per-row totals', async () => {
    const w = await mountStats()
    expect(w.text()).toContain('节点流量排行')
    expect(w.text()).toContain('UK-HY2')
    expect(w.text()).toContain('JP-HY2')
    expect(w.text()).toContain('用户流量排行')
    expect(w.text()).toContain('kazami@kazami.tech')
    expect(w.text()).toContain('kris@vtb.live')
    // Per-row total bytes appear next to the bar.
    expect(w.text()).toContain('2.68 GB')
    expect(w.text()).toContain('2.56 GB')
  })

  it('falls back to the empty-state copy when rankings are empty', async () => {
    apiStubs.statsGet.mockResolvedValue(makeStats({ top_nodes: [], top_users: [] }))
    const w = await mountStats()
    // Empty-state text appears at least once per panel (both panels empty).
    const matches = w.text().match(/暂无流量数据/g) ?? []
    expect(matches.length).toBeGreaterThanOrEqual(2)
  })

  it('renders the audit severity strip with the three count chips', async () => {
    const w = await mountStats()
    expect(w.text()).toContain('系统日志')
    expect(w.text()).toContain('信息')
    expect(w.text()).toContain('警告')
    expect(w.text()).toContain('错误')
    // info=143 appears in the colored box. Avoid asserting just "143"
    // because that string also appears in the recent-orders timestamp
    // — assert via the View All link's presence + the total subtext.
    expect(w.text()).toContain('共 143 条')
    expect(w.text()).toContain('查看全部')
  })

  it('renders the recent-orders activity feed (kept below the screenshot blocks)', async () => {
    const w = await mountStats()
    expect(w.text()).toContain('近期订单')
    expect(w.text()).toContain('alice@example.com → Pro 30d')
  })

  it('hits the single aggregate endpoint instead of fanning out', async () => {
    await mountStats()
    expect(apiStubs.statsGet).toHaveBeenCalledTimes(1)
    expect(apiStubs.plansList).toHaveBeenCalledTimes(1)
  })
})
