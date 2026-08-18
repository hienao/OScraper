import { Outlet, useLocation } from 'react-router-dom'
import { Suspense, useEffect, useState } from 'react'
import { Spinner } from '@appica/ui-react/spinner'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth'
import { AdminSetupDialog } from '@/components/common/admin-setup-dialog'
import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'

export function AppLayout() {
  const user = useAuthStore((state) => state.user)
  const [navigationOpen, setNavigationOpen] = useState(false)
  const location = useLocation()
  const { t } = useTranslation()
  useEffect(() => setNavigationOpen(false), [location.pathname])

  const content = <Suspense fallback={<div className="grid min-h-[60vh] place-items-center"><Spinner className="size-8" /></div>}><Outlet /></Suspense>
  if (!user) return <div className="flex min-h-screen flex-col"><AppHeader /><main id="main-content" className="flex-1">{content}</main><footer className="border-t border-neutral-200/70 px-4 py-6 text-center text-sm text-neutral-500 dark:border-neutral-800">OpenlistScraper · {t('common.productDescription')}</footer></div>
  return (
    <div className="flex min-h-screen">
      <a href="#main-content" className="sr-only focus:not-sr-only">{t('navigation.skip')}</a>
      <AppSidebar className="sticky top-0 hidden h-screen shrink-0 lg:flex" />
      <div className="flex min-w-0 flex-1 flex-col"><AppHeader navigationOpen={navigationOpen} onOpenNavigation={() => setNavigationOpen(true)} /><main id="main-content" className="relative flex-1">{content}</main></div>
      {navigationOpen && <div className="fixed inset-0 z-50 lg:hidden"><button type="button" className="absolute inset-0 bg-neutral-950/45" aria-label={t('navigation.close')} onClick={() => setNavigationOpen(false)} /><AppSidebar className="relative flex h-full max-w-[88vw] shadow-2xl" mobile onClose={() => setNavigationOpen(false)} onNavigate={() => setNavigationOpen(false)} /></div>}
      <AdminSetupDialog />
    </div>
  )
}
