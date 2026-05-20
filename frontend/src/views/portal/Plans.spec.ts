import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { Order, Plan, PortalInbound, PaymentMethod } from '@/api/portal/billing'
import type { UserProfile } from '@/api/portal/profile'

// Stub the API modules BEFORE importing Plans.vue. Hoisted by vi —
// must be at the top of the file or vi won't pick it up before the
// dynamic component import.
const apiStubs = vi.hoisted(() => ({
  listPlans: vi.fn<[], Promise<Plan[]>>(),
  listInbounds: vi.fn<[], Promise<PortalInbound[]>>(),
  paymentMethods: vi.fn<[], Promise<PaymentMethod[]>>(),
  purchase: vi.fn<[unknown], Promise<Order>>(),
  purchaseViaPayment: vi.fn<[PaymentMethod, unknown], Promise<Order>>(),
  getOrder: vi.fn<[number], Promise<Order>>(),
  profileGet: vi.fn<[], Promise<UserProfile>>(),
}))

vi.mock('@/api/portal/billing', () => ({
  portalBillingApi: {
    listPlans: apiStubs.listPlans,
    listInbounds: apiStubs.listInbounds,
    paymentMethods: apiStubs.paymentMethods,
    purchase: apiStubs.purchase,
    purchaseViaPayment: apiStubs.purchaseViaPayment,
    getOrder: apiStubs.getOrder,
    listOrders: vi.fn(),
  },
}))

vi.mock('@/api/portal/profile', () => ({
  portalProfileApi: { get: apiStubs.profileGet },
}))

// Mount-time imports come AFTER the mocks above.
import Plans from './Plans.vue'

function makePlan(over: Partial<Plan> = {}): Plan {
  return {
    id: 1,
    name: 'Pro 30d',
    price_cents: 500,
    traffic_limit_bytes: 100 * 1024 * 1024 * 1024,
    duration_days: 30,
    enabled: true,
    ...over,
  }
}

function makeInbound(over: Partial<PortalInbound> = {}): PortalInbound {
  return {
    node_id: 1,
    node_name: 'tokyo-1',
    inbound_tag: 'vless-tcp',
    protocol: 'vless',
    remark: '',
    port: 443,
    ...over,
  }
}

function makeProfile(over: Partial<UserProfile> = {}): UserProfile {
  return {
    id: 1,
    email: 'alice@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 1000,
    sub_id: 'sub-1',
    created_at: '2026-05-01T00:00:00Z',
    ...over,
  }
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/portal/plans', component: { template: '<div/>' } },
      { path: '/portal/orders', component: { template: '<div/>' } },
    ],
  })
}

async function mountPlans() {
  const router = makeRouter()
  await router.push('/portal/plans')
  await router.isReady()
  const w = mount(Plans, {
    global: {
      plugins: [router],
      // $t is referenced in the loading template — stub to return
      // the key so the render doesn't crash. Real i18n lookup isn't
      // what we're testing here.
      mocks: { $t: (key: string) => key },
    },
    attachTo: document.body,
  })
  // Wait for onMounted's load() to complete
  await flushPromises()
  return { w, router }
}

