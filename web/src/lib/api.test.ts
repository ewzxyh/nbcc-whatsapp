import { afterEach, describe, expect, it, vi } from 'vitest'
import { AGENDA_ERROR_MESSAGE, AgendaApiError, buildAgendaUrl, fetchAgenda } from './api'
import { fixtureResponse } from '../test/fixtures'

function stubFetch(impl: (url: string, init?: RequestInit) => Promise<Response>) {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('buildAgendaUrl', () => {
  it('uses the default window when no range is given', () => {
    expect(buildAgendaUrl()).toBe('/api/agenda')
  })

  it('passes from and to as query parameters', () => {
    expect(buildAgendaUrl({ from: '2026-10-01', to: '2026-10-31' })).toBe(
      '/api/agenda?from=2026-10-01&to=2026-10-31',
    )
  })
})

describe('fetchAgenda', () => {
  it('returns the parsed response on 200', async () => {
    const body = fixtureResponse()
    const fetchMock = stubFetch(async () => new Response(JSON.stringify(body), { status: 200 }))
    const result = await fetchAgenda({ from: '2026-09-29', to: '2027-01-11' })
    expect(result.items).toHaveLength(body.items.length)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/agenda?from=2026-09-29&to=2027-01-11')
  })

  it('throws a typed error with a generic message on a server error', async () => {
    stubFetch(async () => new Response('{"error":"NBCC feed is unavailable"}', { status: 502 }))
    const error = await fetchAgenda().catch((e: unknown) => e)
    expect(error).toBeInstanceOf(AgendaApiError)
    expect((error as AgendaApiError).status).toBe(502)
    expect((error as AgendaApiError).message).toBe(AGENDA_ERROR_MESSAGE)
    expect((error as AgendaApiError).message).not.toContain('NBCC feed')
  })

  it('throws a typed error when the network request fails', async () => {
    stubFetch(async () => {
      throw new TypeError('Failed to fetch')
    })
    const error = await fetchAgenda().catch((e: unknown) => e)
    expect(error).toBeInstanceOf(AgendaApiError)
    expect((error as AgendaApiError).status).toBeNull()
  })

  it('throws a typed error when the body has the wrong shape', async () => {
    stubFetch(async () => new Response('{"ok":true}', { status: 200 }))
    await expect(fetchAgenda()).rejects.toBeInstanceOf(AgendaApiError)
  })

  it('rethrows aborts so callers can ignore them', async () => {
    stubFetch(async () => {
      throw new DOMException('aborted', 'AbortError')
    })
    const error = await fetchAgenda().catch((e: unknown) => e)
    expect(error).not.toBeInstanceOf(AgendaApiError)
    expect((error as Error).name).toBe('AbortError')
  })

  it('asks the browser for a fresh copy when reloading', async () => {
    const fetchMock = stubFetch(async () => new Response(JSON.stringify(fixtureResponse()), { status: 200 }))
    await fetchAgenda(undefined, { reload: true })
    expect(fetchMock.mock.calls[0][1]?.cache).toBe('reload')
  })
})
