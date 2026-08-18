import { Activity, Download, Refresh } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { logApi } from '@/api/services'
import type { APIRequestLog, ApplicationLog, AuditLog, Page } from '@/api/types'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

type LogTab = 'api' | 'application' | 'audit'
type LogEntry = APIRequestLog | ApplicationLog | AuditLog

function csvCell(value: unknown) {
  const text = value == null ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value)
  return `"${text.replaceAll('"', '""')}"`
}

export function LogsPage() {
  const { t } = useTranslation()
  const [tab, setTab] = useState<LogTab>('api')
  const [search, setSearch] = useState('')
  const queryString = new URLSearchParams({ page: '1', size: '100', ...(search.trim() ? { q: search.trim() } : {}) }).toString()
  const logs = useQuery<Page<LogEntry>>({ queryKey: ['logs', tab, search], queryFn: async () => (tab === 'api' ? await logApi.api(queryString) : tab === 'application' ? await logApi.application(queryString) : await logApi.audit(queryString)) as Page<LogEntry> })
  const items = logs.data?.items ?? []
  const exportCSV = () => {
    if (items.length === 0) return
    const columns = Object.keys(items[0]) as (keyof LogEntry)[]
    const csv = [columns.map(csvCell).join(','), ...items.map((item) => columns.map((column) => csvCell(item[column])).join(','))].join('\n')
    const url = URL.createObjectURL(new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `openlist-scraper-${tab}-logs-${new Date().toISOString().slice(0, 10)}.csv`
    anchor.click()
    URL.revokeObjectURL(url)
  }
  return <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8"><Panel title={t('navigation.logs')} description={t('logs.description')} icon={<Activity size={20} />} action={<div className="flex gap-2"><Button variant="outline" size="sm" className="gap-2" disabled={items.length === 0} onClick={exportCSV}><Download size={15} />{t('logs.export')}</Button><Button variant="outline" size="sm" className="gap-2" disabled={logs.isFetching} onClick={() => void logs.refetch()}><Refresh size={15} className={logs.isFetching ? 'animate-spin' : ''} />{t('common.refresh')}</Button></div>}>
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div className="flex rounded-xl bg-neutral-100 p-1 dark:bg-neutral-900">{(['api', 'application', 'audit'] as LogTab[]).map((value) => <button key={value} type="button" onClick={() => setTab(value)} className={`rounded-lg px-3 py-2 text-sm font-medium ${tab === value ? 'bg-white shadow-sm dark:bg-neutral-800' : 'text-neutral-500'}`}>{t(`logs.tabs.${value}`)}</button>)}</div><Input className="sm:max-w-sm" value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('logs.search')} /></div>
    {logs.error && <div className="mt-4"><Message variant="error">{errorMessage(logs.error, t('logs.loadError'))}</Message></div>}
    {logs.isLoading && <p className="py-12 text-center text-sm text-neutral-500">{t('common.loading')}</p>}
    {!logs.isLoading && items.length === 0 && <p className="py-12 text-center text-sm text-neutral-500">{t('logs.empty')}</p>}
    <div className="mt-5 overflow-x-auto"><table className="w-full min-w-[760px] text-left text-sm"><thead><tr className="border-b border-neutral-200 text-xs uppercase tracking-wide text-neutral-500 dark:border-neutral-800"><th className="px-3 py-3">{t('logs.time')}</th><th className="px-3 py-3">{t('logs.type')}</th><th className="px-3 py-3">{t('logs.summary')}</th><th className="px-3 py-3">{t('logs.context')}</th></tr></thead><tbody>
      {tab === 'api' && (items as APIRequestLog[]).map((item) => <tr key={item.id} className="border-b border-neutral-100 align-top dark:border-neutral-900"><td className="whitespace-nowrap px-3 py-3 text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</td><td className="px-3 py-3"><Badge variant="outline">{item.method}</Badge><span className={`ml-2 text-xs font-semibold ${item.status_code >= 400 ? 'text-red-600' : 'text-emerald-600'}`}>{item.status_code}</span></td><td className="px-3 py-3"><p className="font-mono text-xs">{item.route}</p>{item.error_code && <p className="mt-1 text-xs text-red-600">{item.error_code}</p>}</td><td className="px-3 py-3 text-xs text-neutral-500">{item.latency_ms} ms{item.job_id ? ` · Job #${item.job_id}` : ''}{item.target_id ? ` · Target #${item.target_id}` : ''}</td></tr>)}
      {tab === 'application' && (items as ApplicationLog[]).map((item) => <tr key={item.id} className="border-b border-neutral-100 align-top dark:border-neutral-900"><td className="whitespace-nowrap px-3 py-3 text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</td><td className="px-3 py-3"><Badge variant="outline">{item.level}</Badge><p className="mt-1 text-xs text-neutral-500">{item.source}</p></td><td className="px-3 py-3"><p>{item.message}</p>{item.fields && <details className="mt-1"><summary className="cursor-pointer text-xs text-neutral-500">{t('logs.fields')}</summary><pre className="mt-2 max-w-2xl whitespace-pre-wrap break-all text-xs">{item.fields}</pre></details>}</td><td className="px-3 py-3 text-xs text-neutral-500">{item.job_id ? `Job #${item.job_id}` : '—'}{item.target_id ? ` · Target #${item.target_id}` : ''}</td></tr>)}
      {tab === 'audit' && (items as AuditLog[]).map((item) => <tr key={item.id} className="border-b border-neutral-100 align-top dark:border-neutral-900"><td className="whitespace-nowrap px-3 py-3 text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</td><td className="px-3 py-3"><Badge variant="outline">{item.action}</Badge></td><td className="px-3 py-3"><p>{item.target}</p><details className="mt-1"><summary className="cursor-pointer text-xs text-neutral-500">{t('logs.fields')}</summary><pre className="mt-2 max-w-2xl whitespace-pre-wrap break-all text-xs">{item.detail}</pre></details></td><td className="px-3 py-3 text-xs text-neutral-500">Actor #{item.actor_id}</td></tr>)}
    </tbody></table></div>
    {logs.data && <p className="mt-4 text-xs text-neutral-500">{t('logs.total', { count: logs.data.total })}</p>}
  </Panel></div>
}
