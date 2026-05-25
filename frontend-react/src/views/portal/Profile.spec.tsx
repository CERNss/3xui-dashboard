import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { App } from 'antd'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { portalProfileApi } from '@/api/portal/profile'
import type { LoginMethodsResponse, UserProfile } from '@/api/portal/profile'
import '@/i18n'
import Profile from './Profile'

const updateProfileMutateAsync = vi.fn()
const startEmailVerificationMutateAsync = vi.fn()
const changeEmailMutateAsync = vi.fn()
const changePasswordMutateAsync = vi.fn()
const startOidcLinkMutateAsync = vi.fn()
const profileRefetch = vi.fn()
const methodsRefetch = vi.fn()

let profile: UserProfile
let methods: LoginMethodsResponse

vi.mock('@/hooks/queries/portal/profile', () => ({
  useProfile: () => ({
    data: profile,
    error: null,
    isFetching: false,
    isLoading: false,
    refetch: profileRefetch,
  }),
  useLoginMethods: () => ({
    data: methods,
    error: null,
    isFetching: false,
    isLoading: false,
    refetch: methodsRefetch,
  }),
  useUpdateProfile: () => ({ isPending: false, mutateAsync: updateProfileMutateAsync }),
  useStartEmailVerification: () => ({ isPending: false, mutateAsync: startEmailVerificationMutateAsync }),
  useChangeEmail: () => ({ isPending: false, mutateAsync: changeEmailMutateAsync }),
  useChangePassword: () => ({ isPending: false, mutateAsync: changePasswordMutateAsync }),
  useStartOidcLink: () => ({ isPending: false, mutateAsync: startOidcLinkMutateAsync }),
}))

vi.mock('@/api/portal/profile', async () => {
  const actual = await vi.importActual<typeof import('@/api/portal/profile')>('@/api/portal/profile')
  return {
    ...actual,
    portalProfileApi: {
      ...actual.portalProfileApi,
      confirmEmailVerification: vi.fn(),
    },
  }
})

const confirmEmailVerificationMock = vi.mocked(portalProfileApi.confirmEmailVerification)

function makeProfile(overrides: Partial<UserProfile> = {}): UserProfile {
  return {
    id: 7,
    email: 'alice@example.com',
    display_name: 'Alice',
    email_verified: true,
    status: 'active',
    balance_cents: 0,
    sub_id: 'sub-7',
    created_at: '2026-05-01T00:00:00Z',
    ...overrides,
  }
}

function makeMethods(overrides: Partial<LoginMethodsResponse> = {}): LoginMethodsResponse {
  return {
    email: { bound: true, email: 'alice@example.com', verified: true },
    oidc_providers: [
      {
        key: 'google',
        name: 'Google',
        icon: 'https://idp.example/google.png',
        linked: true,
        provider_email: 'alice@gmail.example',
      },
      {
        key: 'github',
        name: 'GitHub',
        linked: false,
      },
    ],
    ...overrides,
  }
}

function renderProfile() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <App>
        <Profile />
      </App>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  profile = makeProfile()
  methods = makeMethods()
  updateProfileMutateAsync.mockReset()
  updateProfileMutateAsync.mockResolvedValue(profile)
  startEmailVerificationMutateAsync.mockReset()
  startEmailVerificationMutateAsync.mockResolvedValue({ status: 'ok' })
  changeEmailMutateAsync.mockReset()
  changeEmailMutateAsync.mockResolvedValue(profile)
  confirmEmailVerificationMock.mockReset()
  confirmEmailVerificationMock.mockResolvedValue({ status: 'ok', verification_token: 'verify-token' })
  changePasswordMutateAsync.mockReset()
  changePasswordMutateAsync.mockResolvedValue({ status: 'ok' })
  startOidcLinkMutateAsync.mockReset()
  startOidcLinkMutateAsync.mockResolvedValue({ authorize_url: 'https://idp.example/link' })
  profileRefetch.mockReset()
  methodsRefetch.mockReset()
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, assign: vi.fn() },
  })
})

