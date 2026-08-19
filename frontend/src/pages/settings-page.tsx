import { Check, Refresh, Settings } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { settingsApi } from '@/api/services'
import type { ScrapingSettingsInput } from '@/api/types'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

const defaults: ScrapingSettingsInput = {
  api_key: '', base_url: 'https://api.themoviedb.org', image_base_url: 'https://image.tmdb.org',
  language: 'zh-CN', region: '', poster_size: 'w500', backdrop_size: 'w1280', timeout_seconds: 20,
}

export function SettingsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const settings = useQuery({ queryKey: ['scraping-settings'], queryFn: settingsApi.scraping })
  const [form, setForm] = useState<ScrapingSettingsInput>(defaults)
  const [notice, setNotice] = useState<{ variant: 'error' | 'success'; text: string } | null>(null)

  useEffect(() => {
    if (!settings.data) return
    setForm({
      api_key: '', base_url: settings.data.base_url, image_base_url: settings.data.image_base_url,
      language: settings.data.language, region: settings.data.region, poster_size: settings.data.poster_size,
      backdrop_size: settings.data.backdrop_size, timeout_seconds: settings.data.timeout_seconds,
    })
  }, [settings.data])

  const save = useMutation({
    mutationFn: settingsApi.saveScraping,
    onSuccess: async () => {
      setForm((current) => ({ ...current, api_key: '' }))
      setNotice({ variant: 'success', text: t('settings.saved') })
      await queryClient.invalidateQueries({ queryKey: ['scraping-settings'] })
    },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('settings.saveError')) }),
  })
  const test = useMutation({
    mutationFn: settingsApi.testTMDB,
    onSuccess: () => setNotice({ variant: 'success', text: t('settings.testPassed') }),
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('settings.testError')) }),
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    setNotice(null)
    save.mutate({ ...form, api_key: form.api_key.trim(), region: form.region.trim().toUpperCase() })
  }

  return (
    <div className="mx-auto max-w-5xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
      {notice && <Message variant={notice.variant}>{notice.text}</Message>}
      <Panel title={t('settings.title')} description={t('settings.description')} icon={<Settings size={20} />}>
        {settings.isLoading && <p className="text-sm text-neutral-500">{t('common.loading')}</p>}
        {settings.error && <Message variant="error">{errorMessage(settings.error, t('errors.requestFailed'))}</Message>}
        {settings.data && <form className="space-y-6" onSubmit={submit}>
          <div className="grid gap-5 md:grid-cols-2">
            <div className="md:col-span-2"><FormField label={t('settings.apiKey')} description={settings.data.has_api_key ? t('settings.apiKeySaved') : t('settings.apiKeyMissing')}><Input type="password" value={form.api_key} onChange={(event) => setForm({ ...form, api_key: event.target.value })} placeholder={settings.data.api_key_mask || t('settings.apiKeyPlaceholder')} autoComplete="new-password" /></FormField></div>
            <FormField label={t('settings.baseUrl')}><Input type="url" value={form.base_url} onChange={(event) => setForm({ ...form, base_url: event.target.value })} required /></FormField>
            <FormField label={t('settings.imageBaseUrl')}><Input type="url" value={form.image_base_url} onChange={(event) => setForm({ ...form, image_base_url: event.target.value })} required /></FormField>
            <FormField label={t('settings.language')} description={t('settings.languageHint')}><Input value={form.language} onChange={(event) => setForm({ ...form, language: event.target.value })} placeholder="zh-CN" required /></FormField>
            <FormField label={t('settings.region')} description={t('settings.regionHint')}><Input value={form.region} onChange={(event) => setForm({ ...form, region: event.target.value.toUpperCase() })} placeholder="CN" maxLength={2} /></FormField>
            <FormField label={t('settings.posterSize')}><select className="h-10 w-full rounded-lg border border-neutral-300 bg-transparent px-3 text-sm dark:border-neutral-700" value={form.poster_size} onChange={(event) => setForm({ ...form, poster_size: event.target.value })}><option value="w342">w342</option><option value="w500">w500</option><option value="w780">w780</option><option value="original">original</option></select></FormField>
            <FormField label={t('settings.backdropSize')}><select className="h-10 w-full rounded-lg border border-neutral-300 bg-transparent px-3 text-sm dark:border-neutral-700" value={form.backdrop_size} onChange={(event) => setForm({ ...form, backdrop_size: event.target.value })}><option value="w780">w780</option><option value="w1280">w1280</option><option value="original">original</option></select></FormField>
            <FormField label={t('settings.timeout')}><Input type="number" min={1} max={120} value={form.timeout_seconds} onChange={(event) => setForm({ ...form, timeout_seconds: Number(event.target.value) })} required /></FormField>
          </div>
          <div className="flex flex-wrap justify-end gap-2">
            <Button type="button" variant="outline" className="gap-2" disabled={test.isPending || !settings.data.has_api_key} onClick={() => test.mutate()}><Refresh size={16} />{test.isPending ? t('settings.testing') : t('settings.test')}</Button>
            <Button type="submit" className="gap-2" disabled={save.isPending}>{save.isPending ? t('settings.saving') : <><Check size={16} />{t('common.save')}</>}</Button>
          </div>
        </form>}
      </Panel>
    </div>
  )
}
