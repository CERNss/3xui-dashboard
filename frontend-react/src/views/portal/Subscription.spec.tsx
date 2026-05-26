import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import QRCode from 'qrcode'
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import { portalProfileApi } from '@/api/portal/profile'
import { portalTrafficApi } from '@/api/portal/traffic'
import '@/i18n'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Subscription from './Subscription'

vi.mock('qrcode', () => ({
  default: {
    toDataURL: vi.fn(),
  },
}))

vi.mock('@/api/portal/profile', () => ({
  portalProfileApi: {
    get: vi.fn(),
    rotateSubID: vi.fn(),
  },
}))

vi.mock('@/api/portal/traffic', () => ({
  portalTrafficApi: {
    own: vi.fn(),
  },
}))

const profileGetMock = vi.mocked(portalProfileApi.get)
const trafficOwnMock = vi.mocked(portalTrafficApi.own)
const qrMock = QRCode.toDataURL as unknown as Mock<(value: string) => Promise<string>>

function renderSubscription() {
  return renderWithProviders(<Subscription />)
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

beforeEach(() => {
  profileGetMock.mockReset()
  profileGetMock.mockResolvedValue({
    id: 1,
    email: 'user@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 1200,
    sub_id: 'sub-token',
    created_at: '2026-01-01T00:00:00Z',
  })
  trafficOwnMock.mockReset()
  trafficOwnMock.mockResolvedValue([
    {
      node_id: 1,
      inbound_tag: 'vless-main',
      client_email: 'user@example.com',
      up: 1024,
      down: 2048,
      total: 3072,
      limit: 4096,
      expires_at: '2026-06-01T00:00:00Z',
    },
  ])
  qrMock.mockReset()
  qrMock.mockImplementation((value: string) => Promise.resolve(`data:image/png;base64,${value}`))
  Object.assign(navigator, {
    clipboard: {
      writeText: vi.fn().mockResolvedValue(undefined),
    },
  })
})

describe('Subscription', () => {
  it('renders the seven formats and keeps base64 URL query-free', async () => {
    renderSubscription()

    expect(await screen.findByRole('button', { name: /Base64/ })).toBeInTheDocument()
    for (const label of ['Clash', 'Sing-box', 'SIP008', 'WireGuard', 'WG (ZIP)', 'JSON']) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }

    expect(screen.getByDisplayValue('http://localhost:3000/sub/sub-token')).toBeInTheDocument()
    expect(qrMock).toHaveBeenLastCalledWith('http://localhost:3000/sub/sub-token', expect.any(Object))
  })

  it.each([
    ['Clash', 'clash', /Full Mihomo config/],
    ['Sing-box', 'singbox', /sing-box JSON/],
    ['SIP008', 'sip008', /Shadowsocks-only/],
    ['WireGuard', 'wireguard', /wg-quick/],
    ['JSON', 'json', /Raw Xray config/],
  ])('adds the format query for %s', async (_label, format, buttonName) => {
    renderSubscription()

    await userEvent.click(await screen.findByRole('button', { name: buttonName }))

    expect(screen.getByDisplayValue(`http://localhost:3000/sub/sub-token?format=${format}`)).toBeInTheDocument()
    await waitFor(() =>
      expect(qrMock).toHaveBeenLastCalledWith(`http://localhost:3000/sub/sub-token?format=${format}`, expect.any(Object)),
    )
  })

  it('suppresses copy and QR for wireguard ZIP and exposes a download button', async () => {
    renderSubscription()

    await userEvent.click(await screen.findByRole('button', { name: /WG \(ZIP\)/ }))

    expect(screen.getByDisplayValue('http://localhost:3000/sub/sub-token?format=wireguard-zip')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /copy/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('img', { name: /subscription QR/i })).not.toBeInTheDocument()

    const download = screen.getByRole('link', { name: /Download file/i })
    expect(download).toHaveAttribute('href', 'http://localhost:3000/sub/sub-token?format=wireguard-zip')
    expect(download).toHaveAttribute('download')
  })

  it('copies the displayed URL and shows the success message', async () => {
    renderSubscription()

    await userEvent.click(await screen.findByRole('button', { name: /copy/i }))

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('http://localhost:3000/sub/sub-token')
    expect(await screen.findByText('Copied')).toBeInTheDocument()
  })

  it('ignores stale QR results when formats are switched rapidly', async () => {
    const first = deferred<string>()
    const second = deferred<string>()
    qrMock
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
      .mockImplementation((value: string) => Promise.resolve(`data:image/png;base64,${value}`))

    renderSubscription()
    await screen.findByDisplayValue('http://localhost:3000/sub/sub-token')

    await userEvent.click(screen.getByRole('button', { name: /Clash/ }))
    second.resolve('data:image/png;base64,current-clash')
    first.resolve('data:image/png;base64,stale-base64')

    const qrRegion = screen.getByLabelText('subscription QR')
    await waitFor(() => expect(within(qrRegion).getByRole('img', { name: /subscription QR/i })).toHaveAttribute('src', 'data:image/png;base64,current-clash'))
    expect(within(qrRegion).getByRole('img', { name: /subscription QR/i })).not.toHaveAttribute('src', 'data:image/png;base64,stale-base64')
  })
})
