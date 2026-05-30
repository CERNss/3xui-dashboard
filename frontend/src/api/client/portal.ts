import { usePortalAuthStore } from '@/stores/portalAuth'
import { createApiClient } from './factory'

export const portalClient = createApiClient({
  baseURL: '/api/user',
  loginPath: '/login',
  onUnauthorized: () => usePortalAuthStore.getState().clear(),
})
