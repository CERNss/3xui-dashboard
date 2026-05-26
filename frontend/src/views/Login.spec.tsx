import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminAuthApi } from '@/api/admin/auth'
import { portalAuthApi } from '@/api/portal/auth'
import { i18n } from '@/i18n'
import { AuthLayout } from '@/components/layout'
import { useAppStore } from '@/stores/app'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Login from './Login'

const adminState = vi.hoisted(() => ({
  token: null as string | null,
  username: null as string | null,
  isAuthenticated: false,
  setSession(token: string, username: string) {
    adminState.token = token
    adminState.username = username
    adminState.isAuthenticated = true
  },
}))

vi.mock('@/api/admin/auth', () => ({
  adminAuthApi: {
    login: vi.fn(),
  },
}))

vi.mock('@/api/portal/auth', () => ({
  portalAuthApi: {
    oidcProviders: vi.fn(),
    oidcStart: vi.fn(),
  },
}))

vi.mock('@/stores/adminAuth', () => ({
  useAdminAuthStore: (selector: (state: typeof adminState) => unknown) => selector(adminState),
}))

vi.mock('@/hooks/queries/branding', () => ({
  useBranding: () => ({
    data: {
      icon_url: '',
      title: '3xui Central',
      subtitle: 'Central panel',
      description: 'Fleet control',
      footer: '',
    },
  }),
}))

const loginMock = vi.mocked(adminAuthApi.login)
const providersMock = vi.mocked(portalAuthApi.oidcProviders)
const oidcStartMock = vi.mocked(portalAuthApi.oidcStart)

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location">{location.pathname + location.search}</span>
}

function renderLogin(initialEntry = '/login') {
  return renderWithProviders(
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="*" element={<LocationProbe />} />
    </Routes>,
    { initialEntries: [initialEntry] },
  )
}

function renderLoginShell(initialEntry = '/login') {
  return renderWithProviders(
    <Routes>
      <Route
        path="/login"
        element={
          <AuthLayout>
            <Login />
          </AuthLayout>
        }
      />
    </Routes>,
    { initialEntries: [initialEntry] },
  )
}

beforeEach(() => {
  adminState.token = null
  adminState.username = null
  adminState.isAuthenticated = false
  useAppStore.getState().setLocale('en-US')
  void i18n.changeLanguage('en')
  loginMock.mockReset()
  providersMock.mockReset()
  providersMock.mockResolvedValue([])
  oidcStartMock.mockReset()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: vi.fn() },
  })
})

describe('Login', () => {
  it('submits admin credentials and stores the session', async () => {
    loginMock.mockResolvedValue({ token: 'admin-jwt', username: 'root', expires_at: 1 })
    renderLogin()

    await userEvent.type(screen.getByLabelText('Username'), 'root')
    await userEvent.type(screen.getByLabelText('Password'), 'secret')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => expect(loginMock).toHaveBeenCalledWith('root', 'secret'))
    expect(adminState.token).toBe('admin-jwt')
    expect(screen.getByTestId('location')).toHaveTextContent('/admin/status')
  })

  it('rejects empty submission without calling the backend', async () => {
    renderLogin()

    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    expect(await screen.findByText('Username is required')).toBeInTheDocument()
    expect(screen.getByText('Password is required')).toBeInTheDocument()
    expect(loginMock).not.toHaveBeenCalled()
  })

  it('renders OIDC providers as buttons', async () => {
    providersMock.mockResolvedValue([
      { name: 'Acme SSO', login_url: 'https://idp.example/authorize' },
      { name: 'Backup SSO', login_url: 'https://backup.example/authorize' },
    ])
    renderLogin()

    expect(await screen.findByRole('button', { name: 'Acme SSO' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Backup SSO' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Acme SSO' }))
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/authorize')
  })

  it('switches visible translations when locale changes', async () => {
    await i18n.changeLanguage('en')
    useAppStore.getState().setLocale('en-US')
    renderLoginShell()

    expect(screen.getByRole('heading', { name: 'Welcome back' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('switch', { name: /Language/i }))

    expect(await screen.findByRole('heading', { name: '欢迎回来' })).toBeInTheDocument()
  })

  it('uses oidcStart when provider metadata has no login URL', async () => {
    providersMock.mockResolvedValue([{ key: 'dynamic', name: 'Dynamic SSO', login_url: '' }])
    oidcStartMock.mockResolvedValue({ authorize_url: 'https://idp.example/start' })
    renderLogin('/login?next=%2Fadmin%2Fusers')

    await userEvent.click(await screen.findByRole('button', { name: 'Dynamic SSO' }))

    await waitFor(() => expect(oidcStartMock).toHaveBeenCalledWith('/portal/subscription', 'dynamic'))
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/start')
  })

  it('preserves portal next for OIDC start', async () => {
    providersMock.mockResolvedValue([{ key: 'dynamic', name: 'Dynamic SSO', login_url: '' }])
    oidcStartMock.mockResolvedValue({ authorize_url: 'https://idp.example/start' })
    renderLogin('/login?next=%2Fportal%2Forders%3Ftab%3Dactive')

    await userEvent.click(await screen.findByRole('button', { name: 'Dynamic SSO' }))

    await waitFor(() => expect(oidcStartMock).toHaveBeenCalledWith('/portal/orders?tab=active', 'dynamic'))
  })

  it('honors next after successful login', async () => {
    loginMock.mockResolvedValue({ token: 'admin-jwt', username: 'root', expires_at: 1 })
    renderLogin('/login?next=%2Fadmin%2Fusers')

    await userEvent.type(screen.getByLabelText('Username'), 'root')
    await userEvent.type(screen.getByLabelText('Password'), 'secret')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    expect(await screen.findByTestId('location')).toHaveTextContent('/admin/users')
  })
})
