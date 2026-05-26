import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { FleetResult } from '@/api/admin/inbounds'
import type { Node, NodeMetricsResult } from '@/api/admin/nodes'
import { nodesApi } from '@/api/admin/nodes'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import OpsMonitor from './OpsMonitor'

let nodes: Node[] = []
let fleet: FleetResult

vi.mock('@/hooks/queries/admin/nodes', () => ({
  useNodesList: () => ({
    data: nodes,
    error: null,
    isLoading: false,
  }),
}))

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useInboundsFleet: () => ({
    data: fleet,
    error: null,
    isLoading: false,
  }),
}))

vi.mock('@/api/admin/nodes', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/nodes')>()
  return {
    ...actual,
    nodesApi: {
      ...actual.nodesApi,
      metrics: vi.fn(),
    },
  }
})

function renderOpsMonitor() {
  return renderWithProviders(<OpsMonitor />)
}

function makeNode(partial: Partial<Node>): Node {
  return {
    id: 1,
    name: 'Node',
    area: 'cn',
    province: 'sh',
    scheme: 'https',
    host: 'node.test',
    port: 443,
    base_path: '/',
    enabled: true,
    status: 'online',
    cpu_pct: 10,
    mem_pct: 20,
    xray_version: '1.8.0',
    uptime_s: 100,
    last_seen_at: '2026-01-01T00:00:00Z',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...partial,
  }
}

function metrics(id: number): NodeMetricsResult {
  return {
    id,
    from: 1,
    to: 2,
    bucket: '10m',
    points: [
      { time: '2026-01-01T00:00:00Z', cpu: 10 + id, mem: 20 + id },
      { time: '2026-01-01T00:10:00Z', cpu: 20 + id, mem: 30 + id },
    ],
  }
}

beforeEach(() => {
  nodes = [
    makeNode({ id: 1, name: 'Alpha', status: 'online', cpu_pct: 20, mem_pct: 30 }),
    makeNode({ id: 2, name: 'Beta', status: 'offline', cpu_pct: 90, mem_pct: 60 }),
    makeNode({ id: 3, name: 'Gamma', enabled: false, status: 'unknown', cpu_pct: 0, mem_pct: 0 }),
  ]
  fleet = {
    inbounds: [
      {
        node_id: 1,
        node_name: 'Alpha',
        inbound: {
          id: 10,
          up: 0,
          down: 0,
          total: 0,
          allTime: 0,
          remark: 'main',
          enable: true,
          expiryTime: 0,
          trafficReset: '',
          clientStats: [
            { id: 1, inboundId: 10, enable: true, email: 'a@test', up: 0, down: 0, allTime: 0, expiryTime: 0, total: 0, reset: 0 },
            { id: 2, inboundId: 10, enable: false, email: 'b@test', up: 0, down: 0, allTime: 0, expiryTime: 0, total: 0, reset: 0 },
          ],
          listen: '',
          port: 443,
          protocol: 'vless',
          settings: '{}',
          streamSettings: '{}',
          tag: 'main',
          sniffing: '{}',
        },
      },
    ],
    node_errors: { 2: 'timeout' },
  }
  vi.mocked(nodesApi.metrics).mockReset()
  vi.mocked(nodesApi.metrics).mockImplementation((id: number) => {
    if (id === 2) return Promise.reject(new Error('timeout'))
    return Promise.resolve(metrics(id))
  })
})

describe('OpsMonitor', () => {
  it('fans out metrics for enabled nodes and preserves partial failures', async () => {
    renderOpsMonitor()

    await waitFor(() => expect(nodesApi.metrics).toHaveBeenCalledTimes(2))
    expect(nodesApi.metrics).toHaveBeenCalledWith(1, expect.objectContaining({ bucket: '10m' }))
    expect(nodesApi.metrics).toHaveBeenCalledWith(2, expect.objectContaining({ bucket: '10m' }))
    expect(await screen.findByText('1 node metric request failed; healthy series are still shown.')).toBeInTheDocument()
    expect(screen.getByText('Beta')).toBeInTheDocument()
  })

  it('renders KPI cards, resource SVG, and four chart component shapes without a chart library', async () => {
    renderOpsMonitor()

    expect(screen.getAllByText('Needs attention').length).toBeGreaterThan(0)
    expect(screen.getByText('Active inbounds')).toBeInTheDocument()
    expect(screen.getByText('1/1')).toBeInTheDocument()
    expect(screen.getByText('Clients')).toBeInTheDocument()
    expect(screen.getAllByText('1').length).toBeGreaterThan(0)
    expect(await screen.findByRole('img', { name: 'Resource trend' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Fleet health score' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Concurrency queue bars' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Throughput line' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Duration distribution stack' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Error distribution dots' })).toBeInTheDocument()
  })

  it('excludes disabled nodes from metrics fanout', async () => {
    renderOpsMonitor()

    await waitFor(() => expect(nodesApi.metrics).toHaveBeenCalledTimes(2))
    expect(nodesApi.metrics).not.toHaveBeenCalledWith(3, expect.anything())
    expect(screen.getByText('Disabled')).toBeInTheDocument()
  })
})
