import { Logout, Menu, User } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { Link, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/features/auth/use-auth'
import { LanguageSwitcher } from './language-switcher'
import { ThemeToggle } from './theme-toggle'
import { titleKey } from './navigation'

export function AppHeader({ navigationOpen = false, onOpenNavigation }: { navigationOpen?: boolean; onOpenNavigation?: () => void }) {
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  const location = useLocation()
  if (!user) {
    return (
      <header className="sticky top-0 z-40 border-b border-neutral-200/70 bg-white/82 backdrop-blur-xl dark:border-neutral-800 dark:bg-neutral-950/82">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6">
          <Link to="/" className="flex items-center gap-2.5"><BrandMark /><span className="font-bold tracking-tight">OpenlistScraper</span></Link>
          <div className="flex items-center gap-1"><LanguageSwitcher /><ThemeToggle /></div>
        </div>
      </header>
    )
  }
  return (
    <header className="sticky top-0 z-30 border-b border-neutral-200/70 bg-white/82 backdrop-blur-xl dark:border-neutral-800 dark:bg-neutral-950/82">
      <div className="flex h-16 items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-2.5">
          <Button className="lg:hidden" variant="ghost" size="icon-md" aria-label={t('navigation.open')} aria-expanded={navigationOpen} onClick={onOpenNavigation}><Menu size={20} /></Button>
          <p className="truncate text-sm font-semibold sm:text-base">{t(titleKey(location.pathname))}</p>
        </div>
        <div className="flex items-center gap-1.5">
          <LanguageSwitcher /><ThemeToggle />
          <span className="hidden items-center gap-2 rounded-lg bg-neutral-100 px-3 py-2 text-sm dark:bg-neutral-800 md:flex"><User size={16} />{user.username}</span>
          <Button className="hidden gap-2 md:flex" variant="ghost" onClick={() => logout.mutate()} disabled={logout.isPending}><Logout size={17} />{t('navigation.logout')}</Button>
        </div>
      </div>
    </header>
  )
}

export function BrandMark() {
  return <span className="grid size-9 place-items-center rounded-xl bg-emerald-700 font-mono text-sm font-bold text-white shadow-lg shadow-emerald-900/20">OS</span>
}
