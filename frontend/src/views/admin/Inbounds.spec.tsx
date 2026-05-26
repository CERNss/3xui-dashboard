import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Inbounds from './Inbounds'
import type { FleetInbound, FleetResult, Inbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

const setEnableMutateAsync = vi.fn()
const removeMutateAsync = vi.fn()
const resetMutateAsync = vi.fn()
const fleetRefetch = vi.fn()
const nodesRefetch = vi.fn()

let fleet: FleetResult
let nodes: Node[]

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useInboundsFleet: () => ({ data: fleet, error: null, isFetching: false, isLoading: false, refetch: fleetRefetch }),
  useSetInboundEnable: () => ({ error: null, isPending: false, mutateAsync: setEnableMutateAsync }),
  useRemoveInbound: () => ({ error: null, isPending: false, mutateAsync: removeMutateAsync }),
  useResetInboundTraffic: () => ({ error: null, isPending: false, mutateAsync: resetMutateAsync }),
  useCreateInbound: () => ({ error: null, isPending: false, mutateAsync: vi.fn() }),
  useUpdateInbound: () => ({ error: null, isPending: false, mutateAsync: vi.fn() }),
}))

vi.mock('@/hooks/queries/admin/nodes', () => ({
  useNodesList: () => ({ data: nodes, error: null, isFetching: false, isLoading: false, refetch: nodesRefetch }),
}))

function makeInbound(overrides: Partial<Inbound> = {}): Inbound {
  return {
    id: 1,
    up: 1024,
    down: 2048,
    total: 1024 * 1024 * 1024,
    allTime: 4096,
    remark: 'Main inbound',
    enable: true,
    expiryTime: 0,
    trafficReset: 'never',
    clientStats: [{ id: 1, inboundId: 1, enable: true, email: 'alice@example.com', up: 1, down: 2, allTime: 3, expiryTime: 0, total: 0, reset: 0 }],
    listen: '',
    port: 443,
    protocol: 'vless',
    settings: JSON.stringify({ clients: [{ id: 'uuid-1', flow: '', email: 'alice@example.com', expiryTime: 0, enable: true }] }),
    streamSettings: JSON.stringify({ network: 'ws', security: 'tls', wsSettings: { path: '/ws' }, tlsSettings: { serverName: 'edge.example.com' } }),
    sniffing: JSON.stringify({ enabled: true, destOverride: ['http', 'tls'] }),
    tag: 'inbound-443',
    ...overrides,
  }
}

function makeFleetRow(overrides: Partial<FleetInbound> = {}): FleetInbound {
  return {
    node_id: 7,
    node_name: 'Tokyo Node',
    inbound: makeInbound(),
    ...overrides,
  }
}

function renderInbounds() {
  return renderWithProviders(<Inbounds />)
}

beforeEach(() => {
  nodes = [
    {
      id: 7,
      name: 'Tokyo Node',
      area: 'jp',
      province: 'Tokyo',
      scheme: 'https',
      host: 'edge.example.com',
      port: 2053,
      base_path: '/panel/',
      enabled: true,
      status: 'online',
      cpu_pct: 0,
      mem_pct: 0,
      xray_version: '1.8.24',
      uptime_s: 0,
      created_at: '',
      updated_at: '',
    },
  ]
  fleet = { inbounds: [makeFleetRow()] }
  setEnableMutateAsync.mockResolvedValue({})
  removeMutateAsync.mockResolvedValue({})
  resetMutateAsync.mockResolvedValue({})
  fleetRefetch.mockReset()
  nodesRefetch.mockReset()
  vi.restoreAllMocks()
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (query: string) =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }) as unknown as MediaQueryList,
  )
})

describe('Inbounds', () => {
  it('renders fleet rows through ResponsiveListTable and opens editor', async () => {
    const user = userEvent.setup()
    renderInbounds()

    expect(screen.getByRole('heading', { name: 'Inbounds' })).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('Main inbound')).toBeInTheDocument()
    expect(screen.getByText(/vless \/ ws \/ tls/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Edit inbound-443' }))
    expect(screen.getByRole('dialog', { name: 'Edit inbound inbound-443' })).toBeInTheDocument()
    expect(screen.getByLabelText('Remark')).toHaveValue('Main inbound')
  })

  it('toggles, refreshes, resets, and deletes inbounds', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderInbounds()

    await user.click(screen.getByRole('switch', { name: 'Disable inbound-443' }))
    expect(setEnableMutateAsync).toHaveBeenCalledWith({ nodeID: 7, tag: 'inbound-443', enable: false })

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(fleetRefetch).toHaveBeenCalledTimes(1)
    expect(nodesRefetch).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: 'Reset traffic inbound-443' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Reset inbound traffic' }))
    await waitFor(() => expect(resetMutateAsync).toHaveBeenCalledWith({ nodeID: 7, tag: 'inbound-443' }))

    await user.click(screen.getByRole('button', { name: 'Delete inbound-443' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete inbound' }))
    await waitFor(() => expect(removeMutateAsync).toHaveBeenCalledWith({ nodeID: 7, tag: 'inbound-443' }))
  })

  it('expands clients and opens the QR path', async () => {
    const user = userEvent.setup()
    renderInbounds()

    await user.click(screen.getByRole('button', { name: /Expand row/ }))
    const aliceCard = screen.getByText('alice@example.com').closest('.ant-card')
    expect(aliceCard).toBeTruthy()
    expect(within(aliceCard as HTMLElement).getByText(/vless:\/\//)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Show QR alice@example.com' }))
    expect(screen.getByRole('dialog', { name: 'inbound-443 · alice@example.com' })).toBeInTheDocument()
    expect(screen.getByDisplayValue(/vless:\/\//)).toBeInTheDocument()
  })

  it('filters by search and protocol', async () => {
    const user = userEvent.setup()
    fleet = {
      inbounds: [
        makeFleetRow(),
        makeFleetRow({
          inbound: makeInbound({
            id: 2,
            remark: 'Shadow inbound',
            tag: 'ss-8388',
            port: 8388,
            protocol: 'shadowsocks',
            settings: JSON.stringify({ clients: [] }),
          }),
        }),
      ],
    }
    renderInbounds()

    await user.type(screen.getByLabelText('Search inbounds'), 'shadow')
    expect(screen.getByText('Shadow inbound')).toBeInTheDocument()
    expect(screen.queryByText('Main inbound')).not.toBeInTheDocument()
  })
})
