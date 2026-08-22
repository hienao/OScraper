import { Globe } from '@appica/icons-react'
import { useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { AppSelect } from '@/components/common/app-select'
import { readLanguagePreference, subscribeLanguagePreference, writeLanguagePreference, type LanguagePreference } from '@/i18n/language-preference'

export function LanguageSwitcher() {
  const { t } = useTranslation()
  const preference = useSyncExternalStore(subscribeLanguagePreference, readLanguagePreference, () => 'auto')
  return <AppSelect value={preference} onValueChange={(value) => writeLanguagePreference(value as LanguagePreference)} ariaLabel={t('language.label')} className="w-30 border-0 bg-transparent text-xs shadow-none" startSlot={<Globe size={17} aria-hidden="true" />} options={[
    { value: 'auto', label: t('language.auto') },
    { value: 'en', label: t('language.english') },
    { value: 'zh-CN', label: t('language.chinese') },
  ]} />
}
