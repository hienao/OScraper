import { apiRequest } from './client'
import type { ConnectionTestResult, CreateConnectionInput, DirectoryLevel, MediaCandidate, OpenListConnection, ScanRun, ScrapePreview, ScrapingSettings, ScrapingSettingsInput, ScrapeTarget, TargetInput, TMDBSearchResult, TokenResponse, UpdateConnectionInput, User } from './types'

export const authApi = {
  login: (body: { username: string; password: string }) => apiRequest<TokenResponse>('/api/auth/login', { method: 'POST', body, auth: false }),
  profile: () => apiRequest<User>('/api/user/profile'),
  logout: () => apiRequest<void>('/api/auth/logout', { method: 'POST' }),
  setupAdmin: (body: { username: string; password: string }) => apiRequest<TokenResponse>('/api/auth/setup-admin', { method: 'POST', body }),
}

export const connectionApi = {
  list: () => apiRequest<OpenListConnection[]>('/api/openlist-connections'),
  create: (body: CreateConnectionInput) => apiRequest<OpenListConnection>('/api/openlist-connections', { method: 'POST', body }),
  update: (id: number, body: UpdateConnectionInput) => apiRequest<OpenListConnection>(`/api/openlist-connections/${id}`, { method: 'PUT', body }),
  remove: (id: number) => apiRequest<void>(`/api/openlist-connections/${id}`, { method: 'DELETE' }),
  test: (id: number) => apiRequest<ConnectionTestResult>(`/api/openlist-connections/${id}/test`, { method: 'POST' }),
  rotateToken: (id: number, token: string) => apiRequest<OpenListConnection>(`/api/openlist-connections/${id}/rotate-token`, { method: 'POST', body: { token } }),
}

export const targetApi = {
  list: () => apiRequest<ScrapeTarget[]>('/api/scrape-targets'),
  create: (body: TargetInput) => apiRequest<ScrapeTarget>('/api/scrape-targets', { method: 'POST', body }),
  update: (id: number, body: TargetInput) => apiRequest<ScrapeTarget>(`/api/scrape-targets/${id}`, { method: 'PUT', body }),
  remove: (id: number) => apiRequest<void>(`/api/scrape-targets/${id}`, { method: 'DELETE' }),
  tree: (id: number, path?: string, refresh = false) => {
    const query = new URLSearchParams()
    if (path) query.set('path', path)
    if (refresh) query.set('refresh', 'true')
    return apiRequest<DirectoryLevel>(`/api/scrape-targets/${id}/tree?${query.toString()}`)
  },
  scan: (id: number, refresh = true) => apiRequest<ScanRun>(`/api/scrape-targets/${id}/scans?refresh=${refresh}`, { method: 'POST' }),
  scanResult: (id: number, scanId: number) => apiRequest<ScanRun>(`/api/scrape-targets/${id}/scans/${scanId}`),
  candidates: (id: number, scanId?: number) => apiRequest<MediaCandidate[]>(`/api/scrape-targets/${id}/candidates${scanId ? `?scan_id=${scanId}` : ''}`),
}

export const settingsApi = {
  scraping: () => apiRequest<ScrapingSettings>('/api/settings/scraping'),
  saveScraping: (body: ScrapingSettingsInput) => apiRequest<ScrapingSettings>('/api/settings/scraping', { method: 'PUT', body }),
  testTMDB: () => apiRequest<{ ok: boolean }>('/api/settings/scraping/test-tmdb', { method: 'POST' }),
}

export const previewApi = {
  search: (targetId: number, body: { candidate_id: number; title?: string; year?: number }) => apiRequest<TMDBSearchResult[]>(`/api/scrape-targets/${targetId}/previews/search`, { method: 'POST', body }),
  create: (targetId: number, body: { candidate_id: number; tmdb_id?: number; title?: string; year?: number }) => apiRequest<ScrapePreview>(`/api/scrape-targets/${targetId}/previews`, { method: 'POST', body }),
  get: (targetId: number, previewId: number) => apiRequest<ScrapePreview>(`/api/scrape-targets/${targetId}/previews/${previewId}`),
}
