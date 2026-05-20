import { portalClient } from '../client/portal'

export interface UserTokenResponse {
  token: string
  expires_at: number
  user_id: number
  email: string
}

/** OIDC provider descriptor returned by /auth/oidc/providers. */
export interface OIDCProvider {
  name: string       // "集换社" — human-readable display name
  icon?: string      // optional URL or SVG path; frontend falls back to a generic globe icon
  login_url: string  // absolute or relative URL to start the OIDC flow
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

  /** List configured OIDC providers. Empty array when OIDC is not set up
   *  in the backend config — login UI hides the "或使用 X 登录" section. */
  oidcProviders: () =>
    portalClient.get<OIDCProvider[]>('/auth/oidc/providers').then((r) => r.data ?? []),
}
