import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminOrdersApi, type AdminOrder } from '@/api/admin/orders'
import { adminPlansApi } from '@/api/admin/plans'
import { adminUsersApi } from '@/api/admin/users'
import Orders from './Orders'

vi.mock('@/api/admin/orders', () => ({
  adminOrdersApi: {
    list: vi.fn(),
    refund: vi.fn(),
  },
}))

vi.mock('@/api/admin/plans', () => ({
  adminPlansApi: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/admin/users', () => ({
  adminUsersApi: {
    list: vi.fn(),
  },
}))

const ordersListMock = vi.mocked(adminOrdersApi.list)
const refundMock = vi.mocked(adminOrdersApi.refund)
const plansListMock = vi.mocked(adminPlansApi.list)
const usersListMock = vi.mocked(adminUsersApi.list)

function makeOrder(overrides: Partial<AdminOrder> = {}): AdminOrder {
  return {
    id: 100,
    user_id: 1,
    plan_id: 1,
    idempotency_key: 'k-100',
    price_cents: 500,
    status: 'completed',
    client_ownership_id: 1,
    error_message: '',
    created_at: '2026-05-20T10:00:00Z',
    completed_at: '2026-05-20T10:00:05Z',
    ...overrides,
  }
}

function renderOrders() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <Orders />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  ordersListMock.mockReset()
  refundMock.mockReset()
  plansListMock.mockReset()
  usersListMock.mockReset()

  ordersListMock.mockResolvedValue({
    orders: [
      makeOrder(),
      makeOrder({ id: 101, user_id: 2, price_cents: 100, status: 'failed', created_at: '2026-05-21T10:00:00Z' }),
      makeOrder({ id: 102, status: 'refunded', price_cents: 200, created_at: '2026-05-19T10:00:00Z' }),
    ],
    limit: 200,
    offset: 0,
  })
  plansListMock.mockResolvedValue([
    { id: 1, name: 'Pro 30d', price_cents: 500, duration_days: 30, traffic_limit_bytes: 0, enabled: true },
  ])
  usersListMock.mockResolvedValue({
    users: [
      { id: 1, email: 'alice@example.com', email_verified: true, status: 'active', balance_cents: 1000, auto_renew: false, sub_id: 's1', created_at: '', updated_at: '' },
      { id: 2, email: 'bob@example.com', email_verified: true, status: 'active', balance_cents: 0, auto_renew: false, sub_id: 's2', created_at: '', updated_at: '' },
    ],
    limit: 500,
    offset: 0,
  })
  refundMock.mockResolvedValue(makeOrder({ id: 100, status: 'refunded' }))
})

describe('Orders', () => {
  it('loads orders, plans, and users for joined rows and KPI totals', async () => {
    renderOrders()

    expect(await screen.findAllByText('Pro 30d')).toHaveLength(3)
    expect(screen.getAllByText('alice@example.com')).toHaveLength(2)
    expect(screen.getByText('bob@example.com')).toBeInTheDocument()
    expect(screen.getAllByText('¥5.00')).toHaveLength(2)
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()

    expect(ordersListMock).toHaveBeenCalledWith({ limit: 200 })
    expect(plansListMock).toHaveBeenCalledTimes(1)
    expect(usersListMock).toHaveBeenCalledWith({ limit: 500 })
  })

  it('filters by status and keeps rows created-desc by default', async () => {
    renderOrders()

    await screen.findByText('bob@example.com')
    const cellsBeforeFilter = screen.getAllByRole('cell').map((cell) => cell.textContent ?? '')
    expect(cellsBeforeFilter.findIndex((text) => text.includes('#101'))).toBeLessThan(
      cellsBeforeFilter.findIndex((text) => text.includes('#100')),
    )

    await userEvent.click(screen.getByText('Failed'))

    expect(screen.getByText('bob@example.com')).toBeInTheDocument()
    expect(screen.queryByText('alice@example.com')).not.toBeInTheDocument()
  })

  it('only exposes refund for completed or paid orders and invalidates after mutation', async () => {
    renderOrders()

    const aliceRow = (await screen.findAllByRole('row', { name: /#100/i }))[0]
    expect(within(aliceRow).getByRole('button', { name: 'Refund' })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Refund' })).toHaveLength(1)

    await userEvent.click(within(aliceRow).getByRole('button', { name: 'Refund' }))
    const confirmButtons = await screen.findAllByRole('button', { name: 'Refund' })
    await userEvent.click(confirmButtons[confirmButtons.length - 1])

    await waitFor(() => expect(refundMock).toHaveBeenCalledWith(100, 'admin manual refund'))
    await waitFor(() => expect(ordersListMock).toHaveBeenCalledTimes(2))
  })
})
