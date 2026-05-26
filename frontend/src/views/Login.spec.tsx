import { screen, waitFor, within } from '@testing-library/react'
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

const portalState = vi.hoisted(() => ({
  token: null as string | null,
  user: null as { id: number; email: string } | null,
  isAuthenticated: false,
  setSession(token: string, user: { id: number; email: string }) {
    portalState.token = token
    portalState.user = user
    portalState.isAuthenticated = true
  },
}))

vi.mock('@/api/admin/auth', () => ({
  adminAuthApi: {
    login: vi.fn(),
  },
}))

vi.mock('@/api/portal/auth', () => ({
  portalAuthApi: {
    login: vi.fn(),
    register: vi.fn(),
    registrationPolicy: vi.fn(),
    startEmailVerification: vi.fn(),
    oidcProviders: vi.fn(),
    oidcStart: vi.fn(),
  },
}))

vi.mock('@/stores/adminAuth', () => ({
  useAdminAuthStore: (selector: (state: typeof adminState) => unknown) => selector(adminState),
}))

vi.mock('@/stores/portalAuth', () => ({
  usePortalAuthStore: (selector: (state: typeof portalState) => unknown) => selector(portalState),
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

const adminLoginMock = vi.mocked(adminAuthApi.login)
const portalLoginMock = vi.mocked(portalAuthApi.login)
const registerMock = vi.mocked(portalAuthApi.register)
const registrationPolicyMock = vi.mocked(portalAuthApi.registrationPolicy)
const startEmailVerificationMock = vi.mocked(portalAuthApi.startEmailVerification)
const providersMock = vi.mocked(portalAuthApi.oidcProviders)
const oidcStartMock = vi.mocked(portalAuthApi.oidcStart)

function authFailure(status = 401) {
  return { status, message: 'invalid credentials' }
}

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

function currentPanel() {
  return screen.getByRole('tabpanel')
}

beforeEach(() => {
  adminState.token = null
  adminState.username = null
  adminState.isAuthenticated = false
  portalState.token = null
  portalState.user = null
  portalState.isAuthenticated = false
  useAppStore.getState().setLocale('en-US')
  void i18n.changeLanguage('en')
  adminLoginMock.mockReset()
  portalLoginMock.mockReset()
  registerMock.mockReset()
  registrationPolicyMock.mockReset()
  registrationPolicyMock.mockResolvedValue({ email_verification_required: false })
  startEmailVerificationMock.mockReset()
  providersMock.mockReset()
  providersMock.mockResolvedValue([])
  oidcStartMock.mockReset()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: vi.fn() },
  })
})

