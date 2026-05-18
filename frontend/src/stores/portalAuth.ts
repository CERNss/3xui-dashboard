import { defineStore } from 'pinia'

import { PORTAL_TOKEN_KEY } from '@/api/client/portal'

interface UserProfile {
  id: number
  email: string
}

interface State {
  token: string | null
  user: UserProfile | null
}

export const usePortalAuthStore = defineStore('portalAuth', {
  state: (): State => ({
    token: localStorage.getItem(PORTAL_TOKEN_KEY),
    user: null,
  }),
  getters: {
    isAuthenticated: (state) => state.token !== null,
  },
  actions: {
    setSession(token: string, user: UserProfile) {
      this.token = token
      this.user = user
      localStorage.setItem(PORTAL_TOKEN_KEY, token)
    },
    clear() {
      this.token = null
      this.user = null
      localStorage.removeItem(PORTAL_TOKEN_KEY)
    },
  },
})
