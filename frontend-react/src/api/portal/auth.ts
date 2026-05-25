import { portalClient } from '../client/portal'
import type { EmailVerificationPurpose } from './profile'

export interface UserTokenResponse {
  token: string
  expires_at: number
  user_id: number
  email: string
  redirect_after?: string
  next?: string
}

export interface OIDCPendingResponse {
  status: 'pending'
  pending_token: string
  provider: {
    key: string
    name: string
    icon?: string | null
  }
  provider_email: string
  provider_email_verified: boolean
  existing_user: boolean
  existing_user_id?: number
  email?: string
  email_verified?: boolean
  existing_has_oidc?: boolean
  expires_at: number
  redirect_after?: string
  next?: string
}

export type OIDCCallbackResponse = UserTokenResponse | OIDCPendingResponse
export interface OIDCBindExistingInput {
  pendingToken: string
  password: string
}

export interface OIDCCreateAccountInput {
  pendingToken: string
  displayName: string
  email: string
  password: string
  verificationToken: string
}

/** OIDC provider descriptor returned by /auth/oidc/providers. */
export interface OIDCProvider {
  key?: string       // stable provider key for multi-provider installs
  name: string       // "集换社" — human-readable display name
  icon?: string      // optional URL or SVG path; frontend falls back to a generic globe icon
  start_url?: string // relative API endpoint that starts this provider's OIDC flow
  login_url?: string // absolute authorize URL, when backend chooses to precompute it
}

export interface RegistrationPolicy {
  email_verification_required: boolean
}

export interface EmailVerificationStartResponse {
  status: string
  expires_at?: string
  resend_at?: string
  cooldown_seconds?: number
  resend_after_seconds?: number
}

export interface EmailVerificationConfirmResponse {
  status: string
  verification_token: string
  expires_at?: string
}

export const portalAuthApi = {
  login: (email: string, password: string) =>
    portalClient.post<UserTokenResponse>('/auth/login', { email, password }).then((r) => r.data),

  /** Register a portal account. `code` is the 6-digit verification code
   *  delivered via sendCode; required when the backend has SMTP enabled. */
  register: (email: string, password: string, code?: string) =>
    portalClient
      .post<UserTokenResponse>('/auth/register', { email, password, code })
      .then((r) => r.data),

  /** Dispatch a fresh 6-digit code to `email` for the register flow.
   *  Rate-limited 60s per email. Returns 204 on success. */
  sendCode: (email: string) =>
    portalClient.post<void>('/auth/send-code', { email }).then((r) => r.data),

  startEmailVerification: (input: { email: string; purpose: EmailVerificationPurpose }) =>
    portalClient
      .post<EmailVerificationStartResponse>('/auth/email-verification/start', input)
      .then((r) => r.data),

  registrationPolicy: () =>
    portalClient
      .get<RegistrationPolicy>('/auth/registration-policy')
      .then((r) => r.data),

  /** List configured OIDC providers. Empty array when OIDC is not set up
   *  in the backend config — login UI hides the "或使用 X 登录" section. */
  oidcProviders: () =>
    portalClient.get<OIDCProvider[]>('/auth/oidc/providers').then((r) => r.data ?? []),

  /** Start the OIDC dance. Returns the IDP's authorize URL; the
   *  caller is expected to navigate there via window.location.href. */
  confirmEmailVerification: (input: { email: string; code: string; purpose: EmailVerificationPurpose }) =>
    portalClient
      .post<EmailVerificationConfirmResponse>('/auth/email-verification/confirm', input)
      .then((r) => r.data),

  oidcStart: (redirectAfter?: string, providerKey?: string) =>
    portalClient
      .post<{ authorize_url: string }>('/auth/oidc/start', {
        redirect_after: redirectAfter ?? '',
        provider_key: providerKey ?? '',
      })
      .then((r) => r.data),

  /** Exchange the IDP-returned code + state for a portal JWT. */
  oidcCallback: (code: string, state: string) =>
    portalClient
      .post<OIDCCallbackResponse>('/auth/oidc/callback', { code, state })
      .then((r) => r.data),

  /** Finish a pending OIDC account decision. */
  oidcBindExisting: ({ pendingToken, password }: OIDCBindExistingInput) =>
    portalClient
      .post<UserTokenResponse>('/auth/oidc/bind-existing', {
        pending_token: pendingToken,
        password,
      })
      .then((r) => r.data),

  oidcCreateAccount: ({ pendingToken, displayName, email, password, verificationToken }: OIDCCreateAccountInput) =>
    portalClient
      .post<UserTokenResponse>('/auth/oidc/create-account', {
        pending_token: pendingToken,
        display_name: displayName,
        email,
        password,
        verification_token: verificationToken,
      })
      .then((r) => r.data),
}
