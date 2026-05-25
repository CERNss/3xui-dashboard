import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { AdminOrder } from '@/api/admin/orders'

const apiStubs = vi.hoisted(() => ({
  ordersList: vi.fn(),
  plansList: vi.fn(),
  usersList: vi.fn(),
}))
vi.mock('@/api/admin/orders', () => ({
  adminOrdersApi: { list: apiStubs.ordersList },
}))
vi.mock('@/api/admin/plans', () => ({
  adminPlansApi: { list: apiStubs.plansList },
}))
vi.mock('@/api/admin/users', () => ({
  adminUsersApi: { list: apiStubs.usersList },
}))

import Orders from './Orders.vue'

function makeOrder(over: Partial<AdminOrder> = {}): AdminOrder {
  return {
    id: 100,
    user_id: 1,
    plan_id: 1,
    idempotency_key: 'k-abc',
    price_cents: 500,
    status: 'completed',
    client_ownership_id: 1,
    error_message: '',
    created_at: '2026-05-20T10:00:00Z',
    completed_at: '2026-05-20T10:00:05Z',
    ...over,
  }
}

async function mountOrders() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/orders', component: { template: '<div/>' } }],
  })
  await router.push('/admin/orders')
  await router.isReady()
  const w = mount(Orders, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.ordersList.mockResolvedValue({
    orders: [
      makeOrder(),
      makeOrder({ id: 101, price_cents: 100, status: 'failed', user_id: 2 }),
      makeOrder({ id: 102, price_cents: 200, status: 'refunded' }),
    ],
    limit: 200,
    offset: 0,
  })
  apiStubs.plansList.mockResolvedValue([
    { id: 1, name: 'Pro 30d', price_cents: 500, duration_days: 30, traffic_limit_bytes: 0, enabled: true },
  ])
  apiStubs.usersList.mockResolvedValue({
    users: [
      { id: 1, email: 'alice@example.com', email_verified: true, status: 'active', balance_cents: 1000, sub_id: 's1', created_at: '', updated_at: '' },
      { id: 2, email: 'bob@example.com', email_verified: true, status: 'active', balance_cents: 0, sub_id: 's2', created_at: '', updated_at: '' },
    ],
    limit: 500,
    offset: 0,
  })
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Orders.vue smoke', () => {
  it('mounts and renders order controls', async () => {
    const w = await mountOrders()
    expect(w.text()).toContain('总订单')
    expect(w.find('button[title="刷新"]').exists()).toBe(true)
  })

  it('renders order rows joined with plan + user lookups', async () => {
    const w = await mountOrders()
    // Plan name lookup
    expect(w.text()).toContain('Pro 30d')
    // User email lookup (both users render — different rows)
    expect(w.text()).toContain('alice@example.com')
    expect(w.text()).toContain('bob@example.com')
    // Money formatting
    expect(w.text()).toContain('¥5.00')
    expect(w.text()).toContain('¥1.00')
  })

  it('exposes a status filter as button group', async () => {
    const w = await mountOrders()
    // Filter is button-based (not <select>); each button is one
    // status with a Chinese label via statusPill(). Smoke-check
    // the localized labels render.
    const buttons = w.findAll('button').map((b) => b.text().trim())
    expect(buttons).toContain('全部')
    expect(buttons).toContain('已完成')
    expect(buttons).toContain('失败')
  })

  it('filters rows when a status button is clicked', async () => {
    const w = await mountOrders()
    // Confirm both alice (#100 completed) and bob (#101 failed)
    // visible first.
    expect(w.text()).toContain('alice@example.com')
    expect(w.text()).toContain('bob@example.com')
    // Click the "失败" filter
    const failedBtn = w.findAll('button').find((b) => b.text().trim() === '失败')
    expect(failedBtn).toBeDefined()
    await failedBtn!.trigger('click')
    await flushPromises()
    // Now only bob (the failed one) should show
    expect(w.text()).toContain('bob@example.com')
    expect(w.text()).not.toContain('alice@example.com')
  })

  it('hits the three list endpoints once on mount', async () => {
    await mountOrders()
    expect(apiStubs.ordersList).toHaveBeenCalledTimes(1)
    expect(apiStubs.plansList).toHaveBeenCalledTimes(1)
    expect(apiStubs.usersList).toHaveBeenCalledTimes(1)
  })
})
