import { createApiClient } from './factory'

export const PORTAL_TOKEN_KEY = 'dashboard.portal.token'
export const PORTAL_AUTH_STORAGE_KEY = '3xui.portalAuth'

export const portalClient = createApiClient({
  baseURL: '/api/user',
  tokenStorageKey: PORTAL_TOKEN_KEY,
  persistedStorageKey: PORTAL_AUTH_STORAGE_KEY,
  loginPath: '/login',
})
