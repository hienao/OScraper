import { ArrowLeft, Check, FileText, Movie, Package, Plus, Refresh, Settings, Trash } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { connectionApi, jobApi, localStorageApi, previewApi, targetApi } from '@/api/services'
import type { LibraryType, LocalStorageStatus, MediaCandidate, ScanRun, ScrapePreview, ScrapeTarget, SourceType, TargetInput, TMDBSearchResult } from '@/api/types'
import { AppDialog } from '@/components/common/app-dialog'
import { AppSelect } from '@/components/common/app-select'
import { CheckboxField } from '@/components/common/checkbox-field'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

const emptyForm: TargetInput = { source_type: 'openlist', connection_id: 0, name: '', root_path: '/', library_type: 'movie', rename_enabled: false, enabled: true }

function normalizeRemotePath(value: string) {
  return value.replace(/\/+$/, '') || '/'
}

function isWithinRemotePath(rootPath: string, candidatePath: string) {
  const root = normalizeRemotePath(rootPath)
  const candidate = normalizeRemotePath(candidatePath)
  return root === '/' ? candidate.startsWith('/') : candidate === root || candidate.startsWith(`${root}/`)
}

function localStatusDescription(status: LocalStorageStatus, t: TFunction) {
  const groups = Array.from(new Set([status.gid, ...status.groups])).join(', ')
  if (!status.mounted) return t('targets.localNotMounted', { root: status.root })
  if (!status.readable) return t('targets.localUnreadable', { root: status.root, uid: status.uid, groups })
  if (!status.writable) return t('targets.localReadOnly', { root: status.root, uid: status.uid, groups })
  return t('targets.localWritable', { root: status.root, uid: status.uid, groups })
}

