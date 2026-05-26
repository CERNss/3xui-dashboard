import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Node } from '@/api/admin/nodes'
import type { FleetResult } from '@/api/admin/inbounds'
import type { AdminPlan } from '@/api/admin/plans'
import type { AdminStats } from '@/api/admin/stats'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Overview from './Overview'

const nodesRefetch = vi.fn()
const fleetRefetch = vi.fn()
const statsRefetch = vi.fn()
const plansRefetch = vi.fn()

let nodes: Node[] = []
let fleet: FleetResult = { inbounds: [] }
let stats: AdminStats | undefined
let plans: AdminPlan[] = []
let nodesLoading = false
let fleetLoading = false
let statsLoading = false
let plansLoading = false
let nodesError: unknown = null
let fleetError: unknown = null
let statsError: unknown = null
let plansError: unknown = null

vi.mock('@/hooks/queries/admin/nodes', () => ({
  useNodesList: () => ({
    data: nodes,
    error: nodesError,
    isError: Boolean(nodesError),
    isFetching: nodesLoading,
    isLoading: nodesLoading,
    refetch: nodesRefetch,
  }),
}))

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useInboundsFleet: () => ({
    data: fleet,
    error: fleetError,
    isError: Boolean(fleetError),
    isFetching: fleetLoading,
    isLoading: fleetLoading,
    refetch: fleetRefetch,
  }),
}))

vi.mock('@/hooks/queries/admin/stats', () => ({
  useAdminStats: () => ({
    data: stats,
    error: statsError,
    isError: Boolean(statsError),
    isFetching: statsLoading,
    isLoading: statsLoading,
    refetch: statsRefetch,
  }),
}))

vi.mock('@/hooks/queries/admin/plans', () => ({
  usePlansList: () => ({
    data: plans,
    error: plansError,
    isError: Boolean(plansError),
    isFetching: plansLoading,
    isLoading: plansLoading,
    refetch: plansRefetch,
  }),
}))

function makeStats(overrides: Partial<AdminStats> = {}): AdminStats {
  return {
    users: {
      total: 42,
      active: 38,
      suspended: 4,
      month_new: 6,
      prev_month_new: 3,
      total_balance_cents: 50000,
      avg_balance_cents: 1190,
    },
    plans: { total: 3, enabled: 2, disabled: 1 },
    orders: {
      total: 100,
      completed: 90,
      failed: 5,
      refunded: 5,
      revenue_cents: 45000,
      month_count: 12,
      month_revenue_cents: 6000,
    },
    traffic: {
      month_up_bytes: 9.71 * 1024 ** 3,
      month_down_bytes: 106.56 * 1024 ** 3,
      today_up_bytes: 205.51 * 1024 ** 2,
      today_down_bytes: 2.51 * 1024 ** 3,
    },
    top_nodes: [
      { key: 'UK-HY2', up_bytes: 1024 ** 3, down_bytes: 1.68 * 1024 ** 3, bytes: 2.68 * 1024 ** 3 },
    ],
    top_users: [
      {
        key: 'alice@example.com',
        up_bytes: 1024 ** 3,
        down_bytes: 1.56 * 1024 ** 3,
        bytes: 2.56 * 1024 ** 3,
      },
    ],
    audit: { info: 143, warn: 2, err: 1 },
    recent_orders: [
      {
        id: 100,
        user_id: 1,
        user_email: 'alice@example.com',
        plan_id: 1,
        plan_name: 'Pro 30d',
        price_cents: 500,
        status: 'completed',
        created_at: '2026-05-20T00:00:00Z',
      },
    ],
    ...overrides,
  }
}

function renderOverview(defaultTab: 'status' | 'stats' = 'status') {
  return renderWithProviders(<Overview defaultTab={defaultTab} />)
}

beforeEach(() => {
  nodes = [
    {
      id: 1,
      name: 'tokyo-1',
      area: 'apac',
      province: 'tokyo',
      scheme: 'https',
      host: 'tk.example.com',
      port: 443,
      base_path: '/',
      enabled: true,
      status: 'online',
      cpu_pct: 12,
      mem_pct: 35,
      xray_version: '25.0.0',
      uptime_s: 3600,
      last_seen_at: '2026-05-24T09:38:02Z',
      created_at: '',
      updated_at: '',
    },
    {
      id: 2,
      name: 'disabled-1',
      area: 'apac',
      province: 'tokyo',
      scheme: 'https',
      host: 'disabled.example.com',
      port: 443,
      base_path: '/',
      enabled: false,
      status: 'offline',
      cpu_pct: 0,
      mem_pct: 0,
      xray_version: '',
      uptime_s: 0,
      last_seen_at: null,
      created_at: '',
      updated_at: '',
    },
  ]
  fleet = {
    inbounds: [
      {
        node_id: 1,
        node_name: 'tokyo-1',
        inbound: {
          id: 1,
          tag: 'vless-1',
          remark: 'vless-1',
          protocol: 'vless',
          port: 443,
          enable: true,
          up: 0,
          down: 0,
          total: 0,
          allTime: 0,
          expiryTime: 0,
          trafficReset: '',
          clientStats: [
            {
              id: 1,
              inboundId: 1,
              enable: true,
              email: 'alice@example.com',
              up: 0,
              down: 0,
              allTime: 0,
              expiryTime: 0,
              total: 0,
              reset: 0,
            },
          ],
          listen: '',
          settings: '{}',
          streamSettings: '{}',
          sniffing: '{}',
        },
      },
    ],
  }
  stats = makeStats()
  plans = [
    {
      id: 1,
      name: 'Pro 30d',
      duration_days: 30,
      traffic_limit_bytes: 0,
      price_cents: 500,
      enabled: true,
    },
  ]
  nodesLoading = false
  fleetLoading = false
  statsLoading = false
  plansLoading = false
  nodesError = null
  fleetError = null
  statsError = null
  plansError = null
  nodesRefetch.mockReset()
  fleetRefetch.mockReset()
  statsRefetch.mockReset()
  plansRefetch.mockReset()
})