beforeEach(() => {
  // Default happy-path: one plan, one inbound, profile with enough balance, only balance method.
  apiStubs.listPlans.mockResolvedValue([makePlan()])
  apiStubs.listInbounds.mockResolvedValue([makeInbound()])
  apiStubs.paymentMethods.mockResolvedValue(['balance'])
  apiStubs.profileGet.mockResolvedValue(makeProfile())
  apiStubs.purchase.mockResolvedValue({
    id: 100, user_id: 1, plan_id: 1, idempotency_key: 'k',
    price_cents: 500, status: 'completed', created_at: '2026-05-20T00:00:00Z',
    payment_method: 'balance',
  })
})

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('Plans.vue smoke', () => {
  it('mounts without errors and renders the page title', async () => {
    const { w } = await mountPlans()
    expect(w.html()).toContain('套餐')
  })

  it('renders plan cards from the API', async () => {
    const { w } = await mountPlans()
    expect(w.text()).toContain('Pro 30d')
    expect(w.text()).toContain('¥5.00')
  })

  it('hides payment-method picker when only balance is configured', async () => {
    const { w } = await mountPlans()
    expect(w.text()).not.toContain('支付方式')
  })

  it('shows payment-method picker when alipay is also configured', async () => {
    apiStubs.paymentMethods.mockResolvedValue(['balance', 'alipay'])
    const { w } = await mountPlans()
    expect(w.text()).toContain('支付方式')
    expect(w.text()).toContain('支付宝')
  })

  it('shows Stripe label when stripe is configured', async () => {
    apiStubs.paymentMethods.mockResolvedValue(['balance', 'stripe'])
    const { w } = await mountPlans()
    expect(w.text()).toContain('Stripe')
  })

  it('shows empty-state when no inbounds are configured', async () => {
    apiStubs.listInbounds.mockResolvedValue([])
    const { w } = await mountPlans()
    expect(w.text()).toContain('当前没有可用的节点入站')
  })

  it('disables the buy button when balance is insufficient', async () => {
    apiStubs.profileGet.mockResolvedValue(makeProfile({ balance_cents: 100 }))
    const { w } = await mountPlans()
    const buyButton = w.findAll('button').find((b) => b.text().includes('余额不足'))
    expect(buyButton).toBeDefined()
    expect(buyButton?.attributes('disabled')).toBeDefined()
  })

  it('alipay branch: opens AlipayPayModal instead of calling balance purchase', async () => {
    apiStubs.paymentMethods.mockResolvedValue(['balance', 'alipay'])
    apiStubs.purchaseViaPayment.mockResolvedValue({
      id: 200, user_id: 1, plan_id: 1, idempotency_key: 'k',
      price_cents: 500, status: 'payment_pending',
      created_at: '2026-05-20T00:00:00Z',
      payment_method: 'alipay',
      payment_target_url: 'https://qr.alipay.com/bax123',
      payment_expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
    })

    const { w } = await mountPlans()
    // Pick alipay
    const alipayRadio = [...w.findAll('input[type="radio"]')].find(
      (r) => (r.element as HTMLInputElement).value === 'alipay',
    )
    expect(alipayRadio).toBeDefined()
    await alipayRadio!.setValue()

    // The buy button is the only enabled "立即购买" button on the page
    // since profile has enough balance. Trigger click.
    const buyButton = w.findAll('button').find((b) => b.text().includes('立即购买'))
    await buyButton!.trigger('click')
    await flushPromises()

    // Confirm dialog open — accept it
    const confirmButton = [...document.body.querySelectorAll('button')].find(
      (b) => b.textContent?.includes('支付宝支付'),
    )
    expect(confirmButton).toBeDefined()
    confirmButton!.click()
    await flushPromises()

    expect(apiStubs.purchaseViaPayment).toHaveBeenCalledWith(
      'alipay',
      expect.objectContaining({ plan_id: 1, node_id: 1, inbound_tag: 'vless-tcp' }),
    )
    expect(apiStubs.purchase).not.toHaveBeenCalled()
  })

  it('stripe branch: redirects via window.location.href to payment_target_url', async () => {
    apiStubs.paymentMethods.mockResolvedValue(['balance', 'stripe'])
    apiStubs.purchaseViaPayment.mockResolvedValue({
      id: 201, user_id: 1, plan_id: 1, idempotency_key: 'k',
      price_cents: 500, status: 'payment_pending',
      created_at: '2026-05-20T00:00:00Z',
      payment_method: 'stripe',
      payment_target_url: 'https://checkout.stripe.com/c/pay/cs_test_a1B2c3',
      payment_expires_at: new Date(Date.now() + 30 * 60 * 1000).toISOString(),
    })

    // Capture window.location.href assignments via a spy.
    const hrefSpy = vi.fn<[string], void>()
    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        get href() { return '' },
        set href(v: string) { hrefSpy(v) },
      },
    })

    const { w } = await mountPlans()
    const stripeRadio = [...w.findAll('input[type="radio"]')].find(
      (r) => (r.element as HTMLInputElement).value === 'stripe',
    )
    await stripeRadio!.setValue()

    const buyButton = w.findAll('button').find((b) => b.text().includes('立即购买'))
    await buyButton!.trigger('click')
    await flushPromises()

    const confirmButton = [...document.body.querySelectorAll('button')].find(
      (b) => b.textContent?.includes('Stripe支付'),
    )
    confirmButton!.click()
    await flushPromises()

    expect(apiStubs.purchaseViaPayment).toHaveBeenCalledWith('stripe', expect.any(Object))
    expect(hrefSpy).toHaveBeenCalledWith('https://checkout.stripe.com/c/pay/cs_test_a1B2c3')
  })

  it('balance branch: calls portalBillingApi.purchase (not purchaseViaPayment)', async () => {
    const { w } = await mountPlans()
    const buyButton = w.findAll('button').find((b) => b.text().includes('立即购买'))
    await buyButton!.trigger('click')
    await flushPromises()

    const confirmButton = [...document.body.querySelectorAll('button')].find(
      (b) => b.textContent?.includes('余额支付'),
    )
    confirmButton!.click()
    await flushPromises()

    expect(apiStubs.purchase).toHaveBeenCalledWith(
      expect.objectContaining({ plan_id: 1, node_id: 1, inbound_tag: 'vless-tcp' }),
    )
    expect(apiStubs.purchaseViaPayment).not.toHaveBeenCalled()
  })
})
