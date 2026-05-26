import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import {
  LEGACY_PORTAL_TOKEN_KEY,
  PORTAL_AUTH_STORAGE_KEY,
  readPersistedToken,
  removeString,
  writeString
} from './storage'

export interface UserProfile {
  id: number
  email: string
}

interface PortalAuthState {
  token: string | null
  user: UserProfile | null
  isAuthenticated: boolean
  setSession: (token: string, user: UserProfile) => void
  clear: () => void
}

const initialToken = readPersistedToken(PORTAL_AUTH_STORAGE_KEY, LEGACY_PORTAL_TOKEN_KEY)

export const usePortalAuthStore = create<PortalAuthState>()(
  persist(
    (set) => ({
      token: initialToken,
      user: null,
      isAuthenticated: initialToken !== null,
      setSession: (token, user) => {
        writeString(LEGACY_PORTAL_TOKEN_KEY, token)
        set({ token, user, isAuthenticated: true })
      },
      clear: () => {
        removeString(LEGACY_PORTAL_TOKEN_KEY)
        set({ token: null, user: null, isAuthenticated: false })
      }
    }),
    {
      name: PORTAL_AUTH_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: ({ token, user }) => ({ token, user }),
      onRehydrateStorage: () => (state) => {
        if (!state?.token) return
        state.isAuthenticated = true
        writeString(LEGACY_PORTAL_TOKEN_KEY, state.token)
      }
    }
  )
)

export type { PortalAuthState }
