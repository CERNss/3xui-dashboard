import { portalClient } from '../client/portal'

export interface UserTokenResponse {
  token: string
  expires_at: number
  user_id: number
  email: string
}

export const portalAuthApi = {
  login: (email: string, password: string) =>
    portalClient.post<UserTokenResponse>('/auth/login', { email, password }).then((r) => r.data),
  register: (email: string, password: string) =>
    portalClient.post<UserTokenResponse>('/auth/register', { email, password }).then((r) => r.data),
}
