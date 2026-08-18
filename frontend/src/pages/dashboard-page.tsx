import { Activity, ArrowRight, Plug, ShieldCheck } from '@appica/icons-react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { connectionApi, jobApi } from '@/api/services'
import { Panel } from '@/components/common/panel'

export function DashboardPage() {
  const { t } = useTranslation()
  const connections = useQuery({ queryKey: ['connections'], queryFn: connectionApi.list })
  const jobs = useQuery({ queryKey: ['jobs', 'dashboard'], queryFn: () => jobApi.list('', 1, 1), refetchInterval: 5000 })
  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-8 sm:px-6 lg:px-8">
      <section className="app-panel relative overflow-hidden px-6 py-9 sm:px-10 sm:py-12">
        <div className="app-grid pointer-events-none absolute inset-0 opacity-70" />
        <div className="relative max-w-3xl"><p className="text-xs font-bold uppercase tracking-[0.2em] text-emerald-700 dark:text-emerald-400">{t('dashboard.eyebrow')}</p><h1 className="mt-3 text-3xl font-bold tracking-tight sm:text-5xl">{t('dashboard.title')}</h1><p className="mt-4 max-w-2xl text-neutral-600 dark:text-neutral-300">{t('dashboard.description')}</p><Link to="/connections" className="mt-6 inline-flex items-center gap-2 rounded-xl bg-emerald-700 px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-emerald-900/20 hover:bg-emerald-800">{t('dashboard.getStarted')}<ArrowRight size={17} /></Link></div>
      </section>
      <div className="grid gap-4 md:grid-cols-3">
        <Panel><Plug className="text-emerald-700" size={24} /><p className="mt-4 text-3xl font-bold">{connections.data?.length ?? 0}</p><h2 className="mt-1 font-semibold">{t('dashboard.connections')}</h2><p className="mt-1 text-sm text-neutral-500">{t('dashboard.connectionsDescription')}</p></Panel>
        <Panel><Activity className="text-orange-600" size={24} /><p className="mt-4 text-3xl font-bold">{jobs.data?.total ?? 0}</p><h2 className="mt-1 font-semibold"><Link to="/jobs" className="hover:text-emerald-700">{t('dashboard.jobs')}</Link></h2><p className="mt-1 text-sm text-neutral-500">{t('dashboard.jobsDescription')}</p></Panel>
        <Panel><ShieldCheck className="text-sky-600" size={24} /><p className="mt-4 text-3xl font-bold">AES</p><h2 className="mt-1 font-semibold">{t('dashboard.safety')}</h2><p className="mt-1 text-sm text-neutral-500">{t('dashboard.safetyDescription')}</p></Panel>
      </div>
    </div>
  )
}
