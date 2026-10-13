import type { AgendaItem, Phase } from './api'
import { addDays, MONCTON_TIME_ZONE } from './agenda'

const MINUTE_MS = 60_000
const HOUR_MS = 60 * MINUTE_MS
const DAY_MS = 24 * HOUR_MS

/** How far back the "time left" meter looks when an assessment has no opening date. */
export const DEFAULT_WINDOW_MS = 14 * DAY_MS
export const PROGRESS_SEGMENTS = 5

const wallClockFormat = new Intl.DateTimeFormat('en-CA', {
  timeZone: MONCTON_TIME_ZONE,
  hourCycle: 'h23',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

/** The wall-clock reading of an instant in Saint John, expressed as if it were UTC. */
function wallClockAsUtc(instant: number): number {
  const parts: Record<string, number> = {}
  for (const part of wallClockFormat.formatToParts(new Date(instant))) {
    if (part.type !== 'literal') parts[part.type] = Number(part.value)
  }
  return Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour % 24, parts.minute)
}

/** The instant of a Saint John wall-clock time on a YYYY-MM-DD date, honouring daylight saving time. */
export function zonedInstant(date: string, hour: number, minute: number): number {
  const [year, month, day] = date.split('-').map(Number)
  const guess = Date.UTC(year, month - 1, day, hour, minute)
  return guess - (wallClockAsUtc(guess) - guess)
}

/**
 * The instant a phase takes effect. Timed items use their start time. All-day items open at the start of
 * the day, and due or closing items at the end of the day (midnight at the close of that date).
 */
export function itemInstant(item: AgendaItem): number {
  if (item.startAt && !item.allDay) return Date.parse(item.startAt)
  const endOfDay = item.type === 'activity' && item.phase !== 'available'
  return endOfDay ? zonedInstant(addDays(item.date, 1), 0, 0) : zonedInstant(item.date, 0, 0)
}

/** The opening instant of an assessment, from its "available" phase when one is loaded. */
export function openingInstant(phases: readonly AgendaItem[]): number | null {
  const opening = phases.find((phase) => phase.phase === 'available')
  return opening ? itemInstant(opening) : null
}

/** Readable duration such as "45 min", "3 h 20 min" or "2 d 4 h". */
export function durationText(ms: number): string {
  const minutes = Math.max(1, Math.floor(ms / MINUTE_MS))
  if (minutes < 60) return `${minutes} min`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `${hours} h ${minutes % 60} min`
  return `${Math.floor(hours / 24)} d ${hours % 24} h`
}

const UPCOMING_LABEL: Record<Phase | 'event', string> = {
  available: 'Opens in',
  due: 'Due in',
  closes: 'Closes in',
  event: 'Starts in',
}

const PASSED_LABEL: Record<Phase | 'event', string> = {
  available: 'Open now',
  due: 'Past due',
  closes: 'Closed',
  event: 'Started',
}

export interface Countdown {
  text: string
  passed: boolean
}

/** Countdown for one item, such as "Due in 2 d 4 h" or "Past due". */
export function countdownFor(item: AgendaItem, now: number): Countdown {
  const key = item.type === 'event' ? 'event' : (item.phase ?? 'due')
  const remaining = itemInstant(item) - now
  if (remaining <= 0) return { text: PASSED_LABEL[key], passed: true }
  return { text: `${UPCOMING_LABEL[key]} ${durationText(remaining)}`, passed: false }
}

/**
 * Share of the window still left before the due instant, from 0 to 1.
 * The window starts at the opening instant, or DEFAULT_WINDOW_MS before the due instant when there is none.
 */
export function timeLeftShare(dueAt: number, now: number, openedAt: number | null): number {
  const start = openedAt ?? dueAt - DEFAULT_WINDOW_MS
  const span = dueAt - start
  if (span <= 0) return 0
  return Math.min(1, Math.max(0, (dueAt - now) / span))
}

/** Lit segments of the meter: at least one while time is left, none once it has passed. */
export function litSegments(share: number): number {
  if (share <= 0) return 0
  return Math.min(PROGRESS_SEGMENTS, Math.max(1, Math.ceil(share * PROGRESS_SEGMENTS)))
}

/** Opening instant for every assessment that has an "available" phase in the loaded items. */
export function openingsByActivity(items: readonly AgendaItem[]): Map<string, number> {
  const openings = new Map<string, number>()
  for (const item of items) {
    if (item.activityId && item.phase === 'available') openings.set(item.activityId, itemInstant(item))
  }
  return openings
}
