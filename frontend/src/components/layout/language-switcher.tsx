import { Globe } from '@appica/icons-react'
import { useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { readLanguagePreference, subscribeLanguagePreference, writeLanguagePreference, type LanguagePreference } from '@/i18n/language-preference'

export function LanguageSwitcher() {
  const { t } = useTranslation()
  const preference = useSyncExternalStore(subscribeLanguagePreference, readLanguagePreference, () => 'auto')
  return (
    <label className="relative flex items-center gap-1.5 rounded-lg px-2 text-neutral-600 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:bg-neutral-900">
      <Globe size={17} aria-hidden="true" />
      <span className="sr-only">{t('language.label')}</span>
      <select className="h-9 max-w-28 bg-transparent text-xs font-medium outline-none" aria-label={t('language.label')} value={preference} onChange={(event) => writeLanguagePreference(event.target.value as LanguagePreference)}>
        <option value="auto">{t('language.auto')}</option>
        <option value="en">{t('language.english')}</option>
        <option value="zh-CN">{t('language.chinese')}</option>
      </select>
    </label>
  )
}