describe('Overview', () => {
  it('defaults /admin/status to the Status tab with KPI strip and node health table', () => {
    renderOverview('status')

    expect(screen.getByRole('heading', { name: 'Status' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Status', selected: true })).toBeInTheDocument()
    expect(screen.getByText('Nodes')).toBeInTheDocument()
    expect(screen.getByText('Inbounds')).toBeInTheDocument()
    expect(screen.getByText('Clients')).toBeInTheDocument()
    expect(screen.getByText('Needs attention')).toBeInTheDocument()
    expect(screen.getByText('Node health')).toBeInTheDocument()
    expect(screen.getByText('tokyo-1')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
  })

  it('defaults /admin/stats to the Stats tab with rankings, audit strip, recent orders, and plans', () => {
    renderOverview('stats')

    expect(screen.getByRole('heading', { name: 'Stats' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Stats', selected: true })).toBeInTheDocument()
    expect(screen.getByText('Month new users')).toBeInTheDocument()
    expect(screen.getByText('+100% vs last month')).toBeInTheDocument()
    expect(screen.getByText('Node traffic ranking')).toBeInTheDocument()
    expect(screen.getByText('UK-HY2')).toBeInTheDocument()
    expect(screen.getByText('User traffic ranking')).toBeInTheDocument()
    expect(screen.getByText('alice@example.com')).toBeInTheDocument()
    expect(screen.getByText('System log')).toBeInTheDocument()
    expect(screen.getByText('Total 146 entries')).toBeInTheDocument()
    expect(screen.getByText('Recent orders')).toBeInTheDocument()
    expect(screen.getByText('alice@example.com -> Pro 30d')).toBeInTheDocument()
    expect(screen.getByText('Pro 30d')).toBeInTheDocument()
  })

  it('omits the month new users delta when current and previous month are both zero', () => {
    stats = makeStats({
      users: {
        total: 0,
        active: 0,
        suspended: 0,
        month_new: 0,
        prev_month_new: 0,
        total_balance_cents: 0,
        avg_balance_cents: 0,
      },
    })

    renderOverview('stats')

    expect(screen.getByText('Month new users')).toBeInTheDocument()
    expect(screen.queryByText(/vs last month/)).not.toBeInTheDocument()
  })

  it('shows empty traffic copy when node and user rankings are empty', () => {
    stats = makeStats({ top_nodes: [], top_users: [] })

    renderOverview('stats')

    expect(screen.getAllByText('No traffic data')).toHaveLength(2)
  })

  it('renders stats loading skeleton cards before aggregate data is available', () => {
    stats = undefined
    statsLoading = true

    renderOverview('stats')

    expect(document.querySelectorAll('.ant-card-loading')).toHaveLength(4)
    expect(screen.queryByText('Month new users')).not.toBeInTheDocument()
  })

  it('shows a stats query error without hiding existing aggregate stats', () => {
    statsError = new Error('stats offline')

    renderOverview('stats')

    expect(screen.getByText('Stats load failed')).toBeInTheDocument()
    expect(screen.getByText('Month new users')).toBeInTheDocument()
  })

  it('keeps inactive panels mounted with display none after first activation', async () => {
    const user = userEvent.setup()
    renderOverview('status')

    await user.click(screen.getByRole('tab', { name: 'Stats' }))
    await user.click(screen.getByRole('tab', { name: 'Status' }))

    const statsPanel = document.querySelector('[aria-label="Stats panel"]')
    expect(statsPanel).toBeInTheDocument()
    expect(statsPanel).not.toBeVisible()
    expect(within(statsPanel as HTMLElement).getByText('Node traffic ranking')).toBeInTheDocument()
  })

  it('refreshes only the active tab queries', async () => {
    const user = userEvent.setup()
    renderOverview('status')

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(nodesRefetch).toHaveBeenCalledTimes(1)
    expect(fleetRefetch).toHaveBeenCalledTimes(1)
    expect(statsRefetch).not.toHaveBeenCalled()
    expect(plansRefetch).not.toHaveBeenCalled()

    await user.click(screen.getByRole('tab', { name: 'Stats' }))
    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(statsRefetch).toHaveBeenCalledTimes(1)
    expect(plansRefetch).toHaveBeenCalledTimes(1)
    expect(nodesRefetch).toHaveBeenCalledTimes(1)
    expect(fleetRefetch).toHaveBeenCalledTimes(1)
  })

  it('shows an invalid payload error instead of rendering incomplete stats', async () => {
    stats = {
      users: makeStats().users,
      plans: makeStats().plans,
      orders: makeStats().orders,
      recent_orders: [],
    } as unknown as AdminStats

    renderOverview('stats')

    expect(await screen.findByText('Stats payload is incomplete')).toBeInTheDocument()
    expect(screen.queryByText('Month new users')).not.toBeInTheDocument()
  })

  it('renders core stats when the plans side fetch fails', async () => {
    plansError = new Error('plans offline')
    plans = []

    renderOverview('stats')

    expect(screen.getByText('Month new users')).toBeInTheDocument()
    expect(screen.getByText('Node traffic ranking')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('Plans load failed')).toBeInTheDocument())
  })
})
