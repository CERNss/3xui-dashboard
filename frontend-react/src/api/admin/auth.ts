import { adminClient } from '../client/admin'

export interface AdminLoginResponse {
  token: string
  expires_at: number
  username: string
}

export const adminAuthApi = {
  login: (username: string, password: string) =>
    adminClient
      .post<AdminLoginResponse>('/auth/login', { username, password })
      .then((r) => r.data),
}
