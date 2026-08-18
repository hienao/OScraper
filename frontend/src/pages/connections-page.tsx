import { Check, Plus, Refresh, Settings, Trash, X } from '@appica/icons-react'
import { Badge } from '@appica/ui-react/badge'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { connectionApi } from '@/api/services'
import type { CreateConnectionInput, OpenListConnection, UpdateConnectionInput } from '@/api/types'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

interface ConnectionFormState {
  name: string
  base_url: string
  token: string
  qps_limit: number
  qpm_limit: number
  enabled: boolean
}

const emptyForm: ConnectionFormState = { name: '', base_url: '', token: '', qps_limit: 5, qpm_limit: 120, enabled: true }

export function ConnectionsPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const connections = useQuery({ queryKey: ['connections'], queryFn: connectionApi.list })
  const [editing, setEditing] = useState<OpenListConnection | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [form, setForm] = useState<ConnectionFormState>(emptyForm)
  const [notice, setNotice] = useState<{ variant: 'error' | 'success'; text: string } | null>(null)

  const create = useMutation({ mutationFn: (input: CreateConnectionInput) => connectionApi.create(input) })
  const update = useMutation({ mutationFn: ({ id, input }: { id: number; input: UpdateConnectionInput }) => connectionApi.update(id, input) })
  const rotate = useMutation({ mutationFn: ({ id, token }: { id: number; token: string }) => connectionApi.rotateToken(id, token) })
  const test = useMutation({
    mutationFn: connectionApi.test,
    onSuccess: async () => { setNotice({ variant: 'success', text: t('connections.testPassed') }); await queryClient.invalidateQueries({ queryKey: ['connections'] }) },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('connections.testError')) }),
  })
  const remove = useMutation({
    mutationFn: connectionApi.remove,
    onSuccess: async () => { setNotice({ variant: 'success', text: t('connections.deleted') }); await queryClient.invalidateQueries({ queryKey: ['connections'] }) },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('connections.deleteError')) }),
  })

  function openCreate() { setEditing(null); setForm(emptyForm); setNotice(null); setFormOpen(true) }
  function openEdit(connection: OpenListConnection) {
    setEditing(connection)
    setForm({ name: connection.name, base_url: connection.base_url, token: '', qps_limit: connection.qps_limit, qpm_limit: connection.qpm_limit, enabled: connection.enabled })
    setNotice(null)
    setFormOpen(true)
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setNotice(null)
    try {
      if (editing) {
        await update.mutateAsync({ id: editing.id, input: { name: form.name.trim(), base_url: form.base_url.trim(), qps_limit: form.qps_limit, qpm_limit: form.qpm_limit, enabled: form.enabled } })
        if (form.token.trim()) await rotate.mutateAsync({ id: editing.id, token: form.token.trim() })
      } else {
        await create.mutateAsync({ name: form.name.trim(), base_url: form.base_url.trim(), token: form.token.trim(), qps_limit: form.qps_limit, qpm_limit: form.qpm_limit })
      }
      await queryClient.invalidateQueries({ queryKey: ['connections'] })
      setFormOpen(false)
      setNotice({ variant: 'success', text: form.token.trim() && editing ? t('connections.tokenRotated') : t('connections.saved') })
    } catch (error) {
      setNotice({ variant: 'error', text: errorMessage(error, t('connections.formError')) })
    }
  }

  const saving = create.isPending || update.isPending || rotate.isPending
  const formatter = new Intl.DateTimeFormat(i18n.resolvedLanguage ?? 'en', { dateStyle: 'medium', timeStyle: 'short' })

  return (
    <div className="mx-auto max-w-7xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
      {notice && <Message variant={notice.variant}>{notice.text}</Message>}
      <Panel title={t('connections.title')} description={t('connections.description')} icon={<Settings size={20} />} action={<Button className="gap-2" onClick={openCreate}><Plus size={17} />{t('connections.add')}</Button>}>
        {connections.isLoading && <p className="text-sm text-neutral-500">{t('common.loading')}</p>}
        {connections.error && <Message variant="error">{errorMessage(connections.error, t('errors.requestFailed'))}</Message>}
        {connections.data?.length === 0 && <div className="py-12 text-center"><span className="mx-auto grid size-12 place-items-center rounded-2xl bg-neutral-100 text-neutral-500 dark:bg-neutral-900"><Settings size={22} /></span><h2 className="mt-4 font-semibold">{t('connections.empty')}</h2><p className="mt-1 text-sm text-neutral-500">{t('connections.emptyDescription')}</p></div>}
        {Boolean(connections.data?.length) && (
          <div className="grid gap-4 lg:grid-cols-2">
            {connections.data?.map((connection) => (
              <article key={connection.id} className="rounded-2xl border border-neutral-200 p-5 dark:border-neutral-800">
                <div className="flex items-start justify-between gap-4"><div><h2 className="font-semibold">{connection.name}</h2><p className="mt-1 break-all text-sm text-neutral-500">{connection.base_url}</p></div><Badge variant={connection.last_test_ok ? 'soft' : 'outline'}>{t(connection.last_test_ok ? 'common.healthy' : 'common.untested')}</Badge></div>
                <dl className="mt-5 grid gap-3 text-sm sm:grid-cols-2">
                  <div><dt className="text-neutral-500">{t('connections.account')}</dt><dd className="mt-0.5 font-medium">{connection.username || '—'}</dd></div>
                  <div><dt className="text-neutral-500">{t('connections.basePath')}</dt><dd className="mt-0.5 break-all font-mono text-xs">{connection.base_path}</dd></div>
                  <div><dt className="text-neutral-500">QPS / QPM</dt><dd className="mt-0.5 font-medium">{connection.qps_limit} / {connection.qpm_limit}</dd></div>
                  <div><dt className="text-neutral-500">{t('connections.lastTest')}</dt><dd className="mt-0.5 font-medium">{connection.last_tested_at ? formatter.format(new Date(connection.last_tested_at)) : '—'}</dd></div>
                </dl>
                <div className="mt-5 flex flex-wrap gap-2">
                  <Button size="sm" variant="outline" className="gap-2" disabled={test.isPending && test.variables === connection.id} onClick={() => test.mutate(connection.id)}><Refresh size={15} />{t(test.isPending && test.variables === connection.id ? 'connections.testing' : 'common.test')}</Button>
                  <Button size="sm" variant="outline" className="gap-2" onClick={() => openEdit(connection)}><Settings size={15} />{t('common.edit')}</Button>
                  <Button size="sm" variant="outline" className="gap-2 text-red-700 dark:text-red-300" disabled={remove.isPending} onClick={() => { if (window.confirm(t('connections.deleteConfirm', { name: connection.name }))) remove.mutate(connection.id) }}><Trash size={15} />{t('common.remove')}</Button>
                </div>
              </article>
            ))}
          </div>
        )}
      </Panel>

      {formOpen && (
        <div className="fixed inset-0 z-50 grid place-items-center overflow-y-auto bg-neutral-950/50 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-labelledby="connection-form-title">
          <form className="app-panel my-8 w-full max-w-xl p-6" onSubmit={(event) => void submit(event)}>
            <div className="flex items-start justify-between gap-4"><div><h2 id="connection-form-title" className="text-xl font-bold">{t(editing ? 'connections.editTitle' : 'connections.createTitle')}</h2><p className="mt-1 text-sm text-neutral-500">{t(editing ? 'connections.editDescription' : 'connections.createDescription')}</p></div><Button type="button" variant="ghost" size="icon-md" aria-label={t('common.close')} onClick={() => setFormOpen(false)}><X size={20} /></Button></div>
            <div className="mt-6 space-y-4">
              <FormField label={t('connections.name')}><Input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder={t('connections.placeholderName')} required maxLength={100} /></FormField>
              <FormField label={t('connections.baseUrl')}><Input type="url" value={form.base_url} onChange={(event) => setForm({ ...form, base_url: event.target.value })} placeholder={t('connections.placeholderUrl')} required /></FormField>
              <FormField label={t(editing ? 'connections.tokenOptional' : 'connections.token')}><Input type="password" value={form.token} onChange={(event) => setForm({ ...form, token: event.target.value })} required={!editing} autoComplete="new-password" /></FormField>
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField label={t('connections.qps')}><Input type="number" min={0} max={1000} value={form.qps_limit} onChange={(event) => setForm({ ...form, qps_limit: Number(event.target.value) })} required /></FormField>
                <FormField label={t('connections.qpm')}><Input type="number" min={0} max={60000} value={form.qpm_limit} onChange={(event) => setForm({ ...form, qpm_limit: Number(event.target.value) })} required /></FormField>
              </div>
              {editing && <label className="flex items-center gap-3 rounded-xl border border-neutral-200 p-3 text-sm dark:border-neutral-800"><input type="checkbox" checked={form.enabled} onChange={(event) => setForm({ ...form, enabled: event.target.checked })} /><span>{t(form.enabled ? 'common.enabled' : 'common.disabled')}</span></label>}
            </div>
            <div className="mt-6 flex justify-end gap-2"><Button type="button" variant="ghost" onClick={() => setFormOpen(false)}>{t('common.cancel')}</Button><Button type="submit" className="gap-2" disabled={saving}>{saving ? t('connections.saving') : <><Check size={16} />{t('common.save')}</>}</Button></div>
          </form>
        </div>
      )}
    </div>
  )
}
