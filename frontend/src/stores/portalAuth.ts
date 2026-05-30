import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import { PORTAL_AUTH_STORAGE_KEY, readPersistedField } from './storage'

export interface UserProfile {
  id: number
  email: string
}

interface PortalAuthState {
  user: UserProfile | null
  isAuthenticated: boolean
  setSession: (user: UserProfile) => void
  clear: () => void
}

// See adminAuth: auth is optimistic because the JWT is in an httpOnly
// cookie the SPA can't read. A persisted identity means "probably
// logged in"; a 401 clears it.
const initialUser = readPersistedField<UserProfile>(PORTAL_AUTH_STORAGE_KEY, 'user')

export const usePortalAuthStore = create<PortalAuthState>()(
  persist(
    (set) => ({
      user: initialUser,
      isAuthenticated: initialUser !== null,
      setSession: (user) => set({ user, isAuthenticated: true }),
      clear: () => set({ user: null, isAuthenticated: false })
    }),
    {
      name: PORTAL_AUTH_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: ({ user }) => ({ user }),
      onRehydrateStorage: () => (state) => {
        if (state) state.isAuthenticated = state.user !== null
      }
    }
  )
)

export type { PortalAuthState }
