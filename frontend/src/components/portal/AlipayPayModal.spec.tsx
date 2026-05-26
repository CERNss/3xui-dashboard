import { act, fireEvent, render, screen } from '@testing-library/react'
import QRCode from 'qrcode'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { portalBillingApi, type Order } from '@/api/portal/billing'
import '@/i18n'
import AlipayPayModal from './AlipayPayModal'

vi.mock('qrcode', () => ({
  default: {
    toDataURL: vi.fn(),
  },
}))

vi.mock('@/api/portal/billing', () => ({
  portalBillingApi: {
    getOrder: vi.fn(),
  },
}))

const getOrderMock = vi.mocked(portalBillingApi.getOrder)
const toDataURLMock = vi.mocked(QRCode.toDataURL) as unknown as ReturnType<typeof vi.fn>

function makeOrder(overrides: Partial<Order> = {}): Order {
  return {
    id: 42,
    user_id: 1,
    plan_id: 1,
    idempotency_key: 'key-42',
    price_cents: 500,
    status: 'payment_pending',
    created_at: '2026-05-20T00:00:00Z',
    payment_method: 'alipay',
    payment_target_url: 'https://qr.alipay.com/bax12345',
    payment_expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
    ...overrides,
  }
}

beforeEach(() => {
  vi.useRealTimers()
  getOrderMock.mockReset()
  toDataURLMock.mockReset()
  toDataURLMock.mockResolvedValue('data:image/png;base64,fake-qr')
})

afterEach(() => {
  vi.useRealTimers()
})

describe('AlipayPayModal', () => {
  it('renders QR + order info when open with a payment_pending order', async () => {
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    expect(screen.getByText(/Alipay payment|支付宝支付/)).toBeInTheDocument()
    expect(screen.getByText(/#42 · ¥5.00/)).toBeInTheDocument()
    expect(await screen.findByAltText('alipay qr')).toHaveAttribute('src', expect.stringContaining('data:image/png;base64'))
    expect(toDataURLMock).toHaveBeenCalledWith('https://qr.alipay.com/bax12345', expect.objectContaining({ width: 240 }))
  })

  it('does not render dialog content when open=false', () => {
    render(<AlipayPayModal open={false} order={makeOrder()} onOpenChange={vi.fn()} />)

    expect(screen.queryByText(/Alipay payment|支付宝支付/)).not.toBeInTheDocument()
  })

  it('polls getOrder every 3s while waiting', async () => {
    vi.useFakeTimers()
    getOrderMock.mockResolvedValue(makeOrder())
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(getOrderMock).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6000)
    })
    expect(getOrderMock).toHaveBeenCalledTimes(3)
  })

  it('flips to success, emits success, and auto-closes when getOrder returns completed', async () => {
    vi.useFakeTimers()
    const onSuccess = vi.fn()
    const onOpenChange = vi.fn()
    getOrderMock.mockResolvedValue(makeOrder({ status: 'completed' }))
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={onOpenChange} onSuccess={onSuccess} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })

    expect(screen.getByText(/Payment successful|支付成功/)).toBeInTheDocument()
    expect(onSuccess).toHaveBeenCalledWith(expect.objectContaining({ status: 'completed' }))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(900)
    })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('stops polling after success', async () => {
    vi.useFakeTimers()
    getOrderMock.mockResolvedValue(makeOrder({ status: 'completed' }))
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(getOrderMock).toHaveBeenCalledTimes(1)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(9000)
    })
    expect(getOrderMock).toHaveBeenCalledTimes(1)
  })

  it('shows failed state when getOrder returns payment_failed', async () => {
    vi.useFakeTimers()
    getOrderMock.mockResolvedValue(makeOrder({ status: 'payment_failed' }))
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })

    expect(screen.getByText(/Payment failed|支付失败/)).toBeInTheDocument()
  })

  it('shows expired state when getOrder returns payment_expired', async () => {
    vi.useFakeTimers()
    getOrderMock.mockResolvedValue(makeOrder({ status: 'payment_expired' }))
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })

    expect(screen.getByText(/QR code expired|二维码已过期/)).toBeInTheDocument()
  })

  it('expires locally when payment_expires_at passes', async () => {
    vi.useFakeTimers()
    getOrderMock.mockResolvedValue(makeOrder())
    render(
      <AlipayPayModal
        open
        order={makeOrder({ payment_expires_at: new Date(Date.now() + 1000).toISOString() })}
        onOpenChange={vi.fn()}
      />,
    )

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })

    expect(screen.getByText(/QR code expired|二维码已过期/)).toBeInTheDocument()
  })

  it('shows the Alipay deep link with the target URL', async () => {
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    expect(await screen.findByRole('link', { name: /Open Alipay app|打开支付宝 APP/ })).toHaveAttribute(
      'href',
      'https://qr.alipay.com/bax12345',
    )
  })

  it('shows an empty pending-order state when order is missing', () => {
    render(<AlipayPayModal open order={null} onOpenChange={vi.fn()} />)

    expect(screen.getByText('No pending order')).toBeInTheDocument()
    expect(toDataURLMock).not.toHaveBeenCalled()
  })

  it('clicking close cancels polling and emits open=false', async () => {
    vi.useFakeTimers()
    const onOpenChange = vi.fn()
    getOrderMock.mockResolvedValue(makeOrder())
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={onOpenChange} />)

    fireEvent.click(screen.getByRole('button', { name: /close/i }))
    expect(onOpenChange).toHaveBeenCalledWith(false)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(9000)
    })
    expect(getOrderMock).not.toHaveBeenCalled()
  })

  it('tolerates transient getOrder errors and keeps polling', async () => {
    vi.useFakeTimers()
    getOrderMock.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce(makeOrder({ status: 'completed' }))
    render(<AlipayPayModal open order={makeOrder()} onOpenChange={vi.fn()} />)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(screen.getByText(/Scan with the Alipay app to complete payment|使用支付宝 App 扫码完成支付/)).toBeInTheDocument()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(screen.getByText(/Payment successful|支付成功/)).toBeInTheDocument()
  })
})
