import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import '@/i18n'
import { LogsPage } from './logs-page'

const mocks = vi.hoisted(() => ({
  api: vi.fn(),
  application: vi.fn(),
  audit: vi.fn(),
  settings: vi.fn(),
  saveSettings: vi.fn(),
  clear: vi.fn(),
}))

vi.mock('@/api/services', () => ({ logApi: mocks }))

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
  mocks.api.mockResolvedValue({ items: [{ id: 1, request_id: 'request-1', occurred_at: new Date().toISOString(), method: 'GET', route: '/api/非常深的目录/An exceptionally long English media directory name/tree', status_code: 200, latency_ms: 12 }], total: 1, page: 1, size: 100 })
  mocks.application.mockResolvedValue({ items: [], total: 0, page: 1, size: 100 })
  mocks.audit.mockResolvedValue({ items: [], total: 0, page: 1, size: 100 })
  mocks.settings.mockResolvedValue({ retention_days: 7 })
})

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><LogsPage /></QueryClientProvider>)
}

it('opens controlled retention and clear dialogs from the log toolbar', async () => {
  renderPage()
  expect((await screen.findAllByText('/api/非常深的目录/An exceptionally long English media directory name/tree')).length).toBe(2)

  fireEvent.click(screen.getByRole('button', { name: 'Retention' }))
  expect(await screen.findByRole('heading', { name: 'Log retention' })).toBeInTheDocument()
  expect(screen.getByRole('combobox', { name: 'Keep logs for' })).toHaveTextContent('7 days')

  fireEvent.click(screen.getByRole('button', { name: 'Close' }))
  fireEvent.click(screen.getByRole('button', { name: 'Clear logs' }))
  expect(await screen.findByRole('heading', { name: 'Clear logs manually' })).toBeInTheDocument()
  expect(screen.getByRole('combobox', { name: 'Logs to clear' })).toHaveTextContent('API logs')
})
