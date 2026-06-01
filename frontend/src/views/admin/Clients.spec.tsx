import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { FleetResult, Inbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'
import type { ListUsersResponse } from '@/api/admin/users'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Clients from './Clients'

const addClientMutateAsync = vi.fn()
const updateClientMutateAsync = vi.fn()
const removeClientMutateAsync = vi.fn()
const fleetRefetch = vi.fn()

let fleet: FleetResult
let nodes: Node[]
let users: ListUsersResponse

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useInboundsFleet: () => ({
    data: fleet,
    error: null,
    isFetching: false,
    isLoading: false,
    refetch: fleetRefetch,
  }),
  useAddClient: () => ({ isPending: false, mutateAsync: addClientMutateAsync }),
  useUpdateClient: () => ({ isPending: false, mutateAsync: updateClientMutateAsync }),
  useRemoveClient: () => ({ isPending: false, mutateAsync: removeClientMutateAsync }),
}))

vi.mock('@/hooks/queries/admin/nodes', () => ({
  useNodesList: () => ({ data: nodes, error: null, isFetching: false, isLoading: false }),
}))

vi.mock('@/hooks/queries/admin/users', () => ({
  useUsersList: () => ({ data: users, error: null, isFetching: false, isLoading: false }),
}))

function makeInbound(overrides: Partial<Inbound> = {}): Inbound {
  return {
    id: 1,
    up: 0,
    down: 0,
    total: 0,
    allTime: 0,
    remark: 'Retail inbound',
    enable: true,
    expiryTime: 0,
    trafficReset: 'never',
    clientStats: [
      { id: 1, inboundId: 1, enable: true, email: 'alice@example.com', up: 0, down: 0, allTime: 0, expiryTime: 0, total: 0, reset: 0 },
    ],
    listen: '',
    port: 45110,
    protocol: 'vless',
    settings: JSON.stringify({
      clients: [
        {
          id: 'uuid-1',
          email: 'alice@example.com',
          enable: true,
          expiryTime: 0,
          totalGB: 1024 * 1024 * 1024,
        },
      ],
    }),
    streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
    sniffing: JSON.stringify({ enabled: true }),
    tag: 'pool-1-45110',
    ...overrides,
  }
}

function renderClients() {
  return renderWithProviders(<Clients />)
}

beforeEach(() => {
  fleet = {
    inbounds: [
      {
        node_id: 7,
        node_name: 'BWG US Node',
        inbound: makeInbound(),
      },
    ],
  }
  nodes = [
    {
      id: 7,
      name: 'BWG US Node',
      area: 'us',
      province: 'California',
      scheme: 'https',
      host: 'node.example.com',
      port: 10138,
      base_path: '/',
      enabled: true,
      status: 'online',
      cpu_pct: 0,
      mem_pct: 0,
      xray_version: '',
      uptime_s: 0,
      created_at: '',
      updated_at: '',
    },
  ]
  users = { users: [], limit: 200, offset: 0 }
  addClientMutateAsync.mockReset()
  updateClientMutateAsync.mockReset()
  removeClientMutateAsync.mockReset()
  fleetRefetch.mockReset()
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

describe('Clients', () => {
  it('renders 3x-ui totalGB as bytes instead of multiplying it as GiB', () => {
    renderClients()

    expect(screen.getByRole('heading', { name: 'Clients' })).toBeInTheDocument()
    expect(screen.getByText('alice@example.com')).toBeInTheDocument()
    expect(screen.getByText('0 B / 1.00 GiB')).toBeInTheDocument()
    expect(screen.queryByText(/1048576\.00 TiB/)).not.toBeInTheDocument()
  })
})
