import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { i18n } from '@/i18n'
import { TargetsPage } from './targets-page'

const mocks = vi.hoisted(() => ({
  targetList: vi.fn(),
  targetScan: vi.fn(),
  connectionList: vi.fn(),
  localStatus: vi.fn(),
}))

vi.mock('@/api/services', () => ({
  targetApi: {
    list: mocks.targetList,
    create: vi.fn(), update: vi.fn(), remove: vi.fn(), tree: vi.fn(),
    scan: mocks.targetScan, scanResult: vi.fn(), candidates: vi.fn(),
  },
  connectionApi: { list: mocks.connectionList },
  localStorageApi: { status: mocks.localStatus, tree: vi.fn() },
  previewApi: { search: vi.fn(), create: vi.fn() },
  jobApi: { submit: vi.fn() },
}))

beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false, media: query, onchange: null,
      addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn(),
    })),
  })
})

beforeEach(async () => {
  await i18n.changeLanguage('zh-CN')
  mocks.targetList.mockResolvedValue([{
    id: 2, source_type: 'local', connection_name: '本地存储', name: '已删除目录', root_path: '/media/removed',
    library_type: 'movie', rename_enabled: false, enabled: true, created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  }])
  mocks.connectionList.mockResolvedValue([])
  mocks.localStatus.mockResolvedValue({ root: '/media', mounted: true, readable: true, writable: true, free_bytes: 1, total_bytes: 1, uid: 1000, gid: 1000, groups: [1000] })
  mocks.targetScan.mockResolvedValue({
    id: 8, target_id: 2, status: 'failed', candidate_count: 0, video_count: 0, scraped_candidate_count: 0,
    error_code: 'local.path_unavailable', error_message: 'The directory does not exist. Please select a directory again', created_at: new Date().toISOString(),
  })
})

it('translates a stored missing-directory scan failure into actionable text', async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<MemoryRouter><QueryClientProvider client={client}><TargetsPage /></QueryClientProvider></MemoryRouter>)

  fireEvent.click(await screen.findByRole('button', { name: '扫描媒体' }))

  expect(await screen.findByText('目录不存在，请重新选择目录。')).toBeInTheDocument()
})
