import { createApiClient } from './factory'

export const PORTAL_TOKEN_KEY = 'dashboard.portal.token'

export const portalClient = createApiClient({
  baseURL: '/api/user',
  tokenStorageKey: PORTAL_TOKEN_KEY,
  loginPath: '/login',
})
