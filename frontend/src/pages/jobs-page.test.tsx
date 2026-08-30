import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import '@/i18n'
import { JobsPage } from './jobs-page'

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  settings: vi.fn(),
  saveSettings: vi.fn(),
  get: vi.fn(),
  operations: vi.fn(),
  retry: vi.fn(),
  cancel: vi.fn(),
}))

vi.mock('@/api/services', () => ({ jobApi: mocks }))

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false, media: query, onchange: null,
      addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn(),
    })),
  })
})

beforeEach(() => {
  mocks.list.mockResolvedValue({ items: [], total: 0, page: 1, size: 50 })
  mocks.settings.mockResolvedValue({ retention_days: 7 })
})

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><JobsPage /></QueryClientProvider>)
}

it('opens the job record retention dialog with the seven-day default', async () => {
  renderPage()
  expect(await screen.findByText('No scrape jobs yet.')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Retention' }))

  expect(await screen.findByRole('heading', { name: 'Job record retention' })).toBeInTheDocument()
  expect(screen.getByRole('combobox', { name: 'Keep job records for' })).toHaveTextContent('7 days')
})
