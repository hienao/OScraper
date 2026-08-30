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
export type SourceType = 'openlist' | 'local'

export interface ScrapeTarget {
  id: number
  source_type: SourceType
  connection_id?: number
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
  source_type: SourceType
  connection_id?: number
  name: string
  root_path: string
  library_type: LibraryType
  rename_enabled: boolean
  enabled: boolean
}

export interface LocalStorageStatus {
  root: string
  mounted: boolean
  readable: boolean
  writable: boolean
  free_bytes: number
  total_bytes: number
  uid: number
  gid: number
  groups: number[]
}

export interface DirectoryNode {
  name: string
  path: string
  is_dir: boolean
  size: number
  modified?: string
}

export interface DirectoryWarning {
  code: 'openlist.unsafe_entry_skipped'
  name: string
  reason: 'empty_name' | 'dot_segment' | 'path_separator' | 'control_character'
  invalid_character?: string
}

export interface DirectoryLevel {
  target_id: number
  root_path: string
  path: string
  entries: DirectoryNode[]
  warnings?: DirectoryWarning[]
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
  scraped: boolean
  status: 'ready' | 'needs_review'
  created_at: string
}

export interface ScanRun {
  id: number
  target_id: number
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  candidate_count: number
  video_count: number
  scraped_candidate_count: number
  error_code?: string
  error_message?: string
  started_at?: string
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
  proxy_host: string
  proxy_port: number
  ai_enabled: boolean
  ai_has_api_key: boolean
  ai_api_key_mask?: string
  ai_base_url: string
  ai_model: string
  ai_qpm_limit: number
  ai_timeout_seconds: number
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
  proxy_host: string
  proxy_port: number
  ai_enabled: boolean
  ai_api_key: string
  ai_base_url: string
  ai_model: string
  ai_qpm_limit: number
  ai_timeout_seconds: number
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
  scrape_marker_path: string
  proposed_directory_name: string
  proposed_directory_path: string
  proposed_directory_creates: string[]
  proposed_directory_renames: RenameItem[]
  proposed_file_renames: RenameItem[]
  generated_files: string[]
  artifacts: { path: string; kind: 'nfo' | 'poster' | 'backdrop' | 'episode_nfo' | 'episode_thumb'; source_url?: string; content?: string }[]
  episode_files: { source_path: string; target_path: string; season: number; episode: number }[]
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

export type JobStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'canceled'

export interface JobRecordSettings {
  retention_days: number
}

export interface ScrapeJob {
  id: number
  target_id: number
  preview_id: number
  candidate_id: number
  connection_id: number
  actor_id: number
  status: JobStatus
  stage: 'preparing' | 'renaming' | 'generating' | 'uploading' | 'verifying' | 'marking' | 'completed'
  progress: number
  message?: string
  error_code?: string
  error_message?: string
  checkpoint: number
  attempts: number
  started_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface ScrapeJobOperation {
  id: number
  job_id: number
  sequence: number
  type: 'mkdir' | 'rename' | 'move' | 'upload' | 'marker'
  source_path?: string
  target_path: string
  artifact_kind?: string
  content_type?: string
  status: 'pending' | 'running' | 'succeeded' | 'skipped' | 'failed'
  attempts: number
  last_error?: string
  started_at?: string
  completed_at?: string
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  size: number
}

export interface APIRequestLog {
  id: number; request_id: string; occurred_at: string; method: string; route: string; status_code: number; latency_ms: number; username?: string; target_id?: number; job_id?: number; error_code?: string
}
export interface ApplicationLog {
  id: number; occurred_at: string; level: string; source: string; message: string; fields?: string; request_id?: string; target_id?: number; job_id?: number
}
export interface AuditLog {
  id: number; actor_id: number; action: string; target: string; detail: string; occurred_at: string
}

export type LogType = 'api' | 'application' | 'audit' | 'all'

export interface LogSettings {
  retention_days: number
}

export interface LogCleanupStats {
  api: number
  application: number
  audit: number
}
