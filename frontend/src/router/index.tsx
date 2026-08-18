import { createBrowserRouter, Navigate } from 'react-router-dom'
import { lazy } from 'react'
import { AppLayout } from '@/components/layout/app-layout'
import { GuestOnly, RequireAuth } from './guards'

const ConnectionsPage = lazy(() => import('@/pages/connections-page').then((module) => ({ default: module.ConnectionsPage })))
const DashboardPage = lazy(() => import('@/pages/dashboard-page').then((module) => ({ default: module.DashboardPage })))
const LoginPage = lazy(() => import('@/pages/login-page').then((module) => ({ default: module.LoginPage })))
const LogsPage = lazy(() => import('@/pages/logs-page').then((module) => ({ default: module.LogsPage })))
const ProfilePage = lazy(() => import('@/pages/profile-page').then((module) => ({ default: module.ProfilePage })))
const TargetsPage = lazy(() => import('@/pages/targets-page').then((module) => ({ default: module.TargetsPage })))
const SettingsPage = lazy(() => import('@/pages/settings-page').then((module) => ({ default: module.SettingsPage })))

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      { element: <GuestOnly />, children: [{ path: 'login', element: <LoginPage /> }] },
      {
        element: <RequireAuth />,
        children: [
          { index: true, element: <DashboardPage /> },
          { path: 'connections', element: <ConnectionsPage /> },
          { path: 'targets', element: <TargetsPage /> },
          { path: 'settings', element: <SettingsPage /> },
          { path: 'logs', element: <LogsPage /> },
          { path: 'profile', element: <ProfilePage /> },
        ],
      },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])
