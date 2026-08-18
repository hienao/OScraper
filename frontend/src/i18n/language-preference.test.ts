import { browserLanguage, readLanguagePreference, resolveLanguage } from './language-preference'
import { resources } from './resources'

function keys(value: object, prefix = ''): string[] {
  return Object.entries(value).flatMap(([key, child]) => typeof child === 'object' && child !== null ? keys(child as object, `${prefix}${key}.`) : [`${prefix}${key}`]).sort()
}

describe('language preference', () => {
  it('detects Chinese browser language', () => expect(browserLanguage(['zh-CN', 'en'])).toBe('zh-CN'))
  it('falls back to English', () => expect(browserLanguage(['fr-FR'])).toBe('en'))
  it('prefers explicit selection', () => expect(resolveLanguage('zh-CN')).toBe('zh-CN'))
  it('ignores invalid stored values', () => {
    const storage = { getItem: () => 'fr' } as unknown as Storage
    expect(readLanguagePreference(storage)).toBe('auto')
  })
  it('keeps English and Chinese translation keys in sync', () => expect(keys(resources.en.translation)).toEqual(keys(resources['zh-CN'].translation)))
})
