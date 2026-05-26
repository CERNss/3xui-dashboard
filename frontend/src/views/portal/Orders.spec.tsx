import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { portalBillingApi, type Order, type Plan } from '@/api/portal/billing'
import type { UserProfile } from '@/api/portal/profile'
import '@/i18n'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Orders from './Orders'

let orders: Order[] = []
let plans: Plan[] = []
let profile: UserProfile
const ordersRefetch = vi.fn()

vi.mock('@/api/portal/billing', () => ({
  portalBillingApi: {
    getOrder: vi.fn(),
  },
}))

vi.mock('@/hooks/queries/portal/billing', () => ({
  usePortalOrdersList: () => ({ data: orders, error: null, isLoading: false, refetch: ordersRefetch }),
  usePortalPlansList: () => ({ data: plans, error: null, isLoading: false }),
}))

vi.mock('@/hooks/queries/portal/profile', () => ({
  useProfile: () => ({ data: profile, error: null, isLoading: false }),
}))

const getOrderMock = vi.mocked(portalBillingApi.getOrder)

function makePlan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: 1,
    name: 'Pro 30d',
    price_cents: 500,
    traffic_limit_bytes: 100 * 1024 * 1024 * 1024,
    duration_days: 30,
    enabled: true,
    ...overrides,
  }
}

function makeProfile(overrides: Partial<UserProfile> = {}): UserProfile {
  return {
    id: 1,
    email: 'alice@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 1000,
    sub_id: 'sub-1',
    created_at: '2026-05-01T00:00:00Z',
    ...overrides,
  }
}

function makeOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: 100,
    user_id: 1,
    plan_id: 1,
    idempotency_key: 'key',
    price_cents: 500,
    status: 'completed',
    created_at: '2026-05-20T00:00:00Z',
    payment_method: 'balance',
    ...overrides,
  }
}

function renderOrders() {
  return renderWithProviders(<Orders />)
}

beforeEach(() => {
  vi.useRealTimers()
  plans = [makePlan()]
  orders = [makeOrder()]
  profile = makeProfile()
  ordersRefetch.mockReset()
  getOrderMock.mockReset()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: vi.fn() },
  })
})

describe('Portal Orders', () => {
  it('lists order history with plan name, amount, status, and created time', () => {
    orders = [
      makeOrder({ id: 101, status: 'completed', created_at: '2026-05-21T00:00:00Z' }),
      makeOrder({ id: 102, plan_id: 2, status: 'failed', created_at: '2026-05-20T00:00:00Z' }),
    ]
    plans = [makePlan({ id: 1, name: 'Pro 30d' }), makePlan({ id: 2, name: 'Lite 7d' })]
    renderOrders()

    expect(screen.getByRole('heading', { name: 'Orders' })).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('#101')).toBeInTheDocument()
    expect(screen.getByText('Pro 30d')).toBeInTheDocument()
    expect(screen.getAllByText('¥5.00')[0]).toBeInTheDocument()
    expect(screen.getByText('Lite 7d')).toBeInTheDocument()
  })

  it('renders legacy-parity status badge colors', () => {
    orders = [
      makeOrder({ id: 1, status: 'completed' }),
      makeOrder({ id: 2, status: 'failed' }),
      makeOrder({ id: 3, status: 'pending' }),
      makeOrder({ id: 4, status: 'payment_pending', payment_method: 'alipay' }),
    ]
    renderOrders()

    expect(screen.getByText('Completed')).toHaveClass('ant-tag-green')
    expect(screen.getByText('Failed')).toHaveClass('ant-tag-red')
    expect(screen.getByText('Provisioning')).toHaveClass('ant-tag-blue')
    expect(screen.getByText('Awaiting payment')).toHaveClass('ant-tag-orange')
  })

  it('opens AlipayPayModal when continuing an Alipay payment', async () => {
    const user = userEvent.setup()
    orders = [
      makeOrder({
        id: 200,
        status: 'payment_pending',
        payment_method: 'alipay',
        payment_target_url: 'https://qr.alipay.com/bax123',
        payment_expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
      }),
    ]
    getOrderMock.mockResolvedValue(orders[0])
    renderOrders()

    await user.click(screen.getByRole('button', { name: 'Continue payment' }))

    await waitFor(() => expect(getOrderMock).toHaveBeenCalledWith(200))
    expect(await screen.findByText(/#200 · ¥5.00/)).toBeInTheDocument()
  })

  it('redirects to Stripe checkout when continuing a Stripe payment', async () => {
    const user = userEvent.setup()
    orders = [
      makeOrder({
        id: 201,
        status: 'payment_pending',
        payment_method: 'stripe',
        payment_target_url: 'https://checkout.stripe.com/c/pay/cs_test',
      }),
    ]
    getOrderMock.mockResolvedValue(orders[0])
    renderOrders()

    await user.click(screen.getByRole('button', { name: 'Continue payment' }))

    await waitFor(() => expect(window.location.assign).toHaveBeenCalledWith('https://checkout.stripe.com/c/pay/cs_test'))
  })

  it('does not continue payment for already paid orders', () => {
    orders = [makeOrder({ status: 'completed' })]
    renderOrders()

    expect(screen.queryByRole('button', { name: 'Continue payment' })).not.toBeInTheDocument()
  })

  it('shows an error when refreshed payment link is missing', async () => {
    const user = userEvent.setup()
    orders = [makeOrder({ id: 202, status: 'payment_pending', payment_method: 'alipay' })]
    getOrderMock.mockResolvedValue(makeOrder({ id: 202, status: 'payment_pending', payment_method: 'alipay', payment_target_url: '' }))
    renderOrders()

    await user.click(screen.getByRole('button', { name: 'Continue payment' }))

    expect(await screen.findByText('Payment link expired. Please start a new order.')).toBeInTheDocument()
  })

  it('renders unknown plan fallback and empty state link', () => {
    orders = [makeOrder({ plan_id: 99 })]
    plans = []
    const { rerender } = renderOrders()

    expect(screen.getByText('Plan #99')).toBeInTheDocument()

    orders = []
    rerender(<Orders />)
    expect(screen.getByText('No orders yet')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See plans' })).toHaveAttribute('href', '/portal/plans')
  })

  it('renders method labels inside each row', () => {
    orders = [makeOrder({ id: 1, payment_method: 'alipay' }), makeOrder({ id: 2, payment_method: 'stripe' })]
    renderOrders()

    const rows = screen.getAllByRole('row')
    expect(within(rows[1]).getByText('Alipay')).toBeInTheDocument()
    expect(within(rows[2]).getByText('Stripe')).toBeInTheDocument()
  })
})
