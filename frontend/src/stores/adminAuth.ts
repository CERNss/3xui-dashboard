import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import { ADMIN_AUTH_STORAGE_KEY, readPersistedField } from './storage'

interface AdminAuthState {
  username: string | null
  isAuthenticated: boolean
  setSession: (username: string) => void
  clear: () => void
}

// The JWT lives in an httpOnly cookie the SPA can't read, so "are we
// logged in?" is optimistic: a persisted identity means "probably yes",
// and the first 401 (expired/missing cookie) bounces us to /login via
// the client's onUnauthorized hook.
const initialUsername = readPersistedField<string>(ADMIN_AUTH_STORAGE_KEY, 'username')

export const useAdminAuthStore = create<AdminAuthState>()(
  persist(
    (set) => ({
      username: initialUsername,
      isAuthenticated: initialUsername !== null,
      setSession: (username) => set({ username, isAuthenticated: true }),
      clear: () => set({ username: null, isAuthenticated: false })
    }),
    {
      name: ADMIN_AUTH_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: ({ username }) => ({ username }),
      onRehydrateStorage: () => (state) => {
        if (state) state.isAuthenticated = state.username !== null
      }
    }
  )
)

export type { AdminAuthState }
