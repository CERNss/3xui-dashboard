import { useAdminAuthStore } from '@/stores/adminAuth'
import { createApiClient } from './factory'

export const adminClient = createApiClient({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '/api/admin',
  loginPath: '/login',
  onUnauthorized: () => useAdminAuthStore.getState().clear(),
})
