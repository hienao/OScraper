import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { readLanguagePreference, resolveLanguage, subscribeLanguagePreference } from './language-preference'
import { resources } from './resources'

function applyLanguage() {
  const language = resolveLanguage(readLanguagePreference())
  document.documentElement.lang = language
  if (i18n.isInitialized && i18n.resolvedLanguage !== language) void i18n.changeLanguage(language)
}

void i18n.use(initReactI18next).init({
  resources,
  lng: resolveLanguage(),
  supportedLngs: ['en', 'zh-CN'],
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
  initAsync: false,
  react: { useSuspense: false },
})

applyLanguage()
subscribeLanguagePreference(applyLanguage)
export { i18n }
