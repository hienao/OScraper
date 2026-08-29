import { Check, Refresh, Settings } from '@appica/icons-react'
import { Button } from '@appica/ui-react/button'
import { Input } from '@appica/ui-react/input'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { settingsApi } from '@/api/services'
import type { ScrapingSettingsInput } from '@/api/types'
import { AppSelect } from '@/components/common/app-select'
import { CheckboxField } from '@/components/common/checkbox-field'
import { FormField } from '@/components/common/form-field'
import { Message } from '@/components/common/message'
import { Panel } from '@/components/common/panel'
import { errorMessage } from '@/lib/error-message'

const customDomain = '__custom__'
const tmdbAPIDomains = ['https://api.themoviedb.org', 'https://api.tmdb.org']
const tmdbImageDomains = ['https://image.tmdb.org']

const defaults: ScrapingSettingsInput = {
  api_key: '', base_url: tmdbAPIDomains[0], image_base_url: tmdbImageDomains[0],
  language: 'zh-CN', region: '', poster_size: 'w500', backdrop_size: 'w1280', timeout_seconds: 20,
  proxy_host: '', proxy_port: 0, ai_enabled: false, ai_api_key: '', ai_base_url: 'https://api.openai.com/v1',
  ai_model: 'gpt-4o-mini', ai_qpm_limit: 60, ai_timeout_seconds: 30,
}

