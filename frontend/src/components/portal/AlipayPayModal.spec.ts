import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { Order } from '@/api/portal/billing'

// Stub portalBillingApi BEFORE the component import so the modal's
// pollOnce calls hit our mock.
const apiStubs = vi.hoisted(() => ({
  getOrder: vi.fn<[number], Promise<Order>>(),
}))
vi.mock('@/api/portal/billing', () => ({
  portalBillingApi: { getOrder: apiStubs.getOrder },
}))

// Stub qrcode — its real impl writes a 240x240 PNG which is wasted
// effort + ~10ms per test. We just need the render call to resolve.
vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,fake-qr') },
}))

import AlipayPayModal from './AlipayPayModal.vue'

const baseOrder: Order = {
  id: 42,
  user_id: 1,
  plan_id: 1,
  idempotency_key: 'k',
  price_cents: 500,
  status: 'payment_pending',
  created_at: '2026-05-20T00:00:00Z',
  payment_method: 'alipay',
  payment_target_url: 'https://qr.alipay.com/bax12345',
  // Default to 15 min in the future — long enough that tests using
  // real time don't trip the expiry path. Tests that exercise expiry
  // override this explicitly.
  payment_expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
}

function makeOrder(over: Partial<Order> = {}): Order {
  return { ...baseOrder, ...over }
}

async function mountModal(props: { open: boolean; order: Order | null }) {
  const w = mount(AlipayPayModal, {
    props,
    attachTo: document.body,
  })
  // The watch on `props.order` fires renderQR (async) → wait
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.getOrder.mockReset()
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('AlipayPayModal', () => {
  it('renders QR + order info when open with a payment_pending order', async () => {
    await mountModal({ open: true, order: makeOrder() })
    // Teleported into body
    expect(document.body.textContent).toContain('支付宝支付')
    expect(document.body.textContent).toContain('订单 #42')
    expect(document.body.textContent).toContain('¥5.00')
    const img = document.body.querySelector('img[alt="alipay qr"]') as HTMLImageElement | null
    expect(img).not.toBeNull()
    expect(img!.src).toContain('data:image/png;base64')
  })

  it('does not render anything when open=false', async () => {
    await mountModal({ open: false, order: makeOrder() })
    expect(document.body.textContent).not.toContain('支付宝支付')
  })

  it('polls getOrder every 3s while in waiting state', async () => {
    vi.useFakeTimers()
    apiStubs.getOrder.mockResolvedValue(makeOrder())
    await mountModal({ open: true, order: makeOrder() })

    expect(apiStubs.getOrder).not.toHaveBeenCalled()
    // Advance 3 ticks — should fire 3 polls. flushPromises after
    // each advance forces vitest to settle the then-handlers from
    // the mocked getOrder so the call-count assertion is stable
    // (vitest 2.1's advanceTimersByTimeAsync settles microtasks
    // between timers but the boundary's been flaky across releases —
    // explicit flushPromises is the belt-and-suspenders fix).
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(apiStubs.getOrder).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(apiStubs.getOrder).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(apiStubs.getOrder).toHaveBeenCalledTimes(3)
  })

  it('flips to success state + emits "success" when getOrder returns completed', async () => {
    vi.useFakeTimers()
    // First poll returns the completed order
    apiStubs.getOrder.mockResolvedValue(makeOrder({ status: 'completed' }))

    const w = await mountModal({ open: true, order: makeOrder() })
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(document.body.textContent).toContain('支付成功')
    expect(w.emitted('success')).toBeTruthy()
    // The emitted payload is the fresh order from getOrder
    expect(w.emitted('success')![0][0]).toMatchObject({ status: 'completed' })
  })

  it('stops polling after success', async () => {
    vi.useFakeTimers()
    apiStubs.getOrder.mockResolvedValue(makeOrder({ status: 'completed' }))
    await mountModal({ open: true, order: makeOrder() })

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(apiStubs.getOrder).toHaveBeenCalledTimes(1)

    // Advance more time — no further polls because timers cleared.
    await vi.advanceTimersByTimeAsync(10000)
    await flushPromises()
    expect(apiStubs.getOrder).toHaveBeenCalledTimes(1)
  })

  it('flips to failed state when getOrder returns payment_failed', async () => {
    vi.useFakeTimers()
    apiStubs.getOrder.mockResolvedValue(makeOrder({ status: 'payment_failed' }))
    const w = await mountModal({ open: true, order: makeOrder() })

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(document.body.textContent).toContain('支付失败')
    expect(w.emitted('success')).toBeFalsy()
  })

  it('flips to expired state when getOrder returns payment_expired', async () => {
    vi.useFakeTimers()
    apiStubs.getOrder.mockResolvedValue(makeOrder({ status: 'payment_expired' }))
    await mountModal({ open: true, order: makeOrder() })

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(document.body.textContent).toContain('二维码已过期')
  })

  it('expires locally when payment_expires_at passes (no API call needed)', async () => {
    vi.useFakeTimers()
    // expires_at is 1 second from "now"
    const order = makeOrder({
      payment_expires_at: new Date(Date.now() + 1000).toISOString(),
    })
    // getOrder keeps returning pending — local clock should be the
    // one that flips state.
    apiStubs.getOrder.mockResolvedValue(makeOrder())

    await mountModal({ open: true, order })

    // Advance 2s — countdown's setInterval fires every 1s, so two
    // ticks crosses the expiry boundary.
    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(document.body.textContent).toContain('二维码已过期')
  })

  it('shows the "打开支付宝 APP" deep link with the target URL', async () => {
    await mountModal({ open: true, order: makeOrder() })
    const anchor = [...document.body.querySelectorAll('a')].find(
      (a) => a.textContent?.includes('打开支付宝 APP'),
    )
    expect(anchor).toBeDefined()
    expect(anchor!.getAttribute('href')).toBe('https://qr.alipay.com/bax12345')
  })

  it('Escape key closes the modal (emits cancel via update:open)', async () => {
    const w = await mountModal({ open: true, order: makeOrder() })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await w.vm.$nextTick()
    expect(w.emitted('update:open')).toBeTruthy()
    expect(w.emitted('update:open')![0]).toEqual([false])
  })

  it('clicking the X button closes the modal', async () => {
    const w = await mountModal({ open: true, order: makeOrder() })
    const closeBtn = document.body.querySelector('button[class*="text-surface-400"]') as HTMLButtonElement | null
    expect(closeBtn).not.toBeNull()
    closeBtn!.click()
    await w.vm.$nextTick()
    expect(w.emitted('update:open')).toBeTruthy()
  })

  it('tolerates transient getOrder errors and keeps polling', async () => {
    vi.useFakeTimers()
    apiStubs.getOrder
      .mockRejectedValueOnce(new Error('network blip'))
      .mockResolvedValueOnce(makeOrder({ status: 'completed' }))

    const w = await mountModal({ open: true, order: makeOrder() })

    // First poll throws — state stays waiting
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(document.body.textContent).toContain('使用支付宝 App 扫码完成支付')
    expect(w.emitted('success')).toBeFalsy()

    // Next poll succeeds — flips to success
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(document.body.textContent).toContain('支付成功')
  })
})
