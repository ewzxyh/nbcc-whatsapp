export type ItemType = 'activity' | 'event'
export type Phase = 'available' | 'due' | 'closes'

export interface AgendaItem {
  id: string
  type: ItemType
  phase: Phase | null
  title: string
  course: string | null
  courseTitle: string | null
  instructor: string | null
  activityId: string | null
  location: string | null
  allDay: boolean
  date: string
  startAt: string | null
  endAt: string | null
  endDate: string | null
}

export interface AgendaResponse {
  timezone: string
  today: string
  from: string
  to: string
  updatedAt: string
  /** Campus events skipped in this response's window because their details are not supported. */
  omittedEvents: number
  items: AgendaItem[]
}

export interface DateRange {
  from: string
  to: string
}

export const AGENDA_ERROR_MESSAGE = 'The agenda could not be loaded right now.'

export class AgendaApiError extends Error {
  readonly status: number | null

  constructor(status: number | null) {
    super(AGENDA_ERROR_MESSAGE)
    this.name = 'AgendaApiError'
    this.status = status
  }
}

export function buildAgendaUrl(range?: DateRange): string {
  if (!range) return '/api/agenda'
  const params = new URLSearchParams({ from: range.from, to: range.to })
  return `/api/agenda?${params.toString()}`
}

function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

function isAgendaResponse(value: unknown): value is AgendaResponse {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<AgendaResponse>
  return (
    typeof candidate.today === 'string' &&
    typeof candidate.updatedAt === 'string' &&
    Array.isArray(candidate.items)
  )
}

export interface FetchAgendaOptions {
  signal?: AbortSignal
  reload?: boolean
}

export async function fetchAgenda(
  range?: DateRange,
  options: FetchAgendaOptions = {},
): Promise<AgendaResponse> {
  let response: Response
  try {
    response = await fetch(buildAgendaUrl(range), {
      headers: { Accept: 'application/json' },
      cache: options.reload ? 'reload' : 'default',
      signal: options.signal,
    })
  } catch (error) {
    if (isAbortError(error)) throw error
    throw new AgendaApiError(null)
  }

  if (!response.ok) throw new AgendaApiError(response.status)

  let body: unknown
  try {
    body = await response.json()
  } catch {
    throw new AgendaApiError(response.status)
  }
  if (!isAgendaResponse(body)) throw new AgendaApiError(response.status)
  const omittedEvents = typeof body.omittedEvents === 'number' ? body.omittedEvents : 0
  return { ...body, omittedEvents }
}
