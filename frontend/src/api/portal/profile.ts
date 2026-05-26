import { portalClient } from '../client/portal'

export interface UserProfile {
  id: number
  email?: string | null
  display_name?: string | null
  email_verified: boolean
  status: 'active' | 'suspended'
  balance_cents: number
  sub_id: string
  created_at: string
}

export type EmailVerificationPurpose = 'change_email' | 'oidc_create_account'

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

export interface EmailLoginMethod {
  bound: boolean
  email?: string
  verified: boolean
}

export interface OIDCProviderLink {
  key?: string
  provider_key?: string
  name?: string
  display_name?: string
  icon?: string | null
  icon_url?: string | null
  linked: boolean
  provider_email?: string | null
  provider_email_verified?: boolean
  linked_at?: string | null
}

export interface LoginMethodsResponse {
  email: EmailLoginMethod
  oidc_providers: OIDCProviderLink[]
}

export const portalProfileApi = {
  get: () => portalClient.get<UserProfile>('/profile').then((r) => r.data),
  loginMethods: () =>
    portalClient.get<LoginMethodsResponse>('/login-methods').then((r) => r.data),
  updateProfile: (input: { display_name?: string | null }) =>
    portalClient.patch<UserProfile>('/profile', input).then((r) => r.data),
  startEmailVerification: (input: { email: string; purpose: EmailVerificationPurpose }) =>
    portalClient
      .post<EmailVerificationStartResponse>('/email-verification/start', input)
      .then((r) => r.data),
  confirmEmailVerification: (input: { email: string; code: string; purpose: EmailVerificationPurpose }) =>
    portalClient
      .post<EmailVerificationConfirmResponse>('/email-verification/confirm', input)
      .then((r) => r.data),
  changeEmail: (input: { email: string; verificationToken: string }) =>
    portalClient
      .post<UserProfile>('/change-email', {
        email: input.email,
        verification_token: input.verificationToken,
      })
      .then((r) => r.data),
  changePassword: (oldPassword: string, newPassword: string) =>
    portalClient
      .post<{ status: string }>('/change-password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
      .then((r) => r.data),
  startOIDCLink: (providerKey: string, redirectAfter?: string) =>
    portalClient
      .post<{ authorize_url: string }>('/oidc/link/start', {
        provider_key: providerKey,
        redirect_after: redirectAfter ?? '',
      })
      .then((r) => r.data),
  rotateSubID: () =>
    portalClient.post<{ sub_id: string }>('/rotate-sub-id').then((r) => r.data),
}
