import { create } from 'zustand'
import type { User } from '@/api/types'
import { readAccessToken, removeAccessToken, writeAccessToken } from '@/lib/auth-token'

interface AuthState {
  user: User | null
  accessToken: string | null
  ready: boolean
  setUser: (user: User | null) => void
  setAccessToken: (token: string) => void
  setReady: (ready: boolean) => void
  clearSession: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  accessToken: readAccessToken(),
  ready: false,
  setUser: (user) => set({ user }),
  setAccessToken: (token) => { writeAccessToken(token); set({ accessToken: token }) },
  setReady: (ready) => set({ ready }),
  clearSession: () => { removeAccessToken(); set({ user: null, accessToken: null }) },
}))