export function TargetsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const targets = useQuery({ queryKey: ['targets'], queryFn: targetApi.list })
  const connections = useQuery({ queryKey: ['connections'], queryFn: connectionApi.list })
  const localStatus = useQuery({ queryKey: ['local-storage-status'], queryFn: localStorageApi.status })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ScrapeTarget | null>(null)
  const [form, setForm] = useState<TargetInput>(emptyForm)
  const [browsing, setBrowsing] = useState<ScrapeTarget | null>(null)
  const [browserPath, setBrowserPath] = useState('')
  const [localBrowserOpen, setLocalBrowserOpen] = useState(false)
  const [localBrowserPath, setLocalBrowserPath] = useState('/media')
  const [remoteBrowserOpen, setRemoteBrowserOpen] = useState(false)
  const [remoteBrowserPath, setRemoteBrowserPath] = useState('/')
  const [scanTarget, setScanTarget] = useState<ScrapeTarget | null>(null)
  const [scanResult, setScanResult] = useState<ScanRun | null>(null)
  const [matchCandidate, setMatchCandidate] = useState<MediaCandidate | null>(null)
  const [searchTitle, setSearchTitle] = useState('')
  const [searchYear, setSearchYear] = useState('')
  const [searchResults, setSearchResults] = useState<TMDBSearchResult[]>([])
  const [preview, setPreview] = useState<ScrapePreview | null>(null)
  const [matchError, setMatchError] = useState<string | null>(null)
  const [notice, setNotice] = useState<{ variant: 'error' | 'success'; text: string } | null>(null)

  const create = useMutation({ mutationFn: targetApi.create })
  const update = useMutation({ mutationFn: ({ id, input }: { id: number; input: TargetInput }) => targetApi.update(id, input) })
  const remove = useMutation({
    mutationFn: targetApi.remove,
    onSuccess: async () => { setNotice({ variant: 'success', text: t('targets.deleted') }); await queryClient.invalidateQueries({ queryKey: ['targets'] }) },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('targets.deleteError')) }),
  })
  const scan = useMutation({
    mutationFn: (target: ScrapeTarget) => targetApi.scan(target.id),
    onSuccess: (result) => setScanResult(result),
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('targets.scanError')) }),
  })
  const scanStatus = useQuery({
    queryKey: ['target-scan', scanTarget?.id, scanResult?.id],
    queryFn: () => targetApi.scanResult(scanTarget!.id, scanResult!.id),
    enabled: Boolean(scanTarget && scanResult?.id && (scanResult.status === 'pending' || scanResult.status === 'running')),
    refetchInterval: (query) => {
      const status = query.state.data?.status ?? scanResult?.status
      return status === 'pending' || status === 'running' ? 1000 : false
    },
  })
  const searchTMDB = useMutation({
    mutationFn: ({ candidate, title, year }: { candidate: MediaCandidate; title: string; year?: number }) => previewApi.search(candidate.target_id, { candidate_id: candidate.id, title, year }),
    onSuccess: (results) => setSearchResults(results),
    onError: (error) => setMatchError(errorMessage(error, t('targets.matchError'))),
  })
  const createPreview = useMutation({
    mutationFn: ({ candidate, tmdbId }: { candidate: MediaCandidate; tmdbId?: number }) => previewApi.create(candidate.target_id, { candidate_id: candidate.id, tmdb_id: tmdbId }),
    onSuccess: (result) => setPreview(result),
    onError: (error) => setMatchError(errorMessage(error, t('targets.previewError'))),
  })
  const executeJob = useMutation({
    mutationFn: (value: ScrapePreview) => jobApi.submit(value.target_id, { preview_id: value.id, rename_media: value.plan.proposed_directory_creates.length + value.plan.proposed_directory_renames.length + value.plan.proposed_file_renames.length > 0, confirm_directory_fingerprint: value.fingerprint }, crypto.randomUUID()),
    onSuccess: () => { setMatchCandidate(null); void navigate('/jobs') },
    onError: (error) => setMatchError(errorMessage(error, t('targets.executeError'))),
  })
  const tree = useQuery({
    queryKey: ['target-tree', browsing?.id, browserPath],
    queryFn: () => targetApi.tree(browsing!.id, browserPath),
    enabled: Boolean(browsing && browserPath),
  })
  const refreshTree = useMutation({
    mutationFn: () => targetApi.tree(browsing!.id, browserPath, true),
    onSuccess: (data) => queryClient.setQueryData(['target-tree', browsing?.id, browserPath], data),
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('targets.browserError')) }),
  })
  const localTree = useQuery({
    queryKey: ['local-storage-tree', localBrowserPath],
    queryFn: () => localStorageApi.tree(localBrowserPath),
    enabled: localBrowserOpen,
  })
  const remoteTree = useQuery({
    queryKey: ['openlist-connection-tree', form.connection_id, remoteBrowserPath],
    queryFn: () => connectionApi.tree(form.connection_id!, remoteBrowserPath),
    enabled: remoteBrowserOpen && Boolean(form.connection_id),
  })

  const enabledConnections = (connections.data ?? []).filter((connection) => connection.enabled)
  const selectedConnection = connections.data?.find((connection) => connection.id === form.connection_id)
  const localRoot = localStatus.data?.root ?? '/media'

  function openCreate() {
    setEditing(null)
    const connection = enabledConnections[0]
    setForm({ ...emptyForm, source_type: connection ? 'openlist' : 'local', connection_id: connection?.id, root_path: connection ? normalizeRemotePath(connection.base_path) : localRoot })
    setNotice(null)
    setFormOpen(true)
  }
  function openEdit(target: ScrapeTarget) {
    setEditing(target)
    setForm({ source_type: target.source_type, connection_id: target.connection_id, name: target.name, root_path: target.root_path, library_type: target.library_type, rename_enabled: target.rename_enabled, enabled: target.enabled })
    setFormOpen(true)
  }
  function openBrowser(target: ScrapeTarget) { setBrowsing(target); setBrowserPath(target.root_path) }
  function startScan(target: ScrapeTarget) {
    setNotice(null)
    setScanTarget(target)
    setScanResult(null)
    scan.mutate(target)
  }
  function startMatch(candidate: MediaCandidate) {
    setMatchCandidate(candidate)
    setSearchTitle(candidate.parsed_title)
    setSearchYear(candidate.year ? String(candidate.year) : '')
    setSearchResults([])
    setPreview(null)
    setMatchError(null)
    if (candidate.tmdb_id) createPreview.mutate({ candidate, tmdbId: candidate.tmdb_id })
    else searchTMDB.mutate({ candidate, title: candidate.parsed_title, year: candidate.year })
  }
  function runSearch(event: FormEvent) {
    event.preventDefault()
    if (!matchCandidate) return
    setMatchError(null)
    setPreview(null)
    searchTMDB.mutate({ candidate: matchCandidate, title: searchTitle.trim(), year: searchYear ? Number(searchYear) : undefined })
  }

  function executePreview(value: ScrapePreview) {
    const renameCount = value.plan.proposed_directory_creates.length + value.plan.proposed_directory_renames.length + value.plan.proposed_file_renames.length
    if (window.confirm(t('targets.executeConfirm', { renames: renameCount, files: value.plan.artifacts.length }))) executeJob.mutate(value)
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setNotice(null)
    try {
      if (editing) await update.mutateAsync({ id: editing.id, input: form })
      else await create.mutateAsync(form)
      await queryClient.invalidateQueries({ queryKey: ['targets'] })
      setFormOpen(false)
      setNotice({ variant: 'success', text: t('targets.saved') })
    } catch (error) {
      setNotice({ variant: 'error', text: errorMessage(error, t('targets.formError')) })
    }
  }

  function goUp() {
    if (!browsing || browserPath === browsing.root_path) return
    const slash = browserPath.lastIndexOf('/')
    const parent = slash <= 0 ? '/' : browserPath.slice(0, slash)
    setBrowserPath(parent.length < browsing.root_path.length ? browsing.root_path : parent)
  }

  function changeSource(source: SourceType) {
    const connection = enabledConnections[0]
    setForm({ ...form, source_type: source, connection_id: source === 'openlist' ? connection?.id : undefined, root_path: source === 'local' ? localRoot : normalizeRemotePath(connection?.base_path ?? '/') })
  }

  function changeConnection(connectionID: number) {
    const connection = connections.data?.find((item) => item.id === connectionID)
    setForm({ ...form, connection_id: connectionID, root_path: normalizeRemotePath(connection?.base_path ?? '/') })
  }

  function openLocalBrowser() {
    setLocalBrowserPath(form.root_path === localRoot || form.root_path.startsWith(`${localRoot}/`) ? form.root_path : localRoot)
    setLocalBrowserOpen(true)
  }

  function localGoUp() {
    if (localBrowserPath === localRoot) return
    const slash = localBrowserPath.lastIndexOf('/')
    setLocalBrowserPath(slash <= localRoot.length ? localRoot : localBrowserPath.slice(0, slash))
  }

  function openRemoteBrowser() {
    const rootPath = normalizeRemotePath(selectedConnection?.base_path ?? '/')
    const initialPath = isWithinRemotePath(rootPath, form.root_path) ? form.root_path : rootPath
    setRemoteBrowserPath(initialPath)
    setRemoteBrowserOpen(true)
  }

  function remoteGoUp() {
    const rootPath = normalizeRemotePath(remoteTree.data?.root_path ?? selectedConnection?.base_path ?? '/')
    if (remoteBrowserPath === rootPath) return
    const slash = remoteBrowserPath.lastIndexOf('/')
    const parent = slash <= 0 ? '/' : remoteBrowserPath.slice(0, slash)
    setRemoteBrowserPath(isWithinRemotePath(rootPath, parent) ? parent : rootPath)
  }

  const saving = create.isPending || update.isPending
  const currentScan = scanStatus.data ?? scanResult
  const scanActive = scan.isPending || currentScan?.status === 'pending' || currentScan?.status === 'running'

  return (
    <div className="mx-auto max-w-7xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
      {notice && <Message variant={notice.variant}>{notice.text}</Message>}
      <Panel title={t('targets.title')} description={t('targets.description')} icon={<Movie size={20} />} action={<Button className="gap-2" onClick={openCreate}><Plus size={17} />{t('targets.add')}</Button>}>
        {targets.isLoading && <p className="text-sm text-neutral-500">{t('common.loading')}</p>}
        {targets.error && <Message variant="error">{errorMessage(targets.error, t('errors.requestFailed'))}</Message>}
        {targets.data?.length === 0 && <div className="py-12 text-center"><span className="mx-auto grid size-12 place-items-center rounded-2xl bg-neutral-100 text-neutral-500 dark:bg-neutral-900"><Movie size={22} /></span><h2 className="mt-4 font-semibold">{t('targets.empty')}</h2><p className="mt-1 text-sm text-neutral-500">{t('targets.emptyDescription')}</p></div>}
        <div className="grid gap-4 lg:grid-cols-2">
          {targets.data?.map((target) => (
            <article key={target.id} className="rounded-2xl border border-neutral-200 p-5 dark:border-neutral-800">
              <div className="flex items-start justify-between gap-4"><div><h2 className="font-semibold">{target.name}</h2><p className="mt-1 text-sm text-neutral-500">{target.connection_name}</p></div><Badge variant={target.enabled ? 'soft' : 'outline'}>{t(target.enabled ? 'common.enabled' : 'common.disabled')}</Badge></div>
              <p className="mt-4 break-all rounded-xl bg-neutral-50 p-3 font-mono text-xs dark:bg-neutral-900">{target.root_path}</p>
              <div className="mt-3 flex flex-wrap gap-2 text-xs"><Badge variant="outline">{t(`targets.source.${target.source_type}`)}</Badge><Badge variant="outline">{t(`targets.${target.library_type}`)}</Badge>{target.rename_enabled && <Badge variant="outline">{t('targets.rename')}</Badge>}</div>
              <div className="mt-5 flex flex-wrap gap-2">
                <Button size="sm" className="gap-2" disabled={!target.enabled || (scanActive && scanTarget?.id === target.id)} onClick={() => startScan(target)}><Refresh size={15} />{scanActive && scanTarget?.id === target.id ? t('targets.scanning') : t('targets.scan')}</Button>
                <Button size="sm" variant="outline" className="gap-2" onClick={() => openBrowser(target)}><Package size={15} />{t('targets.browse')}</Button>
                <Button size="sm" variant="outline" className="gap-2" onClick={() => openEdit(target)}><Settings size={15} />{t('common.edit')}</Button>
                <Button size="sm" variant="outline" className="gap-2 text-red-700 dark:text-red-300" disabled={remove.isPending} onClick={() => { if (window.confirm(t('targets.deleteConfirm', { name: target.name }))) remove.mutate(target.id) }}><Trash size={15} />{t('common.remove')}</Button>
              </div>
            </article>
          ))}
        </div>
      </Panel>

      <AppDialog open={formOpen} onOpenChange={setFormOpen} width="sm" closeLabel={t('common.close')} title={t(editing ? 'targets.editTitle' : 'targets.createTitle')} description={t('targets.createDescription')} onSubmit={(event) => void submit(event)} bodyClassName="space-y-4" footer={<><Button type="button" variant="ghost" onClick={() => setFormOpen(false)}>{t('common.cancel')}</Button><Button type="submit" className="gap-2" disabled={saving || (form.source_type === 'openlist' && !form.connection_id)}>{saving ? t('targets.saving') : <><Check size={16} />{t('common.save')}</>}</Button></>}>
        <FormField label={t('targets.name')}><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder={t('targets.placeholderName')} required maxLength={100} /></FormField>
        <FormField label={t('targets.sourceType')}><AppSelect value={form.source_type} onValueChange={(value) => changeSource(value as SourceType)} ariaLabel={t('targets.sourceType')} options={[{ value: 'openlist', label: t('targets.source.openlist'), disabled: enabledConnections.length === 0 }, { value: 'local', label: t('targets.source.local') }]} /></FormField>
        {form.source_type === 'openlist' && <FormField label={t('targets.connection')}><AppSelect value={String(form.connection_id ?? '')} onValueChange={(value) => changeConnection(Number(value))} ariaLabel={t('targets.connection')} options={(connections.data ?? []).map((connection) => ({ value: String(connection.id), label: connection.name, disabled: !connection.enabled }))} /></FormField>}
        <FormField label={t('targets.rootPath')} description={form.source_type === 'local' && localStatus.data ? localStatusDescription(localStatus.data, t) : undefined}><div className="flex gap-2"><Input value={form.root_path} readOnly required /><Button type="button" variant="outline" disabled={form.source_type === 'openlist' && !form.connection_id} onClick={form.source_type === 'local' ? openLocalBrowser : openRemoteBrowser}>{t('targets.choose')}</Button></div></FormField>
        <FormField label={t('targets.libraryType')}><AppSelect value={form.library_type} onValueChange={(library_type) => setForm({ ...form, library_type: library_type as LibraryType })} ariaLabel={t('targets.libraryType')} options={(['movie', 'tv', 'anime'] as LibraryType[]).map((type) => ({ value: type, label: t(`targets.${type}`) }))} /></FormField>
        <CheckboxField checked={form.rename_enabled} onCheckedChange={(rename_enabled) => setForm({ ...form, rename_enabled })} label={t('targets.rename')} description={t('targets.renameWarning')} />
        {editing && <CheckboxField checked={form.enabled} onCheckedChange={(enabled) => setForm({ ...form, enabled })} label={t('targets.enabled')} />}
      </AppDialog>

      <AppDialog open={remoteBrowserOpen} onOpenChange={setRemoteBrowserOpen} width="md" closeLabel={t('common.close')} title={t('targets.openListBrowserTitle')} description={<span className="break-all font-mono text-xs">{remoteBrowserPath}</span>}>
            <div className="mt-5 flex flex-wrap gap-2"><Button size="sm" variant="outline" className="gap-2" disabled={remoteBrowserPath === normalizeRemotePath(remoteTree.data?.root_path ?? selectedConnection?.base_path ?? '/')} onClick={remoteGoUp}><ArrowLeft size={15} />{t('targets.up')}</Button><Button size="sm" onClick={() => { setForm({ ...form, root_path: remoteBrowserPath }); setRemoteBrowserOpen(false) }}>{t('targets.selectDirectory')}</Button></div>
            <div className="mt-4 overflow-x-clip rounded-xl border border-neutral-200 dark:border-neutral-800">
              {remoteTree.isLoading && <p className="p-5 text-sm text-neutral-500">{t('common.loading')}</p>}
              {remoteTree.error && <div className="p-4"><Message variant="error">{errorMessage(remoteTree.error, t('targets.browserError'))}</Message></div>}
              {remoteTree.data?.entries.filter((entry) => entry.is_dir).length === 0 && <p className="p-8 text-center text-sm text-neutral-500">{t('targets.noEntries')}</p>}
              {remoteTree.data?.entries.filter((entry) => entry.is_dir).map((entry) => <button key={entry.path} type="button" onClick={() => setRemoteBrowserPath(entry.path)} className="flex w-full min-w-0 items-center gap-3 border-b border-neutral-200 px-4 py-3 text-left last:border-0 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900"><span className="grid size-9 shrink-0 place-items-center rounded-lg bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><Package size={17} /></span><span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{entry.name}</span></span></button>)}
            </div>
      </AppDialog>

      <AppDialog open={localBrowserOpen} onOpenChange={setLocalBrowserOpen} width="md" closeLabel={t('common.close')} title={t('targets.localBrowserTitle')} description={<span className="break-all font-mono text-xs">{localBrowserPath}</span>}>
            <div className="mt-5 flex flex-wrap gap-2"><Button size="sm" variant="outline" className="gap-2" disabled={localBrowserPath === localRoot} onClick={localGoUp}><ArrowLeft size={15} />{t('targets.up')}</Button><Button size="sm" onClick={() => { setForm({ ...form, root_path: localBrowserPath }); setLocalBrowserOpen(false) }}>{t('targets.selectDirectory')}</Button></div>
            <div className="mt-4 overflow-x-clip rounded-xl border border-neutral-200 dark:border-neutral-800">
              {localTree.isLoading && <p className="p-5 text-sm text-neutral-500">{t('common.loading')}</p>}
              {localTree.error && <div className="p-4"><Message variant="error">{errorMessage(localTree.error, t('targets.browserError'))}</Message></div>}
              {localTree.data?.entries.filter((entry) => entry.is_dir).length === 0 && <p className="p-8 text-center text-sm text-neutral-500">{t('targets.noEntries')}</p>}
              {localTree.data?.entries.filter((entry) => entry.is_dir).map((entry) => <button key={entry.path} type="button" onClick={() => setLocalBrowserPath(entry.path)} className="flex w-full min-w-0 items-center gap-3 border-b border-neutral-200 px-4 py-3 text-left last:border-0 hover:bg-neutral-50 dark:border-neutral-800 dark:hover:bg-neutral-900"><span className="grid size-9 shrink-0 place-items-center rounded-lg bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><Package size={17} /></span><span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{entry.name}</span></span></button>)}
            </div>
      </AppDialog>

      {browsing && <AppDialog open onOpenChange={(open) => { if (!open) setBrowsing(null) }} width="md" closeLabel={t('common.close')} title={`${t('targets.browserTitle')} · ${browsing.name}`} description={<span className="break-all font-mono text-xs">{browserPath}</span>}>
            <div className="mt-5 flex flex-wrap gap-2"><Button size="sm" variant="outline" className="gap-2" disabled={browserPath === browsing.root_path} onClick={goUp}><ArrowLeft size={15} />{t('targets.up')}</Button><Button size="sm" variant="outline" className="gap-2" disabled={tree.isFetching || refreshTree.isPending} onClick={() => refreshTree.mutate()}><Refresh size={15} />{t('common.refresh')}</Button></div>
            <div className="mt-4 overflow-x-clip rounded-xl border border-neutral-200 dark:border-neutral-800">
              {tree.isLoading && <p className="p-5 text-sm text-neutral-500">{t('common.loading')}</p>}
              {tree.error && <div className="p-4"><Message variant="error">{errorMessage(tree.error, t('targets.browserError'))}</Message></div>}
              {tree.data?.entries.length === 0 && <p className="p-8 text-center text-sm text-neutral-500">{t('targets.noEntries')}</p>}
              {tree.data?.entries.map((entry) => (
                <button key={entry.path} type="button" disabled={!entry.is_dir} onClick={() => entry.is_dir && setBrowserPath(entry.path)} className="flex w-full min-w-0 items-center gap-3 border-b border-neutral-200 px-4 py-3 text-left last:border-0 enabled:hover:bg-neutral-50 disabled:cursor-default dark:border-neutral-800 dark:enabled:hover:bg-neutral-900">
                  <span className={`grid size-9 shrink-0 place-items-center rounded-lg ${entry.is_dir ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-neutral-100 text-neutral-500 dark:bg-neutral-900'}`}>{entry.is_dir ? <Package size={17} /> : <FileText size={17} />}</span>
                  <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{entry.name}</span><span className="text-xs text-neutral-500">{t(entry.is_dir ? 'targets.directory' : 'targets.file')}</span></span>
                </button>
              ))}
            </div>
      </AppDialog>}

      {scanTarget && <AppDialog open onOpenChange={(open) => { if (!open && !scanActive) setScanTarget(null) }} width="lg" closeLabel={t('common.close')} closeDisabled={scanActive} title={`${t('targets.scanTitle')} · ${scanTarget.name}`} description={t('targets.scanDescription')} footer={currentScan && !scanActive ? <Button onClick={() => setScanTarget(null)}>{t('common.close')}</Button> : undefined}>
            {scanActive && <div className="grid min-h-48 place-items-center"><div className="text-center"><Refresh className="mx-auto animate-spin text-emerald-600" size={28} /><p className="mt-3 text-sm text-neutral-500">{t('targets.scanningDescription')}</p></div></div>}
            {scan.isError && <div className="mt-5"><Message variant="error">{errorMessage(scan.error, t('targets.scanError'))}</Message></div>}
            {scanStatus.isError && <div className="mt-5"><Message variant="error">{errorMessage(scanStatus.error, t('targets.scanError'))}</Message></div>}
            {currentScan?.status === 'failed' && <div className="mt-5"><Message variant="error">{currentScan.error_message || t('targets.scanError')}</Message></div>}
            {currentScan?.status === 'succeeded' && <>
              <div className="mt-5 grid gap-3 sm:grid-cols-3">
                <div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('targets.candidates')}</p><p className="mt-1 text-2xl font-bold">{currentScan.candidate_count}</p></div>
                <div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('targets.videoFiles')}</p><p className="mt-1 text-2xl font-bold">{currentScan.video_count}</p></div>
                <div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('targets.scanStatus')}</p><p className="mt-1 text-sm font-semibold text-emerald-700 dark:text-emerald-300">{t('targets.scanSucceeded')}</p></div>
              </div>
              {currentScan.candidates?.length === 0 && <p className="py-12 text-center text-sm text-neutral-500">{t('targets.noCandidates')}</p>}
              <div className="mt-4 max-h-[55vh] space-y-3 overflow-y-auto pr-1">
                {currentScan.candidates?.map((candidate) => <article key={candidate.id || candidate.path} className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
                  <div className="flex flex-wrap items-start justify-between gap-3"><div className="min-w-0"><h3 className="truncate font-semibold">{candidate.parsed_title || candidate.path.split('/').pop()}</h3><p className="mt-1 break-all font-mono text-xs text-neutral-500">{candidate.path}</p></div><Badge variant={candidate.status === 'ready' ? 'soft' : 'outline'}>{t(`targets.${candidate.status}`)}</Badge></div>
                  <div className="mt-3 flex flex-wrap gap-2 text-xs"><Badge variant="outline">{t(`targets.${candidate.kind}`)}</Badge>{candidate.year && <Badge variant="outline">{candidate.year}</Badge>}{candidate.season !== undefined && <Badge variant="outline">S{String(candidate.season).padStart(2, '0')}</Badge>}{candidate.episode !== undefined && <Badge variant="outline">E{String(candidate.episode).padStart(2, '0')}</Badge>}{candidate.tmdb_id && <Badge variant="outline">TMDB {candidate.tmdb_id}</Badge>}<Badge variant="outline">{t('targets.confidence', { value: candidate.confidence })}</Badge><Badge variant="outline">{t('targets.videoCount', { count: candidate.video_count })}</Badge></div>
                  <div className="mt-3"><Button size="sm" variant="outline" className="gap-2" onClick={() => startMatch(candidate)}><Movie size={15} />{t('targets.tmdbPreview')}</Button></div>
                </article>)}
              </div>
            </>}
      </AppDialog>}

      {matchCandidate && <AppDialog open onOpenChange={(open) => { if (!open) setMatchCandidate(null) }} width="xl" closeLabel={t('common.close')} title={t('targets.tmdbPreview')} description={<span className="break-all font-mono text-xs">{matchCandidate.path}</span>}>
            <form className="grid gap-3 sm:grid-cols-[1fr_8rem_auto]" onSubmit={runSearch}>
              <Input value={searchTitle} onChange={(event) => setSearchTitle(event.target.value)} placeholder={t('targets.searchTitle')} required />
              <Input type="number" min={1870} max={2200} value={searchYear} onChange={(event) => setSearchYear(event.target.value)} placeholder={t('targets.searchYear')} />
              <Button type="submit" className="gap-2" disabled={searchTMDB.isPending}><Refresh size={16} />{searchTMDB.isPending ? t('targets.searching') : t('targets.searchTMDB')}</Button>
            </form>
            {matchError && <div className="mt-4"><Message variant="error">{matchError}</Message></div>}
            {(searchTMDB.isPending || createPreview.isPending) && <div className="grid min-h-40 place-items-center"><Refresh className="animate-spin text-emerald-600" size={26} /></div>}
            {!preview && !searchTMDB.isPending && !createPreview.isPending && searchResults.length === 0 && !matchError && <p className="py-12 text-center text-sm text-neutral-500">{t('targets.noTMDBResults')}</p>}
            {!preview && searchResults.length > 0 && <div className="mt-5 grid max-h-[58vh] gap-3 overflow-y-auto sm:grid-cols-2">
              {searchResults.map((result) => <article key={result.id} className="flex gap-4 rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
                {result.poster_url ? <img className="h-32 w-20 shrink-0 rounded-lg object-cover" src={result.poster_url} alt="" /> : <span className="grid h-32 w-20 shrink-0 place-items-center rounded-lg bg-neutral-100 text-neutral-400 dark:bg-neutral-900"><Movie size={24} /></span>}
                <div className="min-w-0 flex-1"><h3 className="font-semibold">{result.title}</h3><p className="mt-1 text-xs text-neutral-500">{result.original_title || '—'} · {result.year || '—'} · TMDB {result.id}</p><p className="mt-2 line-clamp-3 text-xs text-neutral-600 dark:text-neutral-400">{result.overview || t('targets.noOverview')}</p><div className="mt-3 flex items-center justify-between gap-2"><Badge variant="outline">★ {result.vote_average.toFixed(1)}</Badge><Button size="sm" onClick={() => createPreview.mutate({ candidate: matchCandidate, tmdbId: result.id })}>{t('targets.selectMatch')}</Button></div></div>
              </article>)}
            </div>}
            {preview && <div className="mt-5 space-y-5">
              <div className="overflow-hidden rounded-2xl border border-neutral-200 dark:border-neutral-800">
                {preview.match.backdrop_url && <img className="h-44 w-full object-cover" src={preview.match.backdrop_url} alt="" />}
                <div className="grid gap-5 p-5 sm:grid-cols-[8rem_1fr]">{preview.match.poster_url ? <img className="w-32 rounded-xl object-cover" src={preview.match.poster_url} alt="" /> : <span className="grid h-48 w-32 place-items-center rounded-xl bg-neutral-100 dark:bg-neutral-900"><Movie size={28} /></span>}<div><div className="flex flex-wrap items-center gap-2"><h3 className="text-xl font-bold">{preview.match.title}</h3><Badge variant={preview.plan.ready ? 'soft' : 'outline'}>{t(preview.plan.ready ? 'targets.previewReady' : 'targets.previewBlocked')}</Badge><Badge variant="outline">{t('targets.readOnly')}</Badge></div><p className="mt-1 text-sm text-neutral-500">{preview.match.original_title} · {preview.match.year || '—'} · TMDB {preview.match.id} · ★ {preview.match.vote_average.toFixed(1)}</p><p className="mt-4 text-sm leading-6 text-neutral-700 dark:text-neutral-300">{preview.match.overview || t('targets.noOverview')}</p></div></div>
              </div>
              <div className="grid gap-4 lg:grid-cols-2">
                <div className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
                  <h3 className="font-semibold">{t('targets.renamePlan')}</h3>
                  <p className="mt-2 break-all rounded-lg bg-neutral-50 p-3 font-mono text-xs dark:bg-neutral-900">{preview.plan.proposed_directory_path}</p>
                  {preview.plan.proposed_directory_creates.length > 0 && <div className="mt-4"><h4 className="text-xs font-semibold uppercase tracking-wide text-neutral-500">{t('targets.directoryCreates')}</h4>{preview.plan.proposed_directory_creates.map((directory) => <p key={directory} className="mt-2 break-all font-mono text-xs text-emerald-700 dark:text-emerald-300">+ {directory}</p>)}</div>}
                  {preview.plan.proposed_directory_renames.length > 0 && <div className="mt-4"><h4 className="text-xs font-semibold uppercase tracking-wide text-neutral-500">{t('targets.directoryRenames')}</h4>{preview.plan.proposed_directory_renames.map((item) => <div key={`${item.source_path}-${item.target_path}`} className="mt-2 text-xs"><p className="break-all text-neutral-500">{item.source_path}</p><p className="break-all text-emerald-700 dark:text-emerald-300">→ {item.target_path}</p></div>)}</div>}
                  {preview.plan.proposed_file_renames.length > 0 && <div className="mt-4"><h4 className="text-xs font-semibold uppercase tracking-wide text-neutral-500">{t('targets.fileRenames')}</h4>{preview.plan.proposed_file_renames.map((item) => <div key={`${item.source_path}-${item.target_path}`} className="mt-2 text-xs"><p className="break-all text-neutral-500">{item.source_path}</p><p className="break-all text-emerald-700 dark:text-emerald-300">→ {item.target_path}</p></div>)}</div>}
                  {preview.plan.proposed_directory_creates.length === 0 && preview.plan.proposed_directory_renames.length === 0 && preview.plan.proposed_file_renames.length === 0 && <p className="mt-3 text-sm text-neutral-500">{t('targets.noRenames')}</p>}
                </div>
                <div className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800"><h3 className="font-semibold">{t('targets.generatedFiles')}</h3><ul className="mt-2 space-y-2">{preview.plan.generated_files.map((file) => <li key={file} className="break-all font-mono text-xs text-neutral-600 dark:text-neutral-400">{file}</li>)}</ul>{preview.plan.artifacts.filter((artifact) => (artifact.kind === 'nfo' || artifact.kind === 'episode_nfo') && artifact.content).map((artifact) => <details key={artifact.path} className="mt-4 rounded-lg bg-neutral-50 p-3 dark:bg-neutral-900"><summary className="cursor-pointer text-sm font-medium">{t('targets.nfoPreview')}</summary><pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-all text-xs text-neutral-600 dark:text-neutral-400">{artifact.content}</pre></details>)}</div>
              </div>
              {preview.plan.conflicts.length > 0 && <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-900 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200"><h3 className="font-semibold">{t('targets.previewConflicts')}</h3><ul className="mt-2 space-y-2">{preview.plan.conflicts.map((conflict, index) => <li key={`${conflict.code}-${index}`}><p>{t(`targets.conflicts.${conflict.code}`)}</p>{conflict.source_path && <p className="mt-1 break-all font-mono text-xs opacity-75">{conflict.source_path}{conflict.target_path ? ` → ${conflict.target_path}` : ''}</p>}</li>)}</ul></div>}
              {preview.plan.warnings.length > 0 && <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200"><h3 className="font-semibold">{t('targets.previewWarnings')}</h3><ul className="mt-2 list-disc space-y-1 pl-5">{preview.plan.warnings.map((warning) => <li key={warning}>{t(`targets.warnings.${warning}`)}</li>)}</ul></div>}
              <div className="flex flex-wrap items-center justify-between gap-3"><p className="text-xs text-neutral-500">{t('targets.previewExpires', { value: new Date(preview.expires_at).toLocaleString() })}</p><div className="flex gap-2"><Button variant="outline" onClick={() => setMatchCandidate(null)}>{t('common.close')}</Button><Button disabled={!preview.plan.ready || executeJob.isPending} onClick={() => executePreview(preview)}>{executeJob.isPending ? t('targets.submittingJob') : t('targets.execute')}</Button></div></div>
            </div>}
      </AppDialog>}
    </div>
  )
}
