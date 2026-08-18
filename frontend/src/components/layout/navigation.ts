import { Activity, FileText, Home, Movie, Plug, Settings, User } from '@appica/icons-react'

export const navigation = [
  { to: '/', label: 'navigation.overview', icon: Home, end: true },
  { to: '/connections', label: 'navigation.connections', icon: Plug, end: true },
  { to: '/targets', label: 'navigation.targets', icon: Movie, end: true },
  { to: '/jobs', label: 'navigation.jobs', icon: Activity, end: true },
  { to: '/settings', label: 'navigation.settings', icon: Settings, end: true },
  { to: '/logs', label: 'navigation.logs', icon: FileText, end: true },
]

export const profileNavigation = { to: '/profile', label: 'navigation.profile', icon: User, end: true }

export function titleKey(pathname: string) {
  if (pathname === '/connections') return 'navigation.connections'
  if (pathname === '/targets') return 'navigation.targets'
  if (pathname === '/jobs') return 'navigation.jobs'
  if (pathname === '/settings') return 'navigation.settings'
  if (pathname === '/logs') return 'navigation.logs'
  if (pathname === '/profile') return 'navigation.profile'
  return 'navigation.overview'
}
