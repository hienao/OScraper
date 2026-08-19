import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'

export function RequireAuth() {
  const user = useAuthStore((state) => state.user)
  const location = useLocation()
  return user ? <Outlet /> : <Navigate to="/login" replace state={{ from: location.pathname }} />
}

export function GuestOnly() {
  const user = useAuthStore((state) => state.user)
  return user ? <Navigate to="/" replace /> : <Outlet />
}
