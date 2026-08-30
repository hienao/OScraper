import { Activity, Check, Download, Refresh, Settings, Trash } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@appica/ui-react/table'
import { Tabs, TabsList, TabsTrigger } from '@appica/ui-react/tabs'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { logApi } from '@/api/services'
import type { APIRequestLog, ApplicationLog, AuditLog, LogSettings, LogType, Page } from '@/api/types'
import { AppDialog } from '@/components/common/app-dialog'
import { AppSelect } from '@/components/common/app-select'
import { FormField } from '@/components/common/form-field'
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
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<LogTab>('api')
  const [search, setSearch] = useState('')
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [retentionDays, setRetentionDays] = useState('7')
  const [clearOpen, setClearOpen] = useState(false)
  const [clearType, setClearType] = useState<LogType>('api')
  const [notice, setNotice] = useState<{ variant: 'error' | 'success'; text: string } | null>(null)
  const queryString = new URLSearchParams({ page: '1', size: '100', ...(search.trim() ? { q: search.trim() } : {}) }).toString()
  const logs = useQuery<Page<LogEntry>>({ queryKey: ['logs', tab, search], queryFn: async () => (tab === 'api' ? await logApi.api(queryString) : tab === 'application' ? await logApi.application(queryString) : await logApi.audit(queryString)) as Page<LogEntry> })
  const settings = useQuery<LogSettings>({ queryKey: ['log-settings'], queryFn: logApi.settings })
  const saveSettings = useMutation({
    mutationFn: (days: number) => logApi.saveSettings(days),
    onSuccess: async (value) => { queryClient.setQueryData(['log-settings'], value); setSettingsOpen(false); setNotice({ variant: 'success', text: t('logs.settingsSaved') }); await queryClient.invalidateQueries({ queryKey: ['logs'] }) },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('logs.settingsSaveError')) }),
  })
  const clearLogs = useMutation({
    mutationFn: (type: LogType) => logApi.clear(type),
    onSuccess: async (stats) => { setClearOpen(false); setNotice({ variant: 'success', text: t('logs.cleared', { count: stats.api + stats.application + stats.audit }) }); await queryClient.invalidateQueries({ queryKey: ['logs'] }) },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('logs.clearError')) }),
  })
  const items = logs.data?.items ?? []
  const exportCSV = () => {
    if (items.length === 0) return
    const columns = Object.keys(items[0]) as (keyof LogEntry)[]
    const csv = [columns.map(csvCell).join(','), ...items.map((item) => columns.map((column) => csvCell(item[column])).join(','))].join('\n')
    const url = URL.createObjectURL(new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `oscraper-${tab}-logs-${new Date().toISOString().slice(0, 10)}.csv`
    anchor.click()
    URL.revokeObjectURL(url)
  }
  function openSettings() {
    if (settings.error) {
      setNotice({ variant: 'error', text: errorMessage(settings.error, t('logs.settingsLoadError')) })
      return
    }
    setRetentionDays(String(settings.data?.retention_days ?? 7))
    setSettingsOpen(true)
  }
  function openClear() {
    setClearType(tab)
    setClearOpen(true)
  }
  function submitSettings(event: FormEvent) {
    event.preventDefault()
    saveSettings.mutate(Number(retentionDays))
  }
  return <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
    {notice && <div className="mb-4"><Message variant={notice.variant}>{notice.text}</Message></div>}
    <Panel title={t('navigation.logs')} description={t('logs.description')} icon={<Activity size={20} />} action={<div className="flex flex-wrap justify-end gap-2"><Button variant="outline" size="sm" className="min-h-11 gap-2" disabled={settings.isLoading} onClick={openSettings}><Settings size={15} />{t('logs.retention')}</Button><Button variant="outline" size="sm" className="min-h-11 gap-2 text-red-700 dark:text-red-300" onClick={openClear}><Trash size={15} />{t('logs.clear')}</Button><Button variant="outline" size="sm" className="min-h-11 gap-2" disabled={items.length === 0} onClick={exportCSV}><Download size={15} />{t('logs.export')}</Button><Button variant="outline" size="sm" className="min-h-11 gap-2" disabled={logs.isFetching} onClick={() => void logs.refetch()}><Refresh size={15} className={logs.isFetching ? 'animate-spin' : ''} />{t('common.refresh')}</Button></div>}>
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><Tabs value={tab} onValueChange={(value) => setTab(value as LogTab)} className="block"><TabsList aria-label={t('navigation.logs')}>{(['api', 'application', 'audit'] as LogTab[]).map((value) => <TabsTrigger key={value} value={value}>{t(`logs.tabs.${value}`)}</TabsTrigger>)}</TabsList></Tabs><Input className="sm:max-w-sm" value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('logs.search')} /></div>
    {logs.error && <div className="mt-4"><Message variant="error">{errorMessage(logs.error, t('logs.loadError'))}</Message></div>}
    {logs.isLoading && <p className="py-12 text-center text-sm text-neutral-500">{t('common.loading')}</p>}
    {!logs.isLoading && items.length === 0 && <p className="py-12 text-center text-sm text-neutral-500">{t('logs.empty')}</p>}
    <div className="mt-5 space-y-3 sm:hidden">
      {tab === 'api' && (items as APIRequestLog[]).map((item) => <article key={item.id} className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
        <div className="flex flex-wrap items-center justify-between gap-2"><span className="text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</span><span><Badge variant="outline">{item.method}</Badge><span className={`ml-2 text-xs font-semibold ${item.status_code >= 400 ? 'text-red-600' : 'text-emerald-600'}`}>{item.status_code}</span></span></div>
        <p className="mt-3 break-all font-mono text-xs">{item.route}</p>{item.error_code && <p className="mt-1 text-xs text-red-600">{item.error_code}</p>}
        <p className="mt-3 text-xs text-neutral-500">{item.latency_ms} ms{item.job_id ? ` · Job #${item.job_id}` : ''}{item.target_id ? ` · Target #${item.target_id}` : ''}</p>
      </article>)}
      {tab === 'application' && (items as ApplicationLog[]).map((item) => <article key={item.id} className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
        <div className="flex flex-wrap items-center justify-between gap-2"><span className="text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</span><Badge variant="outline">{item.level}</Badge></div>
        <p className="mt-3 text-sm">{item.message}</p><p className="mt-1 text-xs text-neutral-500">{item.source}</p>
        {item.fields && <details className="mt-3"><summary className="cursor-pointer text-xs text-neutral-500">{t('logs.fields')}</summary><pre className="mt-2 whitespace-pre-wrap break-all text-xs">{item.fields}</pre></details>}
        <p className="mt-3 text-xs text-neutral-500">{item.job_id ? `Job #${item.job_id}` : '—'}{item.target_id ? ` · Target #${item.target_id}` : ''}</p>
      </article>)}
      {tab === 'audit' && (items as AuditLog[]).map((item) => <article key={item.id} className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
        <div className="flex flex-wrap items-center justify-between gap-2"><span className="text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</span><Badge variant="outline">{item.action}</Badge></div>
        <p className="mt-3 break-all text-sm">{item.target}</p><details className="mt-2"><summary className="cursor-pointer text-xs text-neutral-500">{t('logs.fields')}</summary><pre className="mt-2 whitespace-pre-wrap break-all text-xs">{item.detail}</pre></details>
        <p className="mt-3 text-xs text-neutral-500">Actor #{item.actor_id}</p>
      </article>)}
    </div>
    <div className="mt-5 hidden overflow-x-auto sm:block"><Table size="sm" borderStyle="none" hoverableRows className="min-w-[760px]"><TableHeader><TableRow className="text-xs uppercase tracking-wide text-neutral-500"><TableHead>{t('logs.time')}</TableHead><TableHead>{t('logs.type')}</TableHead><TableHead>{t('logs.summary')}</TableHead><TableHead>{t('logs.context')}</TableHead></TableRow></TableHeader><TableBody>
      {tab === 'api' && (items as APIRequestLog[]).map((item) => <TableRow key={item.id} className="align-top"><TableCell className="whitespace-nowrap text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</TableCell><TableCell><Badge variant="outline">{item.method}</Badge><span className={`ml-2 text-xs font-semibold ${item.status_code >= 400 ? 'text-red-600' : 'text-emerald-600'}`}>{item.status_code}</span></TableCell><TableCell><p className="font-mono text-xs">{item.route}</p>{item.error_code && <p className="mt-1 text-xs text-red-600">{item.error_code}</p>}</TableCell><TableCell className="text-xs text-neutral-500">{item.latency_ms} ms{item.job_id ? ` · Job #${item.job_id}` : ''}{item.target_id ? ` · Target #${item.target_id}` : ''}</TableCell></TableRow>)}
      {tab === 'application' && (items as ApplicationLog[]).map((item) => <TableRow key={item.id} className="align-top"><TableCell className="whitespace-nowrap text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</TableCell><TableCell><Badge variant="outline">{item.level}</Badge><p className="mt-1 text-xs text-neutral-500">{item.source}</p></TableCell><TableCell><p>{item.message}</p>{item.fields && <details className="mt-1"><summary className="cursor-pointer text-xs text-neutral-500">{t('logs.fields')}</summary><pre className="mt-2 max-w-2xl whitespace-pre-wrap break-all text-xs">{item.fields}</pre></details>}</TableCell><TableCell className="text-xs text-neutral-500">{item.job_id ? `Job #${item.job_id}` : '—'}{item.target_id ? ` · Target #${item.target_id}` : ''}</TableCell></TableRow>)}
      {tab === 'audit' && (items as AuditLog[]).map((item) => <TableRow key={item.id} className="align-top"><TableCell className="whitespace-nowrap text-xs text-neutral-500">{new Date(item.occurred_at).toLocaleString()}</TableCell><TableCell><Badge variant="outline">{item.action}</Badge></TableCell><TableCell><p>{item.target}</p><details className="mt-1"><summary className="cursor-pointer text-xs text-neutral-500">{t('logs.fields')}</summary><pre className="mt-2 max-w-2xl whitespace-pre-wrap break-all text-xs">{item.detail}</pre></details></TableCell><TableCell className="text-xs text-neutral-500">Actor #{item.actor_id}</TableCell></TableRow>)}
    </TableBody></Table></div>
    {logs.data && <p className="mt-4 text-xs text-neutral-500">{t('logs.total', { count: logs.data.total })}</p>}
    </Panel>
    <AppDialog open={settingsOpen} onOpenChange={setSettingsOpen} width="sm" closeLabel={t('common.close')} title={t('logs.retentionTitle')} description={t('logs.retentionDescription')} onSubmit={submitSettings} footer={<><Button type="button" variant="ghost" onClick={() => setSettingsOpen(false)}>{t('common.cancel')}</Button><Button type="submit" className="gap-2" disabled={saveSettings.isPending}><Check size={16} />{saveSettings.isPending ? t('logs.saving') : t('common.save')}</Button></>}>
      <FormField label={t('logs.retentionDays')}><AppSelect value={retentionDays} onValueChange={setRetentionDays} ariaLabel={t('logs.retentionDays')} options={Array.from({ length: 30 }, (_, index) => ({ value: String(index + 1), label: t('logs.dayCount', { count: index + 1 }) }))} /></FormField>
    </AppDialog>
    <AppDialog open={clearOpen} onOpenChange={setClearOpen} width="sm" closeLabel={t('common.close')} title={t('logs.clearTitle')} description={t('logs.clearDescription')} footer={<><Button variant="ghost" onClick={() => setClearOpen(false)}>{t('common.cancel')}</Button><Button className="gap-2 bg-red-600 text-white hover:bg-red-700" disabled={clearLogs.isPending} onClick={() => clearLogs.mutate(clearType)}><Trash size={16} />{clearLogs.isPending ? t('logs.clearing') : t('logs.confirmClear')}</Button></>}>
      <FormField label={t('logs.clearScope')}><AppSelect value={clearType} onValueChange={(value) => setClearType(value as LogType)} ariaLabel={t('logs.clearScope')} options={(['api', 'application', 'audit', 'all'] as LogType[]).map((value) => ({ value, label: t(`logs.clearTypes.${value}`) }))} /></FormField>
      {(clearType === 'audit' || clearType === 'all') && <div className="mt-4"><Message variant="error">{t('logs.auditClearNotice')}</Message></div>}
    </AppDialog>
  </div>
}
