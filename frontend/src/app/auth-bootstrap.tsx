import { Spinner } from '@appica/ui-react/spinner'
import { useEffect, type ReactNode } from 'react'
import { authApi } from '@/api/services'
import { useAuthStore } from '@/stores/auth'

export function AuthBootstrap({ children }: { children: ReactNode }) {
  const token = useAuthStore((state) => state.accessToken)
  const ready = useAuthStore((state) => state.ready)
  const setUser = useAuthStore((state) => state.setUser)
  const setReady = useAuthStore((state) => state.setReady)
  const clearSession = useAuthStore((state) => state.clearSession)

  useEffect(() => {
    let active = true
    async function bootstrap() {
      if (!token) { if (active) setReady(true); return }
      try {
        const profile = await authApi.profile()
        if (active) setUser(profile)
      } catch {
        if (active) clearSession()
      } finally {
        if (active) setReady(true)
      }
    }
    void bootstrap()
    return () => { active = false }
  }, [clearSession, setReady, setUser, token])

  if (!ready) return <div className="grid min-h-screen place-items-center"><Spinner className="size-8" /></div>
  return children
}
