import { useEffect, useState } from 'react'
import { AgendaApiError, fetchAgenda, type AgendaResponse, type DateRange } from '@/lib/api'

// Matches the API's Cache-Control max-age, so a cached copy is never older than the server would serve.
const FRESH_MS = 5 * 60 * 1000

interface CachedAgenda {
  data: AgendaResponse
  loadedAt: number
}

const cache = new Map<string, CachedAgenda>()

interface QueryState {
  key: string
  status: 'loading' | 'ready' | 'error'
  data?: AgendaResponse
  error?: AgendaApiError
}

export type AgendaStatus = 'idle' | 'loading' | 'ready' | 'error'

export interface AgendaQuery {
  status: AgendaStatus
  data: AgendaResponse | undefined
  error: AgendaApiError | undefined
}

export function agendaKey(range?: DateRange): string {
  return range ? `${range.from}|${range.to}` : 'default'
}

function rangeFromKey(key: string): DateRange | undefined {
  if (key === 'default') return undefined
  const [from, to] = key.split('|')
  return { from, to }
}

function freshCopy(key: string): AgendaResponse | undefined {
  const entry = cache.get(key)
  if (!entry || Date.now() - entry.loadedAt >= FRESH_MS) return undefined
  return entry.data
}

/**
 * Loads the agenda for the default window (no range) or one date range.
 * A revision above zero means the user asked for a refresh, so the browser cache is bypassed.
 */
export function useAgenda(range: DateRange | undefined, revision: number, enabled = true): AgendaQuery {
  const key = agendaKey(range)
  const [state, setState] = useState<QueryState | null>(null)

  useEffect(() => {
    if (!enabled) return
    const reload = revision > 0
    if (!reload && freshCopy(key)) {
      setState({ key, status: 'ready', data: freshCopy(key) })
      return
    }

    const controller = new AbortController()
    const previous = cache.get(key)?.data
    // Keeps the previous error visible while a retry runs, so the "Try again" control keeps focus.
    setState((current) => ({
      key,
      status: 'loading',
      data: current?.key === key ? current.data : previous,
      error: current?.key === key ? current.error : undefined,
    }))

    fetchAgenda(rangeFromKey(key), { signal: controller.signal, reload })
      .then((data) => {
        if (controller.signal.aborted) return
        cache.set(key, { data, loadedAt: Date.now() })
        setState({ key, status: 'ready', data })
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        const apiError = error instanceof AgendaApiError ? error : new AgendaApiError(null)
        setState((current) => ({
          key,
          status: 'error',
          data: current?.key === key ? current.data : previous,
          error: apiError,
        }))
      })

    return () => controller.abort()
  }, [key, enabled, revision])

  if (!enabled) return { status: 'idle', data: undefined, error: undefined }

  const current = state?.key === key ? state : undefined
  if (!current) {
    const cached = freshCopy(key)
    return cached
      ? { status: 'ready', data: cached, error: undefined }
      : { status: 'loading', data: cache.get(key)?.data, error: undefined }
  }
  return { status: current.status, data: current.data, error: current.error }
}
