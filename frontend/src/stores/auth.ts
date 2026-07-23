import { defineStore } from 'pinia'
import { api, TOKEN_KEY } from '../api/client'
import type { AuthResponse, PublicUser } from '../types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem(TOKEN_KEY) as string | null,
    user: null as PublicUser | null,
  }),
  getters: {
    isAuthenticated: (state) => !!state.token,
    isAdmin: (state) => state.user?.user_role === 'admin',
  },
  actions: {
    async login(email: string, password: string) {
      this.setSession(await api.login(email, password))
    },
    async register(email: string, password: string) {
      this.setSession(await api.register(email, password))
    },
    async fetchMe() {
      if (!this.token) return
      this.user = await api.me()
    },
    setSession(res: AuthResponse) {
      this.token = res.token
      this.user = res.user
      localStorage.setItem(TOKEN_KEY, res.token)
    },
    logout() {
      this.token = null
      this.user = null
      localStorage.removeItem(TOKEN_KEY)
    },
  },
})
