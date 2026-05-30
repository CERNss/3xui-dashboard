import { adminClient } from '../client/admin'

// The backend also returns a `token` for Bearer clients; the browser
// ignores it and authenticates via the httpOnly session cookie, so it's
// intentionally absent from this type.
export interface AdminLoginResponse {
  expires_at: number
  username: string
}

export const adminAuthApi = {
  login: (username: string, password: string) =>
    adminClient
      .post<AdminLoginResponse>('/auth/login', { username, password })
      .then((r) => r.data),

  // Clears the session cookie server-side. The caller still clears local
  // identity afterwards (the cookie is httpOnly, so JS can't do it).
  logout: () => adminClient.post('/auth/logout').then(() => undefined),
}
