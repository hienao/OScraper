import { useMutation } from '@tanstack/react-query'
import { authApi } from '@/api/services'
import { useAuthStore } from '@/stores/auth'

export function useAuth() {
  const user = useAuthStore((state) => state.user)
  const setUser = useAuthStore((state) => state.setUser)
  const setAccessToken = useAuthStore((state) => state.setAccessToken)
  const clearSession = useAuthStore((state) => state.clearSession)

  const login = useMutation({
    mutationFn: authApi.login,
    onSuccess: async (token) => {
      setAccessToken(token.token)
      setUser(await authApi.profile())
    },
  })
  const logout = useMutation({
    mutationFn: authApi.logout,
    onSettled: clearSession,
  })
  const setupAdmin = useMutation({
    mutationFn: authApi.setupAdmin,
    onSuccess: async (token) => {
      setAccessToken(token.token)
      setUser(await authApi.profile())
    },
  })

  return { user, isAdmin: Boolean(user?.is_admin), login, logout, setupAdmin }
}
