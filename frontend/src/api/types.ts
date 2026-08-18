export interface ApiResponse<T> {
  code: number
  error_code?: string
  message: string
  data?: T
}

export interface User {
  id: number
  username: string
  is_admin: boolean
  requires_admin_setup: boolean
  created_at: string
}

export interface TokenResponse {
  token: string
  expires_at: number
}

export interface OpenListConnection {
  id: number
  name: string
  base_url: string
  username: string
  base_path: string
  qps_limit: number
  qpm_limit: number
  enabled: boolean
  has_token: boolean
  token_mask: string
  last_tested_at?: string
  last_test_ok: boolean
  created_at: string
  updated_at: string
}

export interface ConnectionTestResult {
  ok: boolean
  username: string
  base_path: string
}

export interface CreateConnectionInput {
  name: string
  base_url: string
  token: string
  qps_limit: number
  qpm_limit: number
}

export interface UpdateConnectionInput {
  name: string
  base_url: string
  qps_limit: number
  qpm_limit: number
  enabled: boolean
}

export type LibraryType = 'movie' | 'tv' | 'anime'

export interface ScrapeTarget {
  id: number
  connection_id: number
  connection_name: string
  name: string
  root_path: string
  library_type: LibraryType
  rename_enabled: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface TargetInput {
  connection_id: number
  name: string
  root_path: string
  library_type: LibraryType
  rename_enabled: boolean
  enabled: boolean
}

export interface DirectoryNode {
  name: string
  path: string
  is_dir: boolean
  size: number
  modified?: string
}

export interface DirectoryLevel {
  target_id: number
  root_path: string
  path: string
  entries: DirectoryNode[]
}

export interface MediaCandidate {
  id: number
  scan_id: number
  target_id: number
  path: string
  kind: LibraryType
  fingerprint: string
  representative_file: string
  parsed_title: string
  year?: number
  season?: number
  episode?: number
  tmdb_id?: number
  confidence: number
  video_count: number
  status: 'ready' | 'needs_review'
  created_at: string
}

export interface ScanRun {
  id: number
  target_id: number
  status: 'running' | 'succeeded' | 'failed'
  candidate_count: number
  video_count: number
  error_code?: string
  error_message?: string
  started_at: string
  completed_at?: string
  created_at: string
  candidates?: MediaCandidate[]
}

export interface ScrapingSettings {
  has_api_key: boolean
  api_key_mask?: string
  base_url: string
  image_base_url: string
  language: string
  region: string
  poster_size: string
  backdrop_size: string
  timeout_seconds: number
}

export interface ScrapingSettingsInput {
  api_key: string
  base_url: string
  image_base_url: string
  language: string
  region: string
  poster_size: string
  backdrop_size: string
  timeout_seconds: number
}

export interface TMDBSearchResult {
  id: number
  media_type: 'movie' | 'tv'
  title: string
  original_title: string
  year?: number
  overview: string
  poster_url?: string
  backdrop_url?: string
  vote_average: number
  vote_count: number
  popularity: number
}

export interface TMDBDetail extends TMDBSearchResult {
  release_date?: string
  genres: { id: number; name: string }[]
  runtime?: number
  number_of_seasons?: number
  number_of_episodes?: number
  original_language?: string
  tagline?: string
  status?: string
  imdb_id?: string
  country?: string
  studios: string[]
}

export interface RenameItem {
  source_path: string
  target_path: string
  asset_type: string
}

export interface PreviewPlan {
  read_only: boolean
  ready: boolean
  rename_allowed: boolean
  organize_flat_movie: boolean
  source_path: string
  proposed_directory_name: string
  proposed_directory_path: string
  proposed_directory_creates: string[]
  proposed_directory_renames: RenameItem[]
  proposed_file_renames: RenameItem[]
  generated_files: string[]
  artifacts: { path: string; kind: 'nfo' | 'poster' | 'backdrop'; source_url?: string; content?: string }[]
  warnings: string[]
  conflicts: { code: string; source_path?: string; target_path?: string }[]
}

export interface ScrapePreview {
  id: number
  target_id: number
  candidate_id: number
  fingerprint: string
  match: TMDBDetail
  plan: PreviewPlan
  expires_at: string
  created_at: string
}
