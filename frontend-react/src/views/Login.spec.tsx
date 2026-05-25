import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminAuthApi } from '@/api/admin/auth'
import { portalAuthApi } from '@/api/portal/auth'
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

const loginMock = vi.mocked(adminAuthApi.login)
const providersMock = vi.mocked(portalAuthApi.oidcProviders)
const oidcStartMock = vi.mocked(portalAuthApi.oidcStart)

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location">{location.pathname + location.search}</span>
}

function renderLogin(initialEntry = '/login') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="*" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  adminState.token = null
  adminState.username = null
  adminState.isAuthenticated = false
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

  it('uses oidcStart when provider metadata has no login URL', async () => {
    providersMock.mockResolvedValue([{ key: 'dynamic', name: 'Dynamic SSO', login_url: '' }])
    oidcStartMock.mockResolvedValue({ authorize_url: 'https://idp.example/start' })
    renderLogin('/login?next=%2Fadmin%2Fusers')

    await userEvent.click(await screen.findByRole('button', { name: 'Dynamic SSO' }))

    await waitFor(() => expect(oidcStartMock).toHaveBeenCalledWith('/admin/users', 'dynamic'))
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/start')
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
