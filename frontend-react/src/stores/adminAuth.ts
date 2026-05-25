import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import {
  ADMIN_AUTH_STORAGE_KEY,
  LEGACY_ADMIN_TOKEN_KEY,
  readPersistedToken,
  removeString,
  writeString
} from './storage'

interface AdminAuthState {
  token: string | null
  username: string | null
  isAuthenticated: boolean
  setSession: (token: string, username: string) => void
  clear: () => void
}

const initialToken = readPersistedToken(ADMIN_AUTH_STORAGE_KEY, LEGACY_ADMIN_TOKEN_KEY)

export const useAdminAuthStore = create<AdminAuthState>()(
  persist(
    (set) => ({
      token: initialToken,
      username: null,
      isAuthenticated: initialToken !== null,
      setSession: (token, username) => {
        writeString(LEGACY_ADMIN_TOKEN_KEY, token)
        set({ token, username, isAuthenticated: true })
      },
      clear: () => {
        removeString(LEGACY_ADMIN_TOKEN_KEY)
        set({ token: null, username: null, isAuthenticated: false })
      }
    }),
    {
      name: ADMIN_AUTH_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: ({ token, username }) => ({ token, username }),
      onRehydrateStorage: () => (state) => {
        if (!state?.token) return
        state.isAuthenticated = true
        writeString(LEGACY_ADMIN_TOKEN_KEY, state.token)
      }
    }
  )
)

export type { AdminAuthState }