describe('Login', () => {
  it('submits admin credentials and stores the admin session', async () => {
    adminLoginMock.mockResolvedValue({ token: 'admin-jwt', username: 'root', expires_at: 1 })
    renderLogin()

    const panel = currentPanel()
    await userEvent.type(within(panel).getByLabelText('Email'), 'root@example.com')
    await userEvent.type(within(panel).getByLabelText('Password'), 'secret')
    await userEvent.click(within(panel).getByRole('button', { name: /sign in/i }))

    await waitFor(() => expect(adminLoginMock).toHaveBeenCalledWith('root@example.com', 'secret'))
    expect(portalLoginMock).not.toHaveBeenCalled()
    expect(adminState.token).toBe('admin-jwt')
    expect(screen.getByTestId('location')).toHaveTextContent('/admin/status')
  })

  it('falls back to portal login when admin rejects the credentials', async () => {
    adminLoginMock.mockRejectedValue(authFailure())
    portalLoginMock.mockResolvedValue({
      token: 'portal-jwt',
      user_id: 7,
      email: 'user@example.com',
      expires_at: 1,
    })
    renderLogin('/login?next=%2Fportal%2Forders')

    const panel = currentPanel()
    await userEvent.type(within(panel).getByLabelText('Email'), 'user@example.com')
    await userEvent.type(within(panel).getByLabelText('Password'), 'secret')
    await userEvent.click(within(panel).getByRole('button', { name: /sign in/i }))

    await waitFor(() =>
      expect(portalLoginMock).toHaveBeenCalledWith({
        email: 'user@example.com',
        password: 'secret',
      }),
    )
    expect(portalState.token).toBe('portal-jwt')
    expect(portalState.user).toEqual({ id: 7, email: 'user@example.com' })
    expect(screen.getByTestId('location')).toHaveTextContent('/portal/orders')
  })

  it('keeps portal users away from admin next targets', async () => {
    adminLoginMock.mockRejectedValue(authFailure())
    portalLoginMock.mockResolvedValue({
      token: 'portal-jwt',
      user_id: 7,
      email: 'user@example.com',
      expires_at: 1,
    })
    renderLogin('/login?next=%2Fadmin%2Fusers')

    const panel = currentPanel()
    await userEvent.type(within(panel).getByLabelText('Email'), 'user@example.com')
    await userEvent.type(within(panel).getByLabelText('Password'), 'secret')
    await userEvent.click(within(panel).getByRole('button', { name: /sign in/i }))

    expect(await screen.findByTestId('location')).toHaveTextContent('/portal/subscription')
  })

  it('rejects empty submission without calling the backend', async () => {
    renderLogin()

    await userEvent.click(within(currentPanel()).getByRole('button', { name: /sign in/i }))

    expect(await screen.findByText('Email is required')).toBeInTheDocument()
    expect(screen.getByText('Password is required')).toBeInTheDocument()
    expect(adminLoginMock).not.toHaveBeenCalled()
    expect(portalLoginMock).not.toHaveBeenCalled()
  })

  it('rejects non-email login identifiers before calling the backend', async () => {
    renderLogin()

    const panel = currentPanel()
    await userEvent.type(within(panel).getByLabelText('Email'), 'root')
    await userEvent.type(within(panel).getByLabelText('Password'), 'secret')
    await userEvent.click(within(panel).getByRole('button', { name: /sign in/i }))

    expect(await screen.findByText('Enter a valid email')).toBeInTheDocument()
    expect(adminLoginMock).not.toHaveBeenCalled()
    expect(portalLoginMock).not.toHaveBeenCalled()
  })

  it('renders OIDC providers only on the login tab', async () => {
    providersMock.mockResolvedValue([
      { name: 'Acme SSO', login_url: 'https://idp.example/authorize' },
      { name: 'Backup SSO', login_url: 'https://backup.example/authorize' },
    ])
    renderLogin()

    expect(await screen.findByRole('button', { name: 'Acme SSO' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Backup SSO' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('tab', { name: 'Sign up' }))
    expect(screen.queryByRole('button', { name: 'Acme SSO' })).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('tab', { name: 'Sign in' }))
    await userEvent.click(screen.getByRole('button', { name: 'Acme SSO' }))
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/authorize')
  })

  it('uses oidcStart with a portal-safe next when provider metadata has no login URL', async () => {
    providersMock.mockResolvedValue([{ key: 'dynamic', name: 'Dynamic SSO', login_url: '' }])
    oidcStartMock.mockResolvedValue({ authorize_url: 'https://idp.example/start' })
    renderLogin('/login?next=%2Fadmin%2Fusers')

    await userEvent.click(await screen.findByRole('button', { name: 'Dynamic SSO' }))

    await waitFor(() => expect(oidcStartMock).toHaveBeenCalledWith('/portal/subscription', 'dynamic'))
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/start')
  })

  it('switches visible translations when locale changes', async () => {
    await i18n.changeLanguage('en')
    useAppStore.getState().setLocale('en-US')
    renderLoginShell()

    expect(screen.getByRole('tab', { name: 'Sign in' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('switch', { name: /Language/i }))

    expect(await screen.findByRole('tab', { name: '登录' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: '注册' })).toBeInTheDocument()
  })

  it('opens the register tab from the query string and registers without verification', async () => {
    registerMock.mockResolvedValue({
      token: 'portal-jwt',
      user_id: 9,
      email: 'new@example.com',
      expires_at: 1,
    })
    renderLogin('/login?mode=register&next=%2Fportal%2Fplans')

    const panel = currentPanel()
    expect(screen.getByRole('tab', { name: 'Sign up', selected: true })).toBeInTheDocument()
    expect(within(panel).queryByLabelText('Email verification code')).not.toBeInTheDocument()

    await userEvent.type(within(panel).getByLabelText('Email'), 'new@example.com')
    await userEvent.type(within(panel).getByLabelText('Password'), 'strongpass')
    await userEvent.type(within(panel).getByLabelText('Confirm password'), 'strongpass')
    await userEvent.click(within(panel).getByRole('button', { name: /create account/i }))

    await waitFor(() =>
      expect(registerMock).toHaveBeenCalledWith({
        email: 'new@example.com',
        password: 'strongpass',
        code: undefined,
      }),
    )
    expect(portalState.token).toBe('portal-jwt')
    expect(screen.getByTestId('location')).toHaveTextContent('/portal/plans')
  })

  it('sends a register verification code and submits it when required', async () => {
    registrationPolicyMock.mockResolvedValue({ email_verification_required: true })
    startEmailVerificationMock.mockResolvedValue({ status: 'ok', cooldown_seconds: 45 })
    registerMock.mockResolvedValue({
      token: 'portal-jwt',
      user_id: 9,
      email: 'new@example.com',
      expires_at: 1,
    })
    renderLogin('/login?mode=register')

    const panel = currentPanel()
    expect(await within(panel).findByLabelText('Email verification code')).toBeInTheDocument()
    await userEvent.type(within(panel).getByLabelText('Email'), 'new@example.com')
    await userEvent.type(within(panel).getByLabelText('Password'), 'strongpass')
    await userEvent.type(within(panel).getByLabelText('Confirm password'), 'strongpass')
    await userEvent.click(within(panel).getByRole('button', { name: 'Send code' }))

    await waitFor(() =>
      expect(startEmailVerificationMock).toHaveBeenCalledWith({
        email: 'new@example.com',
        purpose: 'register',
      }),
    )
    expect(within(panel).getByRole('button', { name: 'Retry in 45s' })).toBeDisabled()

    await userEvent.type(within(panel).getByLabelText('Email verification code'), '654321')
    await userEvent.click(within(panel).getByRole('button', { name: /create account/i }))

    await waitFor(() =>
      expect(registerMock).toHaveBeenCalledWith({
        email: 'new@example.com',
        password: 'strongpass',
        code: '654321',
      }),
    )
    expect(screen.getByTestId('location')).toHaveTextContent('/portal/subscription')
  })

  it('validates password confirmation on register', async () => {
    renderLogin('/login?mode=register')

    const panel = currentPanel()
    await userEvent.type(within(panel).getByLabelText('Email'), 'new@example.com')
    await userEvent.type(within(panel).getByLabelText('Password'), 'strongpass')
    await userEvent.type(within(panel).getByLabelText('Confirm password'), 'different')
    await userEvent.click(within(panel).getByRole('button', { name: /create account/i }))

    expect(await screen.findByText('Passwords do not match')).toBeInTheDocument()
    expect(registerMock).not.toHaveBeenCalled()
  })
})
