import { ArrowLeft, Check, FileText, Movie, Package, Plus, Refresh, Settings, Trash, X } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { connectionApi, previewApi, targetApi } from '@/api/services'
import type { LibraryType, MediaCandidate, ScanRun, ScrapePreview, ScrapeTarget, TargetInput, TMDBSearchResult } from '@/api/types'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

const emptyForm: TargetInput = { connection_id: 0, name: '', root_path: '/', library_type: 'movie', rename_enabled: false, enabled: true }

export function TargetsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const targets = useQuery({ queryKey: ['targets'], queryFn: targetApi.list })
  const connections = useQuery({ queryKey: ['connections'], queryFn: connectionApi.list })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ScrapeTarget | null>(null)
  const [form, setForm] = useState<TargetInput>(emptyForm)
  const [browsing, setBrowsing] = useState<ScrapeTarget | null>(null)
  const [browserPath, setBrowserPath] = useState('')
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

  function openCreate() {
    setEditing(null)
    setForm({ ...emptyForm, connection_id: connections.data?.[0]?.id ?? 0 })
    setNotice(null)
    setFormOpen(true)
  }
  function openEdit(target: ScrapeTarget) {
    setEditing(target)
    setForm({ connection_id: target.connection_id, name: target.name, root_path: target.root_path, library_type: target.library_type, rename_enabled: target.rename_enabled, enabled: target.enabled })
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

  const saving = create.isPending || update.isPending

  return (
    <div className="mx-auto max-w-7xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
      {notice && <Message variant={notice.variant}>{notice.text}</Message>}
      <Panel title={t('targets.title')} description={t('targets.description')} icon={<Movie size={20} />} action={<Button className="gap-2" onClick={openCreate} disabled={!connections.data?.length}><Plus size={17} />{t('targets.add')}</Button>}>
        {targets.isLoading && <p className="text-sm text-neutral-500">{t('common.loading')}</p>}
        {targets.error && <Message variant="error">{errorMessage(targets.error, t('errors.requestFailed'))}</Message>}
        {targets.data?.length === 0 && <div className="py-12 text-center"><span className="mx-auto grid size-12 place-items-center rounded-2xl bg-neutral-100 text-neutral-500 dark:bg-neutral-900"><Movie size={22} /></span><h2 className="mt-4 font-semibold">{t('targets.empty')}</h2><p className="mt-1 text-sm text-neutral-500">{t('targets.emptyDescription')}</p></div>}
        <div className="grid gap-4 lg:grid-cols-2">
          {targets.data?.map((target) => (
            <article key={target.id} className="rounded-2xl border border-neutral-200 p-5 dark:border-neutral-800">
              <div className="flex items-start justify-between gap-4"><div><h2 className="font-semibold">{target.name}</h2><p className="mt-1 text-sm text-neutral-500">{target.connection_name}</p></div><Badge variant={target.enabled ? 'soft' : 'outline'}>{t(target.enabled ? 'common.enabled' : 'common.disabled')}</Badge></div>
              <p className="mt-4 break-all rounded-xl bg-neutral-50 p-3 font-mono text-xs dark:bg-neutral-900">{target.root_path}</p>
              <div className="mt-3 flex flex-wrap gap-2 text-xs"><Badge variant="outline">{t(`targets.${target.library_type}`)}</Badge>{target.rename_enabled && <Badge variant="outline">{t('targets.rename')}</Badge>}</div>
              <div className="mt-5 flex flex-wrap gap-2">
                <Button size="sm" className="gap-2" disabled={!target.enabled || scan.isPending} onClick={() => startScan(target)}><Refresh size={15} />{scan.isPending && scanTarget?.id === target.id ? t('targets.scanning') : t('targets.scan')}</Button>
                <Button size="sm" variant="outline" className="gap-2" onClick={() => openBrowser(target)}><Package size={15} />{t('targets.browse')}</Button>
                <Button size="sm" variant="outline" className="gap-2" onClick={() => openEdit(target)}><Settings size={15} />{t('common.edit')}</Button>
                <Button size="sm" variant="outline" className="gap-2 text-red-700 dark:text-red-300" disabled={remove.isPending} onClick={() => { if (window.confirm(t('targets.deleteConfirm', { name: target.name }))) remove.mutate(target.id) }}><Trash size={15} />{t('common.remove')}</Button>
              </div>
            </article>
          ))}
        </div>
      </Panel>

      {formOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center overflow-y-auto bg-neutral-950/50 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="target-form-title">
          <form className="app-panel my-8 w-full max-w-xl p-6" onSubmit={(event) => void submit(event)}>
            <div className="flex items-start justify-between gap-4"><div><h2 id="target-form-title" className="text-xl font-bold">{t(editing ? 'targets.editTitle' : 'targets.createTitle')}</h2><p className="mt-1 text-sm text-neutral-500">{t('targets.createDescription')}</p></div><Button type="button" variant="ghost" size="icon-md" aria-label={t('common.close')} onClick={() => setFormOpen(false)}><X size={20} /></Button></div>
            <div className="mt-6 space-y-4">
              <FormField label={t('targets.name')}><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder={t('targets.placeholderName')} required maxLength={100} /></FormField>
              <FormField label={t('targets.connection')}><select className="h-10 w-full rounded-lg border border-neutral-300 bg-transparent px-3 text-sm dark:border-neutral-700" value={form.connection_id} onChange={(event) => setForm({ ...form, connection_id: Number(event.target.value) })} required>{connections.data?.map((connection) => <option key={connection.id} value={connection.id}>{connection.name}</option>)}</select></FormField>
              <FormField label={t('targets.rootPath')}><Input value={form.root_path} onChange={(event) => setForm({ ...form, root_path: event.target.value })} placeholder={t('targets.placeholderPath')} required /></FormField>
              <FormField label={t('targets.libraryType')}><select className="h-10 w-full rounded-lg border border-neutral-300 bg-transparent px-3 text-sm dark:border-neutral-700" value={form.library_type} onChange={(event) => setForm({ ...form, library_type: event.target.value as LibraryType })}>{(['movie', 'tv', 'anime'] as LibraryType[]).map((type) => <option key={type} value={type}>{t(`targets.${type}`)}</option>)}</select></FormField>
              <label className="flex items-start gap-3 rounded-xl border border-neutral-200 p-3 text-sm dark:border-neutral-800"><input className="mt-1" type="checkbox" checked={form.rename_enabled} onChange={(event) => setForm({ ...form, rename_enabled: event.target.checked })} /><span><strong>{t('targets.rename')}</strong><span className="mt-1 block text-xs text-neutral-500">{t('targets.renameWarning')}</span></span></label>
              {editing && <label className="flex items-center gap-3 rounded-xl border border-neutral-200 p-3 text-sm dark:border-neutral-800"><input type="checkbox" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} /><span>{t('targets.enabled')}</span></label>}
            </div>
            <div className="mt-6 flex justify-end gap-2"><Button type="button" variant="ghost" onClick={() => setFormOpen(false)}>{t('common.cancel')}</Button><Button type="submit" className="gap-2" disabled={saving || form.connection_id === 0}>{saving ? t('targets.saving') : <><Check size={16} />{t('common.save')}</>}</Button></div>
          </form>
        </div>
      )}

      {browsing && (
        <div className="fixed inset-0 z-50 grid place-items-center overflow-y-auto bg-neutral-950/50 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="target-browser-title">
          <div className="app-panel my-8 w-full max-w-3xl p-6">
            <div className="flex items-start justify-between gap-4"><div><h2 id="target-browser-title" className="text-xl font-bold">{t('targets.browserTitle')} · {browsing.name}</h2><p className="mt-1 break-all font-mono text-xs text-neutral-500">{browserPath}</p></div><Button variant="ghost" size="icon-md" aria-label={t('common.close')} onClick={() => setBrowsing(null)}><X size={20} /></Button></div>
            <div className="mt-5 flex gap-2"><Button size="sm" variant="outline" className="gap-2" disabled={browserPath === browsing.root_path} onClick={goUp}><ArrowLeft size={15} />{t('targets.up')}</Button><Button size="sm" variant="outline" className="gap-2" disabled={tree.isFetching || refreshTree.isPending} onClick={() => refreshTree.mutate()}><Refresh size={15} />{t('common.refresh')}</Button></div>
            <div className="mt-4 max-h-[60vh] overflow-y-auto rounded-xl border border-neutral-200 dark:border-neutral-800">
              {tree.isLoading && <p className="p-5 text-sm text-neutral-500">{t('common.loading')}</p>}
              {tree.error && <div className="p-4"><Message variant="error">{errorMessage(tree.error, t('targets.browserError'))}</Message></div>}
              {tree.data?.entries.length === 0 && <p className="p-8 text-center text-sm text-neutral-500">{t('targets.noEntries')}</p>}
              {tree.data?.entries.map((entry) => (
                <button key={entry.path} type="button" disabled={!entry.is_dir} onClick={() => entry.is_dir && setBrowserPath(entry.path)} className="flex w-full items-center gap-3 border-b border-neutral-200 px-4 py-3 text-left last:border-0 enabled:hover:bg-neutral-50 disabled:cursor-default dark:border-neutral-800 dark:enabled:hover:bg-neutral-900">
                  <span className={`grid size-9 shrink-0 place-items-center rounded-lg ${entry.is_dir ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-neutral-100 text-neutral-500 dark:bg-neutral-900'}`}>{entry.is_dir ? <Package size={17} /> : <FileText size={17} />}</span>
                  <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{entry.name}</span><span className="text-xs text-neutral-500">{t(entry.is_dir ? 'targets.directory' : 'targets.file')}</span></span>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {scanTarget && (
        <div className="fixed inset-0 z-50 grid place-items-center overflow-y-auto bg-neutral-950/50 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="target-scan-title">
          <div className="app-panel my-8 w-full max-w-4xl p-6">
            <div className="flex items-start justify-between gap-4">
              <div><h2 id="target-scan-title" className="text-xl font-bold">{t('targets.scanTitle')} · {scanTarget.name}</h2><p className="mt-1 text-sm text-neutral-500">{t('targets.scanDescription')}</p></div>
              <Button variant="ghost" size="icon-md" aria-label={t('common.close')} disabled={scan.isPending} onClick={() => setScanTarget(null)}><X size={20} /></Button>
            </div>
            {scan.isPending && <div className="grid min-h-48 place-items-center"><div className="text-center"><Refresh className="mx-auto animate-spin text-emerald-600" size={28} /><p className="mt-3 text-sm text-neutral-500">{t('targets.scanningDescription')}</p></div></div>}
            {scan.isError && <div className="mt-5"><Message variant="error">{errorMessage(scan.error, t('targets.scanError'))}</Message></div>}
            {scanResult && <>
              <div className="mt-5 grid gap-3 sm:grid-cols-3">
                <div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('targets.candidates')}</p><p className="mt-1 text-2xl font-bold">{scanResult.candidate_count}</p></div>
                <div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('targets.videoFiles')}</p><p className="mt-1 text-2xl font-bold">{scanResult.video_count}</p></div>
                <div className="rounded-xl bg-neutral-50 p-4 dark:bg-neutral-900"><p className="text-xs text-neutral-500">{t('targets.scanStatus')}</p><p className="mt-1 text-sm font-semibold text-emerald-700 dark:text-emerald-300">{t('targets.scanSucceeded')}</p></div>
              </div>
              {scanResult.candidates?.length === 0 && <p className="py-12 text-center text-sm text-neutral-500">{t('targets.noCandidates')}</p>}
              <div className="mt-4 max-h-[55vh] space-y-3 overflow-y-auto pr-1">
                {scanResult.candidates?.map((candidate) => <article key={candidate.id || candidate.path} className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800">
                  <div className="flex flex-wrap items-start justify-between gap-3"><div className="min-w-0"><h3 className="truncate font-semibold">{candidate.parsed_title || candidate.path.split('/').pop()}</h3><p className="mt-1 break-all font-mono text-xs text-neutral-500">{candidate.path}</p></div><Badge variant={candidate.status === 'ready' ? 'soft' : 'outline'}>{t(`targets.${candidate.status}`)}</Badge></div>
                  <div className="mt-3 flex flex-wrap gap-2 text-xs"><Badge variant="outline">{t(`targets.${candidate.kind}`)}</Badge>{candidate.year && <Badge variant="outline">{candidate.year}</Badge>}{candidate.season !== undefined && <Badge variant="outline">S{String(candidate.season).padStart(2, '0')}</Badge>}{candidate.episode !== undefined && <Badge variant="outline">E{String(candidate.episode).padStart(2, '0')}</Badge>}{candidate.tmdb_id && <Badge variant="outline">TMDB {candidate.tmdb_id}</Badge>}<Badge variant="outline">{t('targets.confidence', { value: candidate.confidence })}</Badge><Badge variant="outline">{t('targets.videoCount', { count: candidate.video_count })}</Badge></div>
                  <div className="mt-3"><Button size="sm" variant="outline" className="gap-2" onClick={() => startMatch(candidate)}><Movie size={15} />{t('targets.tmdbPreview')}</Button></div>
                </article>)}
              </div>
              <div className="mt-5 flex justify-end"><Button onClick={() => setScanTarget(null)}>{t('common.close')}</Button></div>
            </>}
          </div>
        </div>
      )}

      {matchCandidate && (
        <div className="fixed inset-0 z-[60] grid place-items-center overflow-y-auto bg-neutral-950/60 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="tmdb-preview-title">
          <div className="app-panel my-8 w-full max-w-5xl p-6">
            <div className="flex items-start justify-between gap-4"><div><h2 id="tmdb-preview-title" className="text-xl font-bold">{t('targets.tmdbPreview')}</h2><p className="mt-1 break-all font-mono text-xs text-neutral-500">{matchCandidate.path}</p></div><Button variant="ghost" size="icon-md" aria-label={t('common.close')} onClick={() => setMatchCandidate(null)}><X size={20} /></Button></div>
            <form className="mt-5 grid gap-3 sm:grid-cols-[1fr_8rem_auto]" onSubmit={runSearch}>
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
                <div className="rounded-xl border border-neutral-200 p-4 dark:border-neutral-800"><h3 className="font-semibold">{t('targets.generatedFiles')}</h3><ul className="mt-2 space-y-2">{preview.plan.generated_files.map((file) => <li key={file} className="break-all font-mono text-xs text-neutral-600 dark:text-neutral-400">{file}</li>)}</ul>{preview.plan.artifacts.filter((artifact) => artifact.kind === 'nfo' && artifact.content).map((artifact) => <details key={artifact.path} className="mt-4 rounded-lg bg-neutral-50 p-3 dark:bg-neutral-900"><summary className="cursor-pointer text-sm font-medium">{t('targets.nfoPreview')}</summary><pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-all text-xs text-neutral-600 dark:text-neutral-400">{artifact.content}</pre></details>)}</div>
              </div>
              {preview.plan.conflicts.length > 0 && <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-900 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200"><h3 className="font-semibold">{t('targets.previewConflicts')}</h3><ul className="mt-2 space-y-2">{preview.plan.conflicts.map((conflict, index) => <li key={`${conflict.code}-${index}`}><p>{t(`targets.conflicts.${conflict.code}`)}</p>{conflict.source_path && <p className="mt-1 break-all font-mono text-xs opacity-75">{conflict.source_path}{conflict.target_path ? ` → ${conflict.target_path}` : ''}</p>}</li>)}</ul></div>}
              {preview.plan.warnings.length > 0 && <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200"><h3 className="font-semibold">{t('targets.previewWarnings')}</h3><ul className="mt-2 list-disc space-y-1 pl-5">{preview.plan.warnings.map((warning) => <li key={warning}>{t(`targets.warnings.${warning}`)}</li>)}</ul></div>}
              <div className="flex flex-wrap items-center justify-between gap-3"><p className="text-xs text-neutral-500">{t('targets.previewExpires', { value: new Date(preview.expires_at).toLocaleString() })}</p><Button onClick={() => setMatchCandidate(null)}>{t('common.close')}</Button></div>
            </div>}
          </div>
        </div>
      )}
    </div>
  )
}
