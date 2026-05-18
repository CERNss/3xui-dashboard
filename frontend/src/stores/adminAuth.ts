import { defineStore } from 'pinia'

import { ADMIN_TOKEN_KEY } from '@/api/client/admin'

interface State {
  token: string | null
  username: string | null
}

export const useAdminAuthStore = defineStore('adminAuth', {
  state: (): State => ({
    token: localStorage.getItem(ADMIN_TOKEN_KEY),
    username: null,
  }),
  getters: {
    isAuthenticated: (state) => state.token !== null,
  },
  actions: {
    setSession(token: string, username: string) {
      this.token = token
      this.username = username
      localStorage.setItem(ADMIN_TOKEN_KEY, token)
    },
    clear() {
      this.token = null
      this.username = null
      localStorage.removeItem(ADMIN_TOKEN_KEY)
    },
  },
})
