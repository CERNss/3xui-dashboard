import { screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { portalProfileApi } from '@/api/portal/profile'
import { portalTrafficApi } from '@/api/portal/traffic'
import '@/i18n'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Usage from './Usage'

vi.mock('@/api/portal/profile', () => ({
  portalProfileApi: {
    get: vi.fn(),
  },
}))

vi.mock('@/api/portal/traffic', () => ({
  portalTrafficApi: {
    own: vi.fn(),
  },
}))

const profileGetMock = vi.mocked(portalProfileApi.get)
const trafficOwnMock = vi.mocked(portalTrafficApi.own)

function renderUsage() {
  return renderWithProviders(<Usage />)
}

beforeEach(() => {
  profileGetMock.mockReset()
  profileGetMock.mockResolvedValue({
    id: 1,
    email: 'alice@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 12345,
    sub_id: 'sub-token',
    created_at: '2026-01-01T00:00:00Z',
  })
  trafficOwnMock.mockReset()
  trafficOwnMock.mockResolvedValue([
    {
      node_id: 1,
      inbound_tag: 'vless-main',
      client_email: 'alice@example.com',
      up: 1024 * 1024,
      down: 3 * 1024 * 1024,
      total: 4 * 1024 * 1024,
      limit: 10 * 1024 * 1024,
      expires_at: '2026-06-01T00:00:00Z',
    },
    {
      node_id: 2,
      inbound_tag: 'trojan-backup',
      client_email: 'alice@example.com',
      up: 512 * 1024,
      down: 512 * 1024,
      total: 1024 * 1024,
      limit: 2 * 1024 * 1024,
      expires_at: '2026-06-10T00:00:00Z',
    },
  ])
})

describe('Usage', () => {
  it('renders aggregate traffic stats, percentage, balance, and subscription URL', async () => {
    renderUsage()

    expect(await screen.findByRole('heading', { name: 'Hi, alice' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '5.00 MiB' })).toBeInTheDocument()
    expect(screen.getByText('41.7% / limit 12.00 MiB')).toBeInTheDocument()
    expect(screen.getByText('¥123.45')).toBeInTheDocument()
    expect(screen.getByText('http://localhost:3000/sub/sub-token')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('across 2 nodes')).toBeInTheDocument()
  })

  it('renders per-client upload, download, limit, and expiry rows', async () => {
    renderUsage()

    const mainRow = await screen.findByRole('row', { name: /#1 vless-main/i })
    expect(within(mainRow).getByText('1.00 MiB')).toBeInTheDocument()
    expect(within(mainRow).getByText('3.00 MiB')).toBeInTheDocument()
    expect(within(mainRow).getByText('4.00 MiB')).toBeInTheDocument()
    expect(within(mainRow).getByText('/ 10.00 MiB')).toBeInTheDocument()

    const backupRow = screen.getByRole('row', { name: /#2 trojan-backup/i })
    expect(within(backupRow).getAllByText('512.00 KiB')).toHaveLength(2)
    expect(within(backupRow).getByText('1.00 MiB')).toBeInTheDocument()
    expect(within(backupRow).getByText('/ 2.00 MiB')).toBeInTheDocument()
  })
})
