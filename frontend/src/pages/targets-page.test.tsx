import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { i18n } from '@/i18n'
import { TargetsPage } from './targets-page'

const mocks = vi.hoisted(() => ({
  targetList: vi.fn(),
  targetScan: vi.fn(),
  targetTree: vi.fn(),
  connectionList: vi.fn(),
  connectionTree: vi.fn(),
  localStatus: vi.fn(),
}))

vi.mock('@/api/services', () => ({
  targetApi: {
    list: mocks.targetList,
    create: vi.fn(), update: vi.fn(), remove: vi.fn(), tree: mocks.targetTree,
    scan: mocks.targetScan, scanResult: vi.fn(), candidates: vi.fn(),
  },
  connectionApi: { list: mocks.connectionList, tree: mocks.connectionTree },
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
  vi.clearAllMocks()
  await i18n.changeLanguage('zh-CN')
  mocks.targetList.mockResolvedValue([{
    id: 2, source_type: 'local', connection_name: '本地存储', name: '已删除目录', root_path: '/media/removed',
    library_type: 'movie', rename_enabled: false, enabled: true, created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  }])
  mocks.connectionList.mockResolvedValue([])
  mocks.localStatus.mockResolvedValue({ root: '/media', mounted: true, readable: true, writable: true, free_bytes: 1, total_bytes: 1, uid: 1000, gid: 1000, groups: [1000] })
  mocks.connectionTree.mockResolvedValue({ root_path: '/media', path: '/media', entries: [], warnings: [] })
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

it('shows the offending path separator without blocking safe directory browsing', async () => {
  const unsafeName = '02.哈利波特8部合集[兽组十年站庆第02弹] /英国台粤DTS:XMA7.1/导评音轨&视觉描述'
  mocks.connectionList.mockResolvedValue([{
    id: 1, name: '115', base_url: 'http://openlist.example', username: 'owner', base_path: '/media',
    qps_limit: 5, qpm_limit: 120, enabled: true, has_token: true, token_mask: '***', last_test_ok: true,
    created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
  }])
  mocks.connectionTree.mockResolvedValue({
    root_path: '/media', path: '/media',
    entries: [{ name: 'Movies', path: '/media/Movies', is_dir: true, size: 0 }],
    warnings: [{ code: 'openlist.unsafe_entry_skipped', name: unsafeName, reason: 'path_separator', invalid_character: '/' }],
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<MemoryRouter><QueryClientProvider client={client}><TargetsPage /></QueryClientProvider></MemoryRouter>)

  await waitFor(() => expect(client.getQueryData(['connections'])).toBeDefined())
  fireEvent.click(await screen.findByRole('button', { name: '添加目标' }))
  fireEvent.click(await screen.findByRole('button', { name: '选择' }))

  expect(await screen.findByText('已跳过 1 个条目：名称包含路径分隔符“/”。请在 OpenList 的 filename_char_mapping 中配置 {"/":"|"}，然后刷新目录。')).toBeInTheDocument()
  expect(screen.getByTitle(unsafeName)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Movies' })).toBeInTheDocument()
})
