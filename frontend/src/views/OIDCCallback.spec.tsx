import { cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { portalAuthApi } from '@/api/portal/auth'
import '@/i18n'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import OIDCCallback, { classifyOidcError } from './OIDCCallback'

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

vi.mock('@/api/portal/auth', () => ({
  portalAuthApi: {
    oidcCallback: vi.fn(),
    oidcBindExisting: vi.fn(),
    oidcCreateAccount: vi.fn(),
    oidcStart: vi.fn(),
    startEmailVerification: vi.fn(),
    confirmEmailVerification: vi.fn(),
  },
}))

vi.mock('@/stores/portalAuth', () => ({
  usePortalAuthStore: (selector: (state: typeof portalState) => unknown) => selector(portalState),
}))

const callbackMock = vi.mocked(portalAuthApi.oidcCallback)
const bindExistingMock = vi.mocked(portalAuthApi.oidcBindExisting)
const createAccountMock = vi.mocked(portalAuthApi.oidcCreateAccount)
const startMock = vi.mocked(portalAuthApi.oidcStart)
const startEmailVerificationMock = vi.mocked(portalAuthApi.startEmailVerification)
const confirmEmailVerificationMock = vi.mocked(portalAuthApi.confirmEmailVerification)

function LocationProbe() {
  const location = useLocation()
  return <span data-testid="location">{location.pathname + location.search}</span>
}

function renderCallback(initialEntry = '/oidc/callback?code=abc&state=xyz') {
  return renderWithProviders(
    <Routes>
      <Route path="/oidc/callback" element={<OIDCCallback />} />
      <Route path="*" element={<LocationProbe />} />
    </Routes>,
    { initialEntries: [initialEntry] },
  )
}

beforeEach(() => {
  portalState.token = null
  portalState.user = null
  portalState.isAuthenticated = false
  callbackMock.mockReset()
  bindExistingMock.mockReset()
  createAccountMock.mockReset()
  startMock.mockReset()
  startEmailVerificationMock.mockReset()
  confirmEmailVerificationMock.mockReset()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: vi.fn() },
  })
})

