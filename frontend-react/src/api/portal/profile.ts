import { portalClient } from '../client/portal'

export interface UserProfile {
  id: number
  email?: string | null
  oidc_subject?: string | null
  email_verified: boolean
  status: 'active' | 'suspended'
  balance_cents: number
  sub_id: string
  created_at: string
}

export interface LoginMethodsResponse {
  email: {
    bound: boolean
    email?: string
    verified: boolean
  }
  oidc: {
    enabled: boolean
    bound: boolean
    name?: string
    icon?: string
  }
}

export const portalProfileApi = {
  get: () => portalClient.get<UserProfile>('/profile').then((r) => r.data),
  loginMethods: () =>
    portalClient.get<LoginMethodsResponse>('/login-methods').then((r) => r.data),
  changePassword: (oldPassword: string, newPassword: string) =>
    portalClient
      .post<{ status: string }>('/change-password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
      .then((r) => r.data),
  bindEmail: (email: string) =>
    portalClient.post<{ status: string }>('/bind-email', { email }).then((r) => r.data),
  startOIDCLink: (redirectAfter?: string) =>
    portalClient
      .post<{ authorize_url: string }>('/oidc/link/start', { redirect_after: redirectAfter ?? '' })
      .then((r) => r.data),
  rotateSubID: () =>
    portalClient.post<{ sub_id: string }>('/rotate-sub-id').then((r) => r.data),
}
