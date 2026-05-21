import { portalClient } from '../client/portal'

export interface UserProfile {
  id: number
  email?: string | null
  email_verified: boolean
  status: 'active' | 'suspended'
  balance_cents: number
  sub_id: string
  created_at: string
}

export const portalProfileApi = {
  get: () => portalClient.get<UserProfile>('/profile').then((r) => r.data),
  changePassword: (oldPassword: string, newPassword: string) =>
    portalClient.post<{ status: string }>('/change-password', {
      old_password: oldPassword,
      new_password: newPassword,
    }),
  bindEmail: (email: string) =>
    portalClient.post<{ status: string }>('/bind-email', { email }),
  rotateSubID: () =>
    portalClient.post<{ sub_id: string }>('/rotate-sub-id').then((r) => r.data),
}
