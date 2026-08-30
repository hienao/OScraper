import { Activity, Check, FileText, Refresh, Settings, X } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { Button } from '@appica/ui-react/button'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { jobApi } from '@/api/services'
import type { JobRecordSettings, JobStatus, ScrapeJob } from '@/api/types'
import { AppDialog } from '@/components/common/app-dialog'
import { AppSelect } from '@/components/common/app-select'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

export function JobsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<JobStatus | ''>('')
  const [selected, setSelected] = useState<ScrapeJob | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [retentionDays, setRetentionDays] = useState('7')
  const [notice, setNotice] = useState<{ variant: 'error' | 'success'; text: string } | null>(null)
  const jobs = useQuery({ queryKey: ['jobs', status], queryFn: () => jobApi.list(status), refetchInterval: 2500 })
  const settings = useQuery<JobRecordSettings>({ queryKey: ['job-record-settings'], queryFn: jobApi.settings })
  const jobDetail = useQuery({ queryKey: ['job', selected?.id], queryFn: () => jobApi.get(selected!.id), enabled: Boolean(selected), refetchInterval: 2000 })
  const current = jobDetail.data ?? selected
  const operations = useQuery({ queryKey: ['job-operations', selected?.id], queryFn: () => jobApi.operations(selected!.id), enabled: Boolean(selected), refetchInterval: current?.status === 'pending' || current?.status === 'running' ? 2000 : false })
  const retry = useMutation({ mutationFn: jobApi.retry, onSuccess: async (job) => { setSelected(job); setNotice(null); await queryClient.invalidateQueries({ queryKey: ['jobs'] }) }, onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('jobs.retryError')) }) })
  const cancel = useMutation({ mutationFn: jobApi.cancel, onSuccess: async (job) => { setSelected(job); setNotice(null); await queryClient.invalidateQueries({ queryKey: ['jobs'] }) }, onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('jobs.cancelError')) }) })
  const saveSettings = useMutation({
    mutationFn: (days: number) => jobApi.saveSettings(days),
    onSuccess: async (value) => { queryClient.setQueryData(['job-record-settings'], value); setSettingsOpen(false); setNotice({ variant: 'success', text: t('jobs.settingsSaved') }); await queryClient.invalidateQueries({ queryKey: ['jobs'] }) },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('jobs.settingsSaveError')) }),
  })

  function openSettings() {
    if (settings.error) {
      setNotice({ variant: 'error', text: errorMessage(settings.error, t('jobs.settingsLoadError')) })
      return
    }
    setRetentionDays(String(settings.data?.retention_days ?? 7))
    setSettingsOpen(true)
  }

  function submitSettings(event: FormEvent) {
    event.preventDefault()
    saveSettings.mutate(Number(retentionDays))
  }

  return <div className="mx-auto max-w-7xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
    {notice && <Message variant={notice.variant}>{notice.text}</Message>}
    <Panel title={t('jobs.title')} description={t('jobs.description')} icon={<Activity size={20} />} action={<div className="flex w-full flex-wrap justify-end gap-2 sm:w-auto"><Button variant="outline" size="sm" className="min-h-11 gap-2" disabled={settings.isLoading} onClick={openSettings}><Settings size={15} />{t('jobs.retention')}</Button><AppSelect className="min-h-11 sm:w-44" value={status} onValueChange={(value) => setStatus(value as JobStatus | '')} ariaLabel={t('jobs.filter')} options={[{ value: '', label: t('jobs.all') }, ...(['pending', 'running', 'succeeded', 'failed', 'canceled'] as JobStatus[]).map((value) => ({ value, label: t(`jobs.status.${value}`) }))]} /></div>}>
      {jobs.isLoading && <p className="text-sm text-neutral-500">{t('common.loading')}</p>}
      {jobs.error && <Message variant="error">{errorMessage(jobs.error, t('jobs.loadError'))}</Message>}
      {jobs.data?.items.length === 0 && <p className="py-12 text-center text-sm text-neutral-500">{t('jobs.empty')}</p>}
      <div className="space-y-3">{jobs.data?.items.map((job) => <button key={job.id} type="button" onClick={() => setSelected(job)} className="w-full rounded-xl border border-neutral-200 p-4 text-left transition hover:border-emerald-400 dark:border-neutral-800">
        <div className="flex flex-wrap items-start justify-between gap-3"><div><p className="font-semibold">{t('jobs.jobNumber', { id: job.id })}</p><p className="mt-1 text-xs text-neutral-500">{t('jobs.targetNumber', { id: job.target_id })} · {new Date(job.created_at).toLocaleString()}</p></div><Badge variant={job.status === 'succeeded' ? 'soft' : 'outline'}>{t(`jobs.status.${job.status}`)}</Badge></div>
        <div className="mt-4 h-2 overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-900"><div className={`h-full rounded-full ${job.status === 'failed' ? 'bg-red-500' : 'bg-emerald-600'}`} style={{ width: `${job.progress}%` }} /></div>
        <div className="mt-2 flex justify-between gap-3 text-xs text-neutral-500"><span>{t(`jobs.stage.${job.stage}`)}</span><span>{job.progress}%</span></div>
        {job.message && <p className="mt-2 text-sm text-neutral-600 dark:text-neutral-400">{job.message}</p>}
      </button>)}</div>
    </Panel>

    {current && <AppDialog open={Boolean(current)} onOpenChange={(open) => { if (!open) setSelected(null) }} width="lg" closeLabel={t('common.close')} title={t('jobs.jobNumber', { id: current.id })} description={`${t(`jobs.status.${current.status}`)} · ${t(`jobs.stage.${current.stage}`)} · ${current.progress}%`} footer={<>{current.status === 'failed' && <Button className="gap-2" disabled={retry.isPending} onClick={() => retry.mutate(current.id)}><Refresh size={16} />{t('jobs.retry')}</Button>}{current.status === 'pending' && <Button variant="outline" disabled={cancel.isPending} onClick={() => cancel.mutate(current.id)}>{t('jobs.cancel')}</Button>}<Button variant="outline" className="gap-2" onClick={() => void operations.refetch()}><FileText size={16} />{t('common.refresh')}</Button></>}>
      {current.error_code && <Message variant="error"><strong>{current.error_code}</strong><span className="mt-1 block">{current.error_message}</span></Message>}
      <div className="mt-5 grid gap-3 sm:grid-cols-3"><div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('jobs.attempts')}</p><p className="mt-1 text-xl font-bold">{current.attempts}</p></div><div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('jobs.checkpoint')}</p><p className="mt-1 text-xl font-bold">{current.checkpoint}</p></div><div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('jobs.preview')}</p><p className="mt-1 text-xl font-bold">#{current.preview_id}</p></div></div>
      <h3 className="mt-6 font-semibold">{t('jobs.operations')}</h3>{operations.isLoading && <p className="mt-3 text-sm text-neutral-500">{t('common.loading')}</p>}
      <div className="mt-3 max-h-[45vh] space-y-2 overflow-y-auto">{operations.data?.map((operation) => <div key={operation.id} className="rounded-xl border border-neutral-200 p-3 dark:border-neutral-800"><div className="flex items-center justify-between gap-3"><span className="flex items-center gap-2 text-sm font-medium">{operation.status === 'succeeded' || operation.status === 'skipped' ? <Check size={15} className="text-emerald-600" /> : operation.status === 'failed' ? <X size={15} className="text-red-600" /> : <Refresh size={15} className={operation.status === 'running' ? 'animate-spin' : ''} />}#{operation.sequence} · {operation.type}</span><Badge variant="outline">{t(`jobs.operationStatus.${operation.status}`)}</Badge></div>{operation.source_path && <p className="mt-2 break-all font-mono text-xs text-neutral-500">{operation.source_path}</p>}<p className="mt-1 break-all font-mono text-xs text-emerald-700 dark:text-emerald-300">→ {operation.target_path}</p>{operation.last_error && <p className="mt-2 text-xs text-red-600">{operation.last_error}</p>}</div>)}</div>
    </AppDialog>}
    <AppDialog open={settingsOpen} onOpenChange={setSettingsOpen} width="sm" closeLabel={t('common.close')} title={t('jobs.retentionTitle')} description={t('jobs.retentionDescription')} onSubmit={submitSettings} footer={<><Button type="button" variant="ghost" onClick={() => setSettingsOpen(false)}>{t('common.cancel')}</Button><Button type="submit" className="gap-2" disabled={saveSettings.isPending}><Check size={16} />{saveSettings.isPending ? t('jobs.saving') : t('common.save')}</Button></>}>
      <FormField label={t('jobs.retentionDays')}><AppSelect value={retentionDays} onValueChange={setRetentionDays} ariaLabel={t('jobs.retentionDays')} options={Array.from({ length: 30 }, (_, index) => ({ value: String(index + 1), label: t('jobs.dayCount', { count: index + 1 }) }))} /></FormField>
    </AppDialog>
  </div>
}
