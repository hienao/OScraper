import { Logout, X } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { Link, NavLink } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/features/auth/use-auth'
import { BrandMark } from './app-header'
import { navigation, profileNavigation } from './navigation'

const linkClass = ({ isActive }: { isActive: boolean }) => `flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors ${isActive ? 'bg-emerald-50 text-emerald-900 dark:bg-emerald-950/70 dark:text-emerald-200' : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-950 dark:text-neutral-400 dark:hover:bg-neutral-900 dark:hover:text-white'}`

export function AppSidebar({ className = '', mobile = false, onClose, onNavigate }: { className?: string; mobile?: boolean; onClose?: () => void; onNavigate?: () => void }) {
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  return (
    <aside className={`w-60 flex-col border-r border-neutral-200/80 bg-white/88 backdrop-blur-xl dark:border-neutral-800 dark:bg-neutral-950/88 ${className}`} aria-label={t('navigation.aria')} role={mobile ? 'dialog' : undefined} aria-modal={mobile || undefined}>
      <div className="flex min-h-20 items-center justify-between px-4 py-3">
        <Link to="/" className="flex min-w-0 items-center gap-2.5" onClick={onNavigate}><BrandMark /><span className="truncate font-bold tracking-tight">OScraper</span></Link>
        {mobile && <Button variant="ghost" size="icon-md" aria-label={t('navigation.close')} onClick={onClose}><X size={20} /></Button>}
      </div>
      <nav className="flex-1 overflow-y-auto px-3 py-4">
        <p className="mb-2 px-3 text-[11px] font-bold uppercase tracking-[0.16em] text-neutral-400">{t('navigation.primary')}</p>
        <div className="space-y-1">
          {navigation.map((item) => { const Icon = item.icon; return <NavLink key={item.to} to={item.to} end={item.end} className={linkClass} onClick={onNavigate}><Icon size={18} /><span>{t(item.label)}</span></NavLink> })}
        </div>
      </nav>
      <div className="border-t border-neutral-200/80 p-3 dark:border-neutral-800">
        {(() => { const Icon = profileNavigation.icon; return <NavLink to={profileNavigation.to} className={linkClass} onClick={onNavigate}><Icon size={18} /><span>{t(profileNavigation.label)}</span></NavLink> })()}
        <div className="mt-2 rounded-xl bg-neutral-50 px-3 py-3 dark:bg-neutral-900/70">
          <p className="truncate text-sm font-semibold">{user?.username}</p><p className="text-xs text-neutral-500">{t('navigation.administrator')}</p>
        </div>
        {mobile && <Button className="mt-2 w-full justify-start gap-2" variant="ghost" onClick={() => logout.mutate()}><Logout size={17} />{t('navigation.logout')}</Button>}
      </div>
    </aside>
  )
}