function domainChoice(value: string, choices: string[]) {
  return choices.includes(value) ? value : customDomain
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
      proxy_host: settings.data.proxy_host, proxy_port: settings.data.proxy_port,
      ai_enabled: settings.data.ai_enabled, ai_api_key: '', ai_base_url: settings.data.ai_base_url,
      ai_model: settings.data.ai_model, ai_qpm_limit: settings.data.ai_qpm_limit,
      ai_timeout_seconds: settings.data.ai_timeout_seconds,
    })
  }, [settings.data])

  const save = useMutation({
    mutationFn: settingsApi.saveScraping,
    onSuccess: async () => {
      setForm((current) => ({ ...current, api_key: '', ai_api_key: '' }))
      setNotice({ variant: 'success', text: t('settings.saved') })
      await queryClient.invalidateQueries({ queryKey: ['scraping-settings'] })
    },
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('settings.saveError')) }),
  })
  const testTMDB = useMutation({
    mutationFn: settingsApi.testTMDB,
    onSuccess: () => setNotice({ variant: 'success', text: t('settings.testPassed') }),
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('settings.testError')) }),
  })
  const testAI = useMutation({
    mutationFn: settingsApi.testAI,
    onSuccess: () => setNotice({ variant: 'success', text: t('settings.aiTestPassed') }),
    onError: (error) => setNotice({ variant: 'error', text: errorMessage(error, t('settings.aiTestError')) }),
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    setNotice(null)
    save.mutate({
      ...form,
      api_key: form.api_key.trim(), ai_api_key: form.ai_api_key.trim(),
      proxy_host: form.proxy_host.trim(), region: form.region.trim().toUpperCase(),
    })
  }

  const apiDomain = domainChoice(form.base_url, tmdbAPIDomains)
  const imageDomain = domainChoice(form.image_base_url, tmdbImageDomains)

  return (
    <div className="mx-auto max-w-5xl space-y-5 px-4 py-8 sm:px-6 lg:px-8">
      {notice && <Message variant={notice.variant}>{notice.text}</Message>}
      <Panel title={t('settings.title')} description={t('settings.description')} icon={<Settings size={20} />}>
        {settings.isLoading && <p className="text-sm text-neutral-500">{t('common.loading')}</p>}
        {settings.error && <Message variant="error">{errorMessage(settings.error, t('errors.requestFailed'))}</Message>}
        {settings.data && <form className="space-y-6" onSubmit={submit}>
          <section className="space-y-5 rounded-xl border border-neutral-200 p-4 dark:border-neutral-800" aria-labelledby="tmdb-settings-heading">
            <div><h2 id="tmdb-settings-heading" className="font-semibold text-neutral-950 dark:text-white">{t('settings.tmdbSection')}</h2><p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">{t('settings.tmdbSectionDescription')}</p></div>
            <div className="grid gap-5 md:grid-cols-2">
              <div className="md:col-span-2"><FormField label={t('settings.apiKey')} description={settings.data.has_api_key ? t('settings.apiKeySaved') : t('settings.apiKeyMissing')}><Input type="password" value={form.api_key} onChange={(event) => setForm({ ...form, api_key: event.target.value })} placeholder={settings.data.api_key_mask || t('settings.apiKeyPlaceholder')} autoComplete="new-password" /></FormField></div>
              <FormField label={t('settings.baseUrl')} description={t('settings.domainHint')}><AppSelect value={apiDomain} onValueChange={(value) => setForm({ ...form, base_url: value === customDomain ? '' : value })} ariaLabel={t('settings.baseUrl')} options={[...tmdbAPIDomains.map((value) => ({ value, label: value })), { value: customDomain, label: t('settings.customDomain') }]} /></FormField>
              <FormField label={t('settings.imageBaseUrl')} description={t('settings.domainHint')}><AppSelect value={imageDomain} onValueChange={(value) => setForm({ ...form, image_base_url: value === customDomain ? '' : value })} ariaLabel={t('settings.imageBaseUrl')} options={[...tmdbImageDomains.map((value) => ({ value, label: value })), { value: customDomain, label: t('settings.customDomain') }]} /></FormField>
              {apiDomain === customDomain && <FormField label={t('settings.customAPIUrl')}><Input type="url" value={form.base_url} onChange={(event) => setForm({ ...form, base_url: event.target.value })} required /></FormField>}
              {imageDomain === customDomain && <FormField label={t('settings.customImageUrl')}><Input type="url" value={form.image_base_url} onChange={(event) => setForm({ ...form, image_base_url: event.target.value })} required /></FormField>}
              <FormField label={t('settings.language')} description={t('settings.languageHint')}><Input value={form.language} onChange={(event) => setForm({ ...form, language: event.target.value })} placeholder="zh-CN" required /></FormField>
              <FormField label={t('settings.region')} description={t('settings.regionHint')}><Input value={form.region} onChange={(event) => setForm({ ...form, region: event.target.value.toUpperCase() })} placeholder="CN" maxLength={2} /></FormField>
              <FormField label={t('settings.posterSize')}><AppSelect value={form.poster_size} onValueChange={(poster_size) => setForm({ ...form, poster_size })} ariaLabel={t('settings.posterSize')} options={['w342', 'w500', 'w780', 'original'].map((value) => ({ value, label: value }))} /></FormField>
              <FormField label={t('settings.backdropSize')}><AppSelect value={form.backdrop_size} onValueChange={(backdrop_size) => setForm({ ...form, backdrop_size })} ariaLabel={t('settings.backdropSize')} options={['w780', 'w1280', 'original'].map((value) => ({ value, label: value }))} /></FormField>
              <FormField label={t('settings.timeout')}><Input type="number" min={1} max={120} value={form.timeout_seconds} onChange={(event) => setForm({ ...form, timeout_seconds: Number(event.target.value) })} required /></FormField>
            </div>
            <div className="flex justify-end"><Button type="button" variant="outline" className="gap-2" disabled={testTMDB.isPending || !settings.data.has_api_key} onClick={() => testTMDB.mutate()}><Refresh size={16} />{testTMDB.isPending ? t('settings.testing') : t('settings.testTMDB')}</Button></div>
          </section>

          <section className="space-y-5 rounded-xl border border-neutral-200 p-4 dark:border-neutral-800" aria-labelledby="network-settings-heading">
            <div><h2 id="network-settings-heading" className="font-semibold text-neutral-950 dark:text-white">{t('settings.proxySection')}</h2><p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">{t('settings.proxyDescription')}</p></div>
            <div className="grid gap-5 md:grid-cols-2">
              <FormField label={t('settings.proxyHost')}><Input value={form.proxy_host} onChange={(event) => setForm({ ...form, proxy_host: event.target.value })} placeholder="127.0.0.1" /></FormField>
              <FormField label={t('settings.proxyPort')}><Input type="number" min={0} max={65535} value={form.proxy_port || ''} onChange={(event) => setForm({ ...form, proxy_port: Number(event.target.value) })} placeholder="7890" /></FormField>
            </div>
          </section>

          <section className="space-y-5 rounded-xl border border-neutral-200 p-4 dark:border-neutral-800" aria-labelledby="ai-settings-heading">
            <div><h2 id="ai-settings-heading" className="font-semibold text-neutral-950 dark:text-white">{t('settings.aiSection')}</h2><p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">{t('settings.aiDescription')}</p></div>
            <CheckboxField checked={form.ai_enabled} onCheckedChange={(ai_enabled) => setForm({ ...form, ai_enabled })} label={t('settings.aiEnabled')} description={t('settings.aiEnabledHint')} />
            {form.ai_enabled && <div className="grid gap-5 md:grid-cols-2">
              <div className="md:col-span-2"><FormField label={t('settings.aiApiKey')} description={settings.data.ai_has_api_key ? t('settings.aiApiKeySaved') : t('settings.aiApiKeyMissing')}><Input type="password" value={form.ai_api_key} onChange={(event) => setForm({ ...form, ai_api_key: event.target.value })} placeholder={settings.data.ai_api_key_mask || t('settings.aiApiKeyPlaceholder')} autoComplete="new-password" /></FormField></div>
              <FormField label={t('settings.aiBaseUrl')}><Input type="url" value={form.ai_base_url} onChange={(event) => setForm({ ...form, ai_base_url: event.target.value })} required /></FormField>
              <FormField label={t('settings.aiModel')}><Input value={form.ai_model} onChange={(event) => setForm({ ...form, ai_model: event.target.value })} required /></FormField>
              <FormField label={t('settings.aiQPM')}><Input type="number" min={1} max={1000} value={form.ai_qpm_limit} onChange={(event) => setForm({ ...form, ai_qpm_limit: Number(event.target.value) })} required /></FormField>
              <FormField label={t('settings.aiTimeout')}><Input type="number" min={1} max={120} value={form.ai_timeout_seconds} onChange={(event) => setForm({ ...form, ai_timeout_seconds: Number(event.target.value) })} required /></FormField>
            </div>}
            <div className="flex justify-end"><Button type="button" variant="outline" className="gap-2" disabled={testAI.isPending || !settings.data.ai_enabled || !settings.data.ai_has_api_key} onClick={() => testAI.mutate()}><Refresh size={16} />{testAI.isPending ? t('settings.testing') : t('settings.testAI')}</Button></div>
          </section>

          <div className="flex justify-end"><Button type="submit" className="gap-2" disabled={save.isPending}>{save.isPending ? t('settings.saving') : <><Check size={16} />{t('common.save')}</>}</Button></div>
        </form>}
      </Panel>
    </div>
  )
}
