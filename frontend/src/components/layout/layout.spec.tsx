import { render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { AdminLayout, AuthLayout, PortalLayout } from './index'

const translations: Record<string, string> = {
  'account.openMenu': 'Open account menu',
  'account.profile': 'Profile',
  'account.adminRole': 'Admin',
  'admin.notifications': 'Notifications',
  'admin.topbarWelcome': 'Welcome back. Here is your account overview.',
  'a11y.openNav': 'Open navigation',
  'app.title': '3x-ui Dashboard',
  'language.chinese': 'Chinese',
  'language.english': 'English',
  'language.label': 'Language',
  'brand.centralPanel': 'central panel',
  'brand.footer': 'Footer',
  'brand.slogan': 'Fleet control',
  'nav.admin': 'Admin',
  'nav.audit': 'Audit log',
  'nav.inbounds': 'Inbounds',
  'nav.nodes': 'Nodes',
  'nav.logout': 'Log out',
  'nav.opsMonitor': 'ops',
  'nav.orders': 'Orders',
  'nav.ordersAdmin': 'Orders',
  'nav.plans': 'Plans',
  'nav.plansAdmin': 'Plans',
  'nav.portal': 'Portal',
  'nav.profile': 'Profile',
  'nav.provisioningPools': 'Provisioning pools',
  'nav.settings': 'Admin settings',
  'nav.status': 'Status',
  'nav.subscription': 'My subscription',
  'nav.usage': 'Usage',
  'nav.users': 'Users',
  'nav.webhooks': 'Webhooks',
  'section.nodes': 'Node ops',
  'section.overview': 'Overview',
  'section.system': 'System',
  'section.users': 'Users & billing',
  'theme.toggleDark': 'Switch to dark',
  'theme.toggleLight': 'Switch to light',
}

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    i18n: { changeLanguage: vi.fn() },
    t: (key: string) => translations[key] ?? key,
  }),
}))

vi.mock('@/hooks/queries/branding', () => ({
  useBranding: () => ({
    data: {
      icon_url: '',
      title: 'Test Dashboard',
      subtitle: 'Central panel',
      description: 'Fleet control',
      footer: 'Footer text',
    },
  }),
}))

function mockMinWidth(matches: boolean) {
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (query: string) =>
      ({
        matches,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }) as unknown as MediaQueryList,
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('layout components', () => {
  it('renders AdminLayout with grouped sidebar chrome and outlet content', () => {
    mockMinWidth(true)

    render(
      <MemoryRouter initialEntries={['/admin/users']} future={{ v7_relativeSplatPath: true, v7_startTransition: true }}>
        <Routes>
          <Route path="/admin" element={<AdminLayout />}>
            <Route path="users" element={<div>Users view</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.getByTestId('admin-layout')).toBeInTheDocument()
    expect(screen.getByText('Overview')).toBeInTheDocument()
    expect(screen.getByText('Node ops')).toBeInTheDocument()
    expect(screen.getByText('Users & billing')).toBeInTheDocument()
    expect(screen.getByText('System')).toBeInTheDocument()
    expect(screen.getByText('Users view')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Users' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Users' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Status' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Stats' })).not.toBeInTheDocument()
  })

  it('renders PortalLayout with exactly five bottom nav items on narrow screens', () => {
    mockMinWidth(false)

    render(
      <MemoryRouter initialEntries={['/portal/orders']} future={{ v7_relativeSplatPath: true, v7_startTransition: true }}>
        <Routes>
          <Route path="/portal" element={<PortalLayout />}>
            <Route path="orders" element={<div>Orders view</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )

    const bottomNav = screen.getByTestId('portal-bottom-nav')
    expect(screen.getByTestId('portal-layout')).toBeInTheDocument()
    expect(within(bottomNav).getAllByRole('link')).toHaveLength(5)
    expect(within(bottomNav).getByRole('link', { name: /Orders/i })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByText('Orders view')).toBeInTheDocument()
  })

  it('renders AuthLayout centered card shell with branding and children', () => {
    render(
      <MemoryRouter future={{ v7_relativeSplatPath: true, v7_startTransition: true }}>
        <AuthLayout cardTitle="Sign in">
          <button type="button">Continue</button>
        </AuthLayout>
      </MemoryRouter>,
    )

    expect(screen.getByTestId('auth-layout')).toBeInTheDocument()
    expect(screen.getByText('Test Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Fleet control')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue' })).toBeInTheDocument()
  })
})
