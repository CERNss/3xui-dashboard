import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useLocation } from 'react-router-dom'
import { AppRouter } from './router'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import '@/i18n'

vi.mock('@/hooks/queries/branding', () => ({
  useBranding: () => ({
    data: {
      title: 'Test Dashboard',
      subtitle: 'Central panel',
      description: 'Fleet control',
      footer: 'Footer text',
    },
  }),
}))

vi.mock('@/hooks/queries/admin/stats', () => ({
  useAdminStats: () => ({
    data: {
      users: {
        total_balance_cents: 0,
      },
    },
  }),
}))

vi.mock('@/api/portal/auth', () => ({
  portalAuthApi: {
    oidcCallback: vi.fn(),
    oidcProviders: vi.fn().mockResolvedValue([]),
    registrationPolicy: vi.fn().mockResolvedValue({ email_verification_required: false }),
    oidcResolve: vi.fn(),
    oidcStart: vi.fn(),
  },
}))

vi.mock('./views/admin/AuditLog', () => ({ default: () => <h1>Audit Log</h1> }))
vi.mock('./views/admin/Inbounds', () => ({ default: () => <h1>Inbounds</h1> }))
vi.mock('./views/admin/Nodes', () => ({ default: () => <h1>Nodes</h1> }))
vi.mock('./views/admin/OpsMonitor', () => ({ default: () => <h1>Ops Monitor</h1> }))
vi.mock('./views/admin/Orders', () => ({ default: () => <h1>Orders</h1> }))
vi.mock('./views/admin/Overview', () => ({ default: () => <h1>Admin Status</h1> }))
vi.mock('./views/admin/Plans', () => ({ default: () => <h1>Plans</h1> }))
vi.mock('./views/admin/ProvisioningPools', () => ({ default: () => <h1>Provisioning Pools</h1> }))
vi.mock('./views/admin/Settings', () => ({ default: () => <h1>Settings</h1> }))
vi.mock('./views/admin/Users', () => ({ default: () => <h1>Users</h1> }))
vi.mock('./views/admin/Webhooks', () => ({ default: () => <h1>Webhooks</h1> }))
vi.mock('./views/portal/Orders', () => ({ default: () => <h1>Portal Orders</h1> }))
vi.mock('./views/portal/Plans', () => ({ default: () => <h1>Portal Plans</h1> }))
vi.mock('./views/portal/Profile', () => ({ default: () => <h1>Profile</h1> }))
vi.mock('./views/portal/Subscription', () => ({ default: () => <h1>Subscription</h1> }))
vi.mock('./views/portal/Usage', () => ({ default: () => <h1>Usage</h1> }))

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location">{location.pathname + location.search}</span>
}

function renderRouter(path: string) {
  return renderWithProviders(
    <>
      <AppRouter />
      <LocationProbe />
    </>,
    { initialPath: path },
  )
}

beforeEach(() => {
  useAdminAuthStore.getState().clear()
  usePortalAuthStore.getState().clear()
})

describe('AppRouter', () => {
  it.each([
    ['/admin/status', 'Admin Status'],
    ['/admin/ops-monitor', 'Ops Monitor'],
    ['/admin/nodes', 'Nodes'],
    ['/admin/inbounds', 'Inbounds'],
    ['/admin/users', 'Users'],
    ['/admin/plans', 'Plans'],
    ['/admin/provisioning-pools', 'Provisioning Pools'],
    ['/admin/orders', 'Orders'],
    ['/admin/audit-log', 'Audit Log'],
    ['/admin/webhooks', 'Webhooks'],
    ['/admin/settings', 'Settings'],
  ])('resolves admin route %s', async (path, title) => {
    useAdminAuthStore.getState().setSession('admin-token', 'root')

    renderRouter(path)

    expect(await screen.findAllByRole('heading', { name: title })).not.toHaveLength(0)
    expect(screen.queryByText('Page not found')).not.toBeInTheDocument()
  })

  it('redirects legacy /admin/stats to the system status stats tab', async () => {
    useAdminAuthStore.getState().setSession('admin-token', 'root')

    renderRouter('/admin/stats')

    expect(await screen.findAllByRole('heading', { name: 'Admin Status' })).not.toHaveLength(0)
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/admin/status?tab=stats'))
  })

  it.each([
    ['/portal/subscription', 'Subscription'],
    ['/portal/usage', 'Usage'],
    ['/portal/plans', 'Portal Plans'],
    ['/portal/orders', 'Portal Orders'],
    ['/portal/profile', 'Profile'],
  ])('resolves portal route %s', async (path, title) => {
    usePortalAuthStore.getState().setSession('portal-token', { id: 7, email: 'user@example.com' })

    renderRouter(path)

    expect(await screen.findByRole('heading', { name: title })).toBeInTheDocument()
  })

  it.each([
    ['/', 'admin', '/admin/status'],
    ['/admin', 'admin', '/admin/status'],
    ['/portal', 'portal', '/portal/subscription'],
  ])('redirects default entry %s', async (path, area, expected) => {
    if (area === 'admin') {
      useAdminAuthStore.getState().setSession('admin-token', 'root')
    } else {
      usePortalAuthStore.getState().setSession('portal-token', { id: 7, email: 'user@example.com' })
    }

    renderRouter(path)

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent(expected))
  })

  it('renders NotFound for unknown admin paths without redirecting away', async () => {
    useAdminAuthStore.getState().setSession('admin-token', 'root')

    renderRouter('/admin/this-does-not-exist')

    expect(await screen.findByText('404')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/admin/this-does-not-exist')
  })

  it('guards anonymous admin paths with an encoded next fullpath', async () => {
    renderRouter('/admin/users?filter=active')

    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(
        '/login?next=%2Fadmin%2Fusers%3Ffilter%3Dactive',
      ),
    )
  })
})
