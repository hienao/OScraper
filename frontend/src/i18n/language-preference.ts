export const LANGUAGE_STORAGE_KEY = 'openlist-scraper-language'
export type SupportedLanguage = 'en' | 'zh-CN'
export type LanguagePreference = 'auto' | SupportedLanguage
const preferenceEvent = 'openlist-scraper-language-change'

export function browserLanguage(languages = [...(globalThis.navigator?.languages ?? []), globalThis.navigator?.language ?? '']): SupportedLanguage {
  return languages.some((language) => language.toLowerCase().startsWith('zh')) ? 'zh-CN' : 'en'
}

export function readLanguagePreference(storage = globalThis.localStorage): LanguagePreference {
  const value = storage?.getItem(LANGUAGE_STORAGE_KEY)
  return value === 'auto' || value === 'en' || value === 'zh-CN' ? value : 'auto'
}

export function resolveLanguage(preference = readLanguagePreference()): SupportedLanguage {
  return preference === 'auto' ? browserLanguage() : preference
}

export function writeLanguagePreference(preference: LanguagePreference) {
  localStorage.setItem(LANGUAGE_STORAGE_KEY, preference)
  window.dispatchEvent(new Event(preferenceEvent))
}

export function subscribeLanguagePreference(listener: () => void) {
  const onStorage = (event: StorageEvent) => { if (event.key === LANGUAGE_STORAGE_KEY) listener() }
  window.addEventListener(preferenceEvent, listener)
  window.addEventListener('storage', onStorage)
  window.addEventListener('languagechange', listener)
  return () => {
    window.removeEventListener(preferenceEvent, listener)
    window.removeEventListener('storage', onStorage)
    window.removeEventListener('languagechange', listener)
  }
}
