import type { ApiResponse } from './types'
import { readAccessToken } from '@/lib/auth-token'
import { i18n } from '@/i18n'

export class ApiError extends Error {
  constructor(message: string, public readonly status: number, public readonly code = -1, public readonly errorCode?: string) {
    super(message)
    this.name = 'ApiError'
  }
}

type RequestOptions = Omit<RequestInit, 'body'> & { body?: unknown; auth?: boolean }

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { auth = true, ...requestOptions } = options
  const headers = new Headers(options.headers)
  const token = readAccessToken()
  if (auth && token && !headers.has('Authorization')) headers.set('Authorization', `Bearer ${token}`)
  if (options.body !== undefined && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

  const response = await fetch(path, {
    ...requestOptions,
    headers,
    credentials: 'omit',
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })

  let payload: ApiResponse<T>
  try {
    payload = await response.json() as ApiResponse<T>
  } catch {
    throw new ApiError(i18n.t('errors.invalidResponse'), response.status)
  }
  if (!response.ok || payload.code !== 0) {
    const translated = payload.error_code ? i18n.t(`errors.codes.${payload.error_code}`, { defaultValue: '' }) : ''
    throw new ApiError(translated || payload.message || i18n.t('errors.requestFailed'), response.status, payload.code, payload.error_code)
  }
  return payload.data as T
}
