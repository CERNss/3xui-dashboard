import { createApiClient } from './factory'

export const ADMIN_TOKEN_KEY = 'dashboard.admin.token'
export const ADMIN_AUTH_STORAGE_KEY = '3xui.adminAuth'

export const adminClient = createApiClient({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '/api/admin',
  tokenStorageKey: ADMIN_TOKEN_KEY,
  persistedStorageKey: ADMIN_AUTH_STORAGE_KEY,
  loginPath: '/login',
})
