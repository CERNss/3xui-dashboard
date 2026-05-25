import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Order, PaymentMethod, Plan } from '@/api/portal/billing'
import type { UserProfile } from '@/api/portal/profile'
import '@/i18n'
import Plans from './Plans'

const purchaseMutateAsync = vi.fn()
const purchaseViaPaymentMutateAsync = vi.fn()
const profileRefetch = vi.fn()

let plans: Plan[] = []
let methods: PaymentMethod[] = []
let profile: UserProfile

vi.mock('@/hooks/queries/portal/billing', () => ({
  usePortalPlansList: () => ({ data: plans, error: null, isLoading: false }),
  usePaymentMethods: () => ({ data: methods, error: null, isLoading: false }),
  usePurchasePlan: () => ({ error: null, isPending: false, mutateAsync: purchaseMutateAsync }),
  usePurchaseViaPayment: () => ({ error: null, isPending: false, mutateAsync: purchaseViaPaymentMutateAsync }),
}))

vi.mock('@/hooks/queries/portal/profile', () => ({
  useProfile: () => ({ data: profile, error: null, isLoading: false, refetch: profileRefetch }),
}))

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

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location">{location.pathname}</span>
}

function renderPlans() {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/portal/plans']} future={{ v7_relativeSplatPath: true, v7_startTransition: true }}>
        <Routes>
          <Route path="/portal/plans" element={<Plans />} />
          <Route path="/portal/orders" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.useRealTimers()
  plans = [makePlan()]
  methods = ['balance']
  profile = makeProfile()
  purchaseMutateAsync.mockReset()
  purchaseMutateAsync.mockResolvedValue(makeOrder())
  purchaseViaPaymentMutateAsync.mockReset()
  profileRefetch.mockReset()
  vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
    act(() => {
      void config.onOk?.()
    })
    return { destroy: vi.fn(), update: vi.fn() }
  })
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: vi.fn() },
  })
})

describe('Portal Plans', () => {
  it('renders enabled plan cards and filters disabled plans', () => {
    plans = [makePlan({ name: 'Enabled One' }), makePlan({ id: 2, name: 'Hidden Plan', enabled: false })]
    renderPlans()

    expect(screen.getByRole('heading', { name: 'Plans' })).toBeInTheDocument()
    expect(screen.getByText('Enabled One')).toBeInTheDocument()
    expect(screen.queryByText('Hidden Plan')).not.toBeInTheDocument()
  })

  it('hides payment-method picker when only balance is configured', () => {
    renderPlans()

    expect(screen.queryByText('Payment method')).not.toBeInTheDocument()
  })

  it('shows Alipay and Stripe payment methods when configured', () => {
    methods = ['balance', 'alipay', 'stripe']
    renderPlans()

    expect(screen.getByText('Payment method')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Alipay' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Stripe' })).toBeInTheDocument()
  })

  it('disables balance purchase when balance is insufficient', () => {
    profile = makeProfile({ balance_cents: 100 })
    renderPlans()

    expect(screen.getByRole('button', { name: 'Insufficient balance' })).toBeDisabled()
  })

  it('keeps external payment available when balance is insufficient', async () => {
    profile = makeProfile({ balance_cents: 100 })
    methods = ['balance', 'alipay']
    renderPlans()

    fireEvent.click(screen.getByText('Alipay'))
    expect(screen.getByRole('button', { name: 'Buy now' })).toBeEnabled()
  })

  it('alipay branch opens AlipayPayModal instead of calling balance purchase', async () => {
    const user = userEvent.setup()
    methods = ['balance', 'alipay']
    purchaseViaPaymentMutateAsync.mockResolvedValue(
      makeOrder({
        id: 200,
        status: 'payment_pending',
        payment_method: 'alipay',
        payment_target_url: 'https://qr.alipay.com/bax123',
        payment_expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
      }),
    )
    renderPlans()

    fireEvent.click(screen.getByText('Alipay'))
    await user.click(screen.getByRole('button', { name: 'Buy now' }))

    await waitFor(() =>
      expect(purchaseViaPaymentMutateAsync).toHaveBeenCalledWith({
        provider: 'alipay',
        input: expect.objectContaining({ plan_id: 1, idempotency_key: expect.any(String) }),
      }),
    )
    expect(purchaseMutateAsync).not.toHaveBeenCalled()
    expect(await screen.findByText(/#200 · ¥5.00/)).toBeInTheDocument()
  })

  it('stripe branch redirects to payment_target_url', async () => {
    const user = userEvent.setup()
    methods = ['balance', 'stripe']
    purchaseViaPaymentMutateAsync.mockResolvedValue(
      makeOrder({
        id: 201,
        status: 'payment_pending',
        payment_method: 'stripe',
        payment_target_url: 'https://checkout.stripe.com/c/pay/cs_test',
      }),
    )
    renderPlans()

    fireEvent.click(screen.getByText('Stripe'))
    await user.click(screen.getByRole('button', { name: 'Buy now' }))

    await waitFor(() => expect(window.location.assign).toHaveBeenCalledWith('https://checkout.stripe.com/c/pay/cs_test'))
    expect(purchaseViaPaymentMutateAsync).toHaveBeenCalledWith({
      provider: 'stripe',
      input: expect.objectContaining({ plan_id: 1 }),
    })
  })

  it('balance branch calls purchase and then navigates to orders', async () => {
    const user = userEvent.setup()
    renderPlans()

    await user.click(screen.getByRole('button', { name: 'Buy now' }))
    await waitFor(() =>
      expect(purchaseMutateAsync).toHaveBeenCalledWith(expect.objectContaining({ plan_id: 1, idempotency_key: expect.any(String) })),
    )
    expect(purchaseViaPaymentMutateAsync).not.toHaveBeenCalled()

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/portal/orders'))
  })

  it('renders plan traffic, duration, and optional IP limit text', () => {
    plans = [makePlan({ ip_limit: 2 })]
    renderPlans()

    const card = screen.getByText('Pro 30d').closest('.ant-card') as HTMLElement
    expect(within(card).getByText(/100 GB/)).toBeInTheDocument()
    expect(within(card).getByText('30 days valid')).toBeInTheDocument()
    expect(within(card).getByText('Up to 2 IPs')).toBeInTheDocument()
  })
})