describe('OIDCCallback', () => {
  it('exchanges code and state, stores the portal token, and navigates to subscription', async () => {
    callbackMock.mockResolvedValue({ token: 'portal-jwt', user_id: 7, email: 'a@example.com', expires_at: 1 })
    renderCallback()

    await waitFor(() => expect(callbackMock).toHaveBeenCalledWith('abc', 'xyz'))
    expect(portalState.token).toBe('portal-jwt')
    expect(await screen.findByTestId('location')).toHaveTextContent('/portal/subscription')
  })

  it('honors next from the callback URL', async () => {
    callbackMock.mockResolvedValue({ token: 'portal-jwt', user_id: 7, email: 'a@example.com', expires_at: 1 })
    renderCallback('/oidc/callback?code=abc&state=xyz&next=%2Fportal%2Ftraffic')

    expect(await screen.findByTestId('location')).toHaveTextContent('/portal/traffic')
  })

  it('does not call the backend when code or state is missing', async () => {
    renderCallback('/oidc/callback?code=abc')

    expect(await screen.findByText('This callback URL is missing code or state.')).toBeInTheDocument()
    expect(callbackMock).not.toHaveBeenCalled()
  })

  it('renders recoverable state errors with a restart action', async () => {
    callbackMock.mockRejectedValue({ status: 400, data: { error: 'state expired' } })
    startMock.mockResolvedValue({ authorize_url: 'https://idp.example/retry' })
    renderCallback()

    expect(await screen.findByText('This login link expired or failed the state check. Start again.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(startMock).toHaveBeenCalled())
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/retry')
  })

  it('renders email conflict actions without storing a token', async () => {
    callbackMock.mockRejectedValue({ status: 409, data: { error: 'oidc: email already linked to a different account' } })
    renderCallback()

    expect(await screen.findByText('This OIDC email is already linked to a different account.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign in first, then link from Profile' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Use a different OIDC account' })).toBeInTheDocument()
    expect(portalState.token).toBeNull()
  })

  it('binds a pending OIDC identity to an existing account after password check', async () => {
    callbackMock.mockResolvedValue({
      status: 'pending',
      pending_token: 'pending-token',
      provider: { key: 'google', name: 'Google' },
      provider_email: 'alice@example.com',
      provider_email_verified: true,
      existing_user: true,
      expires_at: 1,
    })
    bindExistingMock.mockResolvedValue({ token: 'portal-jwt', user_id: 7, email: 'alice@example.com', expires_at: 2 })
    renderCallback()

    expect(await screen.findByText(/Google returned alice@example.com/)).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Password'), 'existing-password')
    await userEvent.click(screen.getByRole('button', { name: 'Bind existing account' }))

    await waitFor(() =>
      expect(bindExistingMock).toHaveBeenCalledWith({
        pendingToken: 'pending-token',
        password: 'existing-password',
      }),
    )
    expect(portalState.token).toBe('portal-jwt')
    expect(await screen.findByTestId('location')).toHaveTextContent('/portal/subscription')
  })

  it('creates a new account from pending OIDC with verified local email', async () => {
    callbackMock.mockResolvedValue({
      status: 'pending',
      pending_token: 'pending-token',
      provider: { key: 'github', name: 'GitHub' },
      provider_email: 'provider@example.com',
      provider_email_verified: true,
      existing_user: false,
      expires_at: 1,
    })
    startEmailVerificationMock.mockResolvedValue({ status: 'ok' })
    confirmEmailVerificationMock.mockResolvedValue({ status: 'ok', verification_token: 'verify-token' })
    createAccountMock.mockResolvedValue({ token: 'portal-jwt', user_id: 8, email: 'local@example.com', expires_at: 2 })
    renderCallback()

    expect(await screen.findByText(/GitHub returned provider@example.com/)).toBeInTheDocument()

    await userEvent.clear(screen.getByLabelText('Email'))
    await userEvent.type(screen.getByLabelText('Email'), 'local@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Send code' }))

    await waitFor(() =>
      expect(startEmailVerificationMock).toHaveBeenCalledWith({
        email: 'local@example.com',
        purpose: 'oidc_create_account',
      }),
    )

    await userEvent.type(screen.getByLabelText('Display name'), 'Local Alice')
    await userEvent.type(screen.getByLabelText('Password'), 'new-password')
    await userEvent.type(screen.getByLabelText('Confirm password'), 'new-password')
    await userEvent.type(screen.getByLabelText('Email verification code'), '654321')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    await waitFor(() =>
      expect(confirmEmailVerificationMock).toHaveBeenCalledWith({
        email: 'local@example.com',
        code: '654321',
        purpose: 'oidc_create_account',
      }),
    )
    await waitFor(() =>
      expect(createAccountMock).toHaveBeenCalledWith({
        pendingToken: 'pending-token',
        displayName: 'Local Alice',
        email: 'local@example.com',
        password: 'new-password',
        verificationToken: 'verify-token',
      }),
    )
    expect(portalState.token).toBe('portal-jwt')
  })

  it('renders profile recovery for email mismatch', async () => {
    callbackMock.mockRejectedValue({ status: 409, data: { error: 'OIDC email does not match' } })
    renderCallback()

    expect(await screen.findByText('The OIDC email does not match the currently bound account.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to Profile' })).toHaveAttribute('href', '/portal/profile')
  })

  it('renders the rejected domain', async () => {
    callbackMock.mockRejectedValue({ status: 403, data: { error: 'domain example.org not allowed' } })
    renderCallback()

    expect(await screen.findByText('This email domain is not allowed.')).toBeInTheDocument()
    expect(screen.getByText('example.org')).toBeInTheDocument()
  })

  it('does not offer retry for suspended accounts or disabled OIDC', async () => {
    callbackMock.mockRejectedValue({ status: 403, data: { error: 'user suspended' } })
    renderCallback()

    expect(await screen.findByText('This account is suspended.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument()

    cleanup()
    callbackMock.mockRejectedValue({ status: 501, data: { error: 'oidc not configured' } })
    renderCallback()

    expect(await screen.findByText('OIDC login is not configured.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument()
  })

  it('shows unknown error details', async () => {
    callbackMock.mockRejectedValue({ status: 418, data: { error: 'teapot' } })
    renderCallback()

    expect(await screen.findByText('OIDC login failed.')).toBeInTheDocument()
    await userEvent.click(screen.getByText('Details'))
    expect(screen.getByText('{"error":"teapot"}')).toBeInTheDocument()
  })

  it('classifies all documented typed errors', () => {
    expect(classifyOidcError({ status: 409, data: { error: 'oidc: email already linked to a different account' } }).kind).toBe('emailConflict')
    expect(classifyOidcError({ status: 409, data: { error: 'OIDC email does not match' } }).kind).toBe('emailMismatch')
    expect(classifyOidcError({ status: 400, data: { error: 'bad state' } }).kind).toBe('stateInvalid')
    expect(classifyOidcError({ status: 400, data: { error: 'OIDC verified email claim is required' } }).kind).toBe('emailUnverified')
    expect(classifyOidcError({ status: 403, data: { error: 'domain example.org not allowed' } }).kind).toBe('domainNotAllowed')
    expect(classifyOidcError({ status: 403, data: { error: 'user suspended' } }).kind).toBe('accountSuspended')
    expect(classifyOidcError({ status: 501, data: { error: 'missing provider' } }).kind).toBe('notConfigured')
    expect(classifyOidcError({ status: 500, data: { error: 'boom' } }).kind).toBe('unknown')
  })
})
