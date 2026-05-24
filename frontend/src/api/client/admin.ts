import { createApiClient } from './factory'

export const ADMIN_TOKEN_KEY = 'dashboard.admin.token'

export const adminClient = createApiClient({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '/api/admin',
  tokenStorageKey: ADMIN_TOKEN_KEY,
  loginPath: '/login',
})
