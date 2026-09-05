import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { i18n } from '@/i18n'
import { SettingsPage } from './settings-page'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  save: vi.fn(),
  testTMDB: vi.fn(),
  testAI: vi.fn(),
}))

vi.mock('@/api/services', () => ({
  settingsApi: {
    scraping: mocks.get,
    saveScraping: mocks.save,
    testTMDB: mocks.testTMDB,
    testAI: mocks.testAI,
  },
}))

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
})

const settings = {
  has_api_key: true,
  api_key_mask: '***',
  base_url: 'https://api.themoviedb.org',
  image_base_url: 'https://image.tmdb.org',
  language: 'zh-CN',
  region: '',
  poster_size: 'w500',
  backdrop_size: 'w1280',
  timeout_seconds: 20,
  proxy_host: '',
  proxy_port: 0,
  ai_enabled: false,
  ai_has_api_key: false,
  ai_base_url: 'https://api.openai.com/v1',
  ai_model: 'gpt-4o-mini',
  ai_qpm_limit: 60,
  ai_timeout_seconds: 30,
}

beforeEach(async () => {
  vi.clearAllMocks()
  await i18n.changeLanguage('en')
  mocks.get.mockResolvedValue(settings)
  mocks.save.mockImplementation(async (input) => ({ ...settings, ...input }))
})

function renderSettings() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><SettingsPage /></QueryClientProvider>)
}

it('shows common metadata languages as named choices', async () => {
  renderSettings()

  expect(await screen.findByRole('combobox', { name: 'Metadata language' })).toHaveTextContent('Simplified Chinese (zh-CN)')
})

it('preserves and saves a custom metadata language', async () => {
  mocks.get.mockResolvedValue({ ...settings, language: 'pt-BR' })
  renderSettings()

  expect(await screen.findByRole('combobox', { name: 'Metadata language' })).toHaveTextContent('Custom language')
  const customInput = screen.getByDisplayValue('pt-BR')
  fireEvent.change(customInput, { target: { value: 'it-IT' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(mocks.save.mock.calls[0]?.[0]).toEqual(expect.objectContaining({ language: 'it-IT' })))
})
