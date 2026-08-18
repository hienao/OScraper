import { browserLanguage, readLanguagePreference, resolveLanguage } from './language-preference'

describe('language preference', () => {
  it('detects Chinese browser language', () => expect(browserLanguage(['zh-CN', 'en'])).toBe('zh-CN'))
  it('falls back to English', () => expect(browserLanguage(['fr-FR'])).toBe('en'))
  it('prefers explicit selection', () => expect(resolveLanguage('zh-CN')).toBe('zh-CN'))
  it('ignores invalid stored values', () => {
    const storage = { getItem: () => 'fr' } as unknown as Storage
    expect(readLanguagePreference(storage)).toBe('auto')
  })
})