describe('Portal Profile', () => {
  it('saves display name as metadata only', async () => {
    const user = userEvent.setup()
    renderProfile()

    const input = screen.getByLabelText('Display name')
    await user.clear(input)
    await user.type(input, 'Alice Cooper')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(updateProfileMutateAsync).toHaveBeenCalledWith({ display_name: 'Alice Cooper' }))
    expect(updateProfileMutateAsync).toHaveBeenCalledTimes(1)
  })

  it('sends a change_email verification code and submits verified email change', async () => {
    const user = userEvent.setup()
    renderProfile()

    const emailInput = screen.getByLabelText('Email')
    await user.clear(emailInput)
    await user.type(emailInput, 'new@example.com')
    await user.click(screen.getByRole('button', { name: 'Send verification code' }))

    await waitFor(() =>
      expect(startEmailVerificationMutateAsync).toHaveBeenCalledWith({
        email: 'new@example.com',
        purpose: 'change_email',
      }),
    )

    await user.type(screen.getByLabelText('Email verification code'), '123456')
    await user.click(screen.getByRole('button', { name: 'Update email' }))

    await waitFor(() =>
      expect(confirmEmailVerificationMock).toHaveBeenCalledWith({
        email: 'new@example.com',
        code: '123456',
        purpose: 'change_email',
      }),
    )
    await waitFor(() =>
      expect(changeEmailMutateAsync).toHaveBeenCalledWith({
        email: 'new@example.com',
        verificationToken: 'verify-token',
      }),
    )
  })

  it('blocks short and mismatched password changes, then clears fields after success', async () => {
    const user = userEvent.setup()
    renderProfile()

    await user.type(screen.getByLabelText('Current password'), 'old-password')
    await user.type(screen.getByLabelText('New password'), 'short')
    await user.type(screen.getByLabelText('Confirm password'), 'shorter')
    await user.click(screen.getByRole('button', { name: 'Update password' }))

    expect(await screen.findByText('New password must be at least 8 characters')).toBeInTheDocument()
    expect(changePasswordMutateAsync).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText('New password'))
    await user.type(screen.getByLabelText('New password'), 'new-password')
    await user.click(screen.getByRole('button', { name: 'Update password' }))
    expect(await screen.findByText('New passwords do not match')).toBeInTheDocument()
    expect(changePasswordMutateAsync).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText('Confirm password'))
    await user.type(screen.getByLabelText('Confirm password'), 'new-password')
    await user.click(screen.getByRole('button', { name: 'Update password' }))

    await waitFor(() =>
      expect(changePasswordMutateAsync).toHaveBeenCalledWith({
        oldPassword: 'old-password',
        newPassword: 'new-password',
      }),
    )
    await waitFor(() => expect(screen.getByLabelText('Current password')).toHaveValue(''))
    expect(screen.getByLabelText('New password')).toHaveValue('')
    expect(screen.getByLabelText('Confirm password')).toHaveValue('')
  })

  it('renders multi-provider OIDC link status without unlink actions', async () => {
    const user = userEvent.setup()
    renderProfile()

    const googleRow = screen.getByText('Google').closest('.ant-list-item') as HTMLElement
    const githubRow = screen.getByText('GitHub').closest('.ant-list-item') as HTMLElement

    expect(within(googleRow).getAllByText('Linked').length).toBeGreaterThan(0)
    expect(within(githubRow).getByRole('button', { name: /connect/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /unlink/i })).not.toBeInTheDocument()

    await user.click(within(githubRow).getByRole('button', { name: /connect/i }))
    await waitFor(() =>
      expect(startOidcLinkMutateAsync).toHaveBeenCalledWith({
        providerKey: 'github',
        redirectAfter: '/portal/profile?linked=oidc',
      }),
    )
    expect(window.location.assign).toHaveBeenCalledWith('https://idp.example/link')
  })
})
