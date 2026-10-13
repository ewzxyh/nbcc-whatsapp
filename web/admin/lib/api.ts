export const API_BASE = '/api/admin'

export type HttpMethod = 'GET' | 'POST' | 'PUT'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly retryAfterSeconds: number | null

  constructor(
    status: number,
    code: string,
    fields: Record<string, string> = {},
    retryAfterSeconds: number | null = null,
  ) {
    // The message is the machine code only. Server text is never shown or logged.
    super(code)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
    this.retryAfterSeconds = retryAfterSeconds
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError
}

export function isSessionError(error: unknown): boolean {
  return isApiError(error) && (error.code === 'unauthenticated' || error.code === 'session_expired')
}

interface RequestOptions {
  method?: HttpMethod
  body?: unknown
  csrfToken?: string | null
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function parseRetryAfter(value: string | null): number | null {
  if (!value) return null
  const seconds = Number(value)
  return Number.isFinite(seconds) && seconds >= 0 ? seconds : null
}

// Calls /api/admin/<path>. The CSRF header is sent on every non-GET request when a token is known.
export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, csrfToken } = options
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET' && csrfToken) headers['X-CSRF-Token'] = csrfToken

  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
      cache: 'no-store',
    })
  } catch {
    throw new ApiError(0, 'network')
  }

  if (response.status === 204) return undefined as T

  let data: unknown = null
  try {
    data = await response.json()
  } catch {
    data = null
  }

  if (!response.ok) {
    const code = isRecord(data) && typeof data.error === 'string' ? data.error : 'unknown'
    const fields: Record<string, string> = {}
    if (isRecord(data) && isRecord(data.fields)) {
      for (const [key, value] of Object.entries(data.fields)) {
        if (typeof value === 'string') fields[key] = value
      }
    }
    throw new ApiError(response.status, code, fields, parseRetryAfter(response.headers.get('Retry-After')))
  }

  return data as T
}
