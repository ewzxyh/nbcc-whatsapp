import type { AgendaItem, DateRange, Phase } from './api'

export const MONCTON_TIME_ZONE = 'America/Moncton'
export const TIMEZONE_NOTE = 'Times: Saint John, NB (Atlantic Time)'
export const CAMPUS_EVENT_LABEL = 'Campus event'

export type TimingBucket = 'today' | 'next7' | 'after7' | 'past'

export const TIMING_TITLES: Record<TimingBucket, string> = {
  today: 'Today',
  next7: 'Next 7 days',
  after7: 'After 7 days',
  past: 'Past',
}

export interface ActivityEntry {
  kind: 'activity'
  key: string
  date: string
  sortAt: number
  title: string
  activityId: string | null
  course: string | null
  courseTitle: string | null
  instructor: string | null
  phases: AgendaItem[]
}

export interface EventEntry {
  kind: 'event'
  key: string
  date: string
  sortAt: number
  title: string
  item: AgendaItem
}

export type AgendaEntry = ActivityEntry | EventEntry

export interface AgendaFilters {
  query: string
  subjects: readonly string[]
  instructors: readonly string[]
  showCampus: boolean
  showPast: boolean
}

export interface SubjectFacet {
  code: string
  title: string | null
  count: number
}

export interface ProfessorFacet {
  name: string
  count: number
}

export interface DayGroup {
  date: string
  entries: AgendaEntry[]
}

export interface BoardColumn {
  id: string
  title: string
  subtitle: string | null
  entries: AgendaEntry[]
}

const DAY_MS = 86_400_000
const MINUTE_MS = 60_000
const PHASE_RANK: Record<Phase, number> = { available: 0, due: 1, closes: 2 }

const pad2 = (value: number) => String(value).padStart(2, '0')

export function dayNumber(date: string): number {
  const [year, month, day] = date.split('-').map(Number)
  return Date.UTC(year, month - 1, day) / DAY_MS
}

function isoFromDayNumber(day: number): string {
  const value = new Date(day * DAY_MS)
  return `${value.getUTCFullYear()}-${pad2(value.getUTCMonth() + 1)}-${pad2(value.getUTCDate())}`
}

export function addDays(date: string, days: number): string {
  return isoFromDayNumber(dayNumber(date) + days)
}

export function daysBetween(from: string, to: string): number {
  return dayNumber(to) - dayNumber(from)
}

export function monthOf(date: string): string {
  return date.slice(0, 7)
}

export function shiftMonth(month: string, delta: number): string {
  const [year, monthIndex] = month.split('-').map(Number)
  const value = new Date(Date.UTC(year, monthIndex - 1 + delta, 1))
  return `${value.getUTCFullYear()}-${pad2(value.getUTCMonth() + 1)}`
}

export function monthRange(month: string): DateRange {
  const [year, monthIndex] = month.split('-').map(Number)
  const lastDay = new Date(Date.UTC(year, monthIndex, 0)).getUTCDate()
  return { from: `${month}-01`, to: `${month}-${pad2(lastDay)}` }
}

/** Weeks start on Sunday. Cells outside the month are null. */
export function monthGrid(month: string): (string | null)[][] {
  const [year, monthIndex] = month.split('-').map(Number)
  const leading = new Date(Date.UTC(year, monthIndex - 1, 1)).getUTCDay()
  const days = new Date(Date.UTC(year, monthIndex, 0)).getUTCDate()
  const cells: (string | null)[] = Array.from({ length: leading }, () => null)
  for (let day = 1; day <= days; day += 1) cells.push(`${month}-${pad2(day)}`)
  while (cells.length % 7 !== 0) cells.push(null)
  const weeks: (string | null)[][] = []
  for (let index = 0; index < cells.length; index += 7) weeks.push(cells.slice(index, index + 7))
  return weeks
}

const timeFormat = new Intl.DateTimeFormat('en-US', {
  timeZone: MONCTON_TIME_ZONE,
  hour: 'numeric',
  minute: '2-digit',
})
const mondayDateFormat = new Intl.DateTimeFormat('en-CA', {
  timeZone: MONCTON_TIME_ZONE,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})
const longDateFormat = new Intl.DateTimeFormat('en-US', {
  timeZone: 'UTC',
  weekday: 'long',
  month: 'long',
  day: 'numeric',
  year: 'numeric',
})
const shortDayFormat = new Intl.DateTimeFormat('en-US', {
  timeZone: 'UTC',
  weekday: 'short',
  month: 'short',
  day: 'numeric',
})
const monthDayFormat = new Intl.DateTimeFormat('en-US', {
  timeZone: 'UTC',
  month: 'short',
  day: 'numeric',
})
const monthTitleFormat = new Intl.DateTimeFormat('en-US', {
  timeZone: 'UTC',
  month: 'long',
  year: 'numeric',
})

const NON_BREAKING_SPACE = /[  ]/g
const cleanSpaces = (text: string) => text.replace(NON_BREAKING_SPACE, ' ')
const dateAnchor = (date: string) => new Date(dayNumber(date) * DAY_MS)

/** Clock time of an instant in Saint John, e.g. "11:59 PM". */
export function formatTime(instant: string): string {
  return cleanSpaces(timeFormat.format(new Date(instant)))
}

/** Calendar date (YYYY-MM-DD) of an instant in Saint John. */
export function monctonDate(instant: string): string {
  return mondayDateFormat.format(new Date(instant))
}

export function formatLongDate(date: string): string {
  return longDateFormat.format(dateAnchor(date))
}

export function formatShortDay(date: string): string {
  return cleanSpaces(shortDayFormat.format(dateAnchor(date)))
}

export function formatMonthDay(date: string): string {
  return cleanSpaces(monthDayFormat.format(dateAnchor(date)))
}

export function formatMonthTitle(month: string): string {
  return monthTitleFormat.format(dateAnchor(`${month}-01`))
}

export function formatDayLabel(date: string, today: string): string {
  const diff = daysBetween(today, date)
  if (diff === 0) return 'Today'
  if (diff === 1) return 'Tomorrow'
  return formatShortDay(date)
}

export function formatCount(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? '' : 's'}`
}

export const OMITTED_EVENTS_NOTICE =
  'Some campus events could not be shown because their details are not in a supported format. All course deadlines are still listed.'

/** The notice text when campus events were skipped in the window, otherwise null. */
export function omittedEventsNotice(omittedEvents: number | undefined): string | null {
  return omittedEvents !== undefined && omittedEvents > 0 ? OMITTED_EVENTS_NOTICE : null
}

export function formatUpdated(updatedAt: string, now: number): string {
  const updated = Date.parse(updatedAt)
  if (Number.isNaN(updated)) return 'Updated time unknown'
  const minutes = Math.floor((now - updated) / MINUTE_MS)
  if (minutes < 1) return 'Updated just now'
  if (minutes < 60) return `Updated ${minutes} min ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `Updated ${hours} ${hours === 1 ? 'hour' : 'hours'} ago`
  return `Updated ${formatShortDay(monctonDate(updatedAt))} at ${formatTime(updatedAt)}`
}

export function formatTimeRange(startAt: string, endAt: string | null): string {
  return endAt ? `${formatTime(startAt)} – ${formatTime(endAt)}` : formatTime(startAt)
}

/** Time part for an item that is not an activity phase: a range, "All day", or "All day until <date>". */
export function formatEventTime(item: AgendaItem): string {
  if (item.allDay || !item.startAt) {
    return item.endDate && item.endDate !== item.date
      ? `All day until ${formatMonthDay(item.endDate)}`
      : 'All day'
  }
  return formatTimeRange(item.startAt, item.endAt)
}

export function phaseLabel(phase: Phase | null): string {
  switch (phase) {
    case 'available':
      return 'Opens'
    case 'due':
      return 'Due'
    case 'closes':
      return 'Access closes'
    default:
      return 'Scheduled'
  }
}

/** Phase line such as "Due 11:59 PM"; the date is added when it differs from the card's day. */
export function describePhase(item: AgendaItem, cardDate: string): string {
  const day = item.date === cardDate ? '' : `${formatMonthDay(item.date)}, `
  const time = item.allDay || !item.startAt ? 'all day' : formatTime(item.startAt)
  return `${phaseLabel(item.phase)} ${day}${time}`
}

export function timingBucket(date: string, today: string): TimingBucket {
  const diff = daysBetween(today, date)
  if (diff < 0) return 'past'
  if (diff === 0) return 'today'
  if (diff <= 7) return 'next7'
  return 'after7'
}

function compareNumbers(a: number, b: number): number {
  return a < b ? -1 : a > b ? 1 : 0
}

function compareText(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0
}

function sortAtOf(item: AgendaItem): number {
  if (item.allDay || !item.startAt) return Number.NEGATIVE_INFINITY
  return Date.parse(item.startAt)
}

export function sortPhases(items: readonly AgendaItem[]): AgendaItem[] {
  return [...items].sort(
    (a, b) =>
      PHASE_RANK[a.phase ?? 'due'] - PHASE_RANK[b.phase ?? 'due'] ||
      compareNumbers(sortAtOf(a), sortAtOf(b)),
  )
}

function compareEntries(a: AgendaEntry, b: AgendaEntry): number {
  if (a.date !== b.date) return compareText(a.date, b.date)
  return compareNumbers(a.sortAt, b.sortAt) || compareText(a.title, b.title)
}

function activityEntry(key: string, items: AgendaItem[]): ActivityEntry {
  const phases = sortPhases(items)
  // The card sits on its due date when it has one, otherwise on its first phase.
  const primary = phases.find((phase) => phase.phase === 'due') ?? phases[0]
  return {
    kind: 'activity',
    key,
    date: primary.date,
    sortAt: sortAtOf(primary),
    title: primary.title,
    activityId: primary.activityId,
    course: primary.course,
    courseTitle: primary.courseTitle,
    instructor: primary.instructor,
    phases,
  }
}

function eventEntry(item: AgendaItem): EventEntry {
  return {
    kind: 'event',
    key: `event:${item.id}`,
    date: item.date,
    sortAt: sortAtOf(item),
    title: item.title,
    item,
  }
}

/** One entry per card. With groupPhases, the phases of one activity share a card. Events are never grouped. */
export function groupEntries(items: readonly AgendaItem[], groupPhases = true): AgendaEntry[] {
  const activities = new Map<string, AgendaItem[]>()
  const entries: AgendaEntry[] = []
  for (const item of items) {
    if (item.type === 'event') {
      entries.push(eventEntry(item))
      continue
    }
    const key = groupPhases && item.activityId ? `activity:${item.activityId}` : `item:${item.id}`
    const group = activities.get(key)
    if (group) group.push(item)
    else activities.set(key, [item])
  }
  for (const [key, group] of activities) entries.push(activityEntry(key, group))
  return entries.sort(compareEntries)
}

export function groupByDate(entries: readonly AgendaEntry[]): DayGroup[] {
  const groups: DayGroup[] = []
  for (const entry of entries) {
    const last = groups.at(-1)
    if (last && last.date === entry.date) last.entries.push(entry)
    else groups.push({ date: entry.date, entries: [entry] })
  }
  return groups
}

export function normalizeText(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim()
}

export function searchTerms(query: string): string[] {
  return normalizeText(query).split(/\s+/).filter(Boolean)
}

function searchableText(item: AgendaItem): string {
  return normalizeText(
    [item.title, item.course, item.courseTitle, item.instructor, item.location]
      .filter((part): part is string => Boolean(part))
      .join(' '),
  )
}

/**
 * Applies every filter. Subject and professor filters keep only activities, so campus
 * events are hidden while either is active. Search terms must all match (AND).
 */
export function applyFilters(items: readonly AgendaItem[], filters: AgendaFilters, today: string): AgendaItem[] {
  const terms = searchTerms(filters.query)
  const subjects = new Set(filters.subjects)
  const instructors = new Set(filters.instructors)
  const hasCourseFilter = subjects.size > 0 || instructors.size > 0

  return items.filter((item) => {
    if (!filters.showPast && item.date < today) return false
    if (item.type === 'event') {
      if (!filters.showCampus || hasCourseFilter) return false
    } else {
      if (subjects.size > 0 && (!item.course || !subjects.has(item.course))) return false
      if (instructors.size > 0 && (!item.instructor || !instructors.has(item.instructor))) return false
    }
    if (terms.length === 0) return true
    const text = searchableText(item)
    return terms.every((term) => text.includes(term))
  })
}

/** Subjects present in the entries, plus selected subjects so they can always be deselected. */
export function deriveSubjects(entries: readonly AgendaEntry[], selected: readonly string[]): SubjectFacet[] {
  const facets = new Map<string, SubjectFacet>()
  for (const entry of entries) {
    if (entry.kind !== 'activity' || !entry.course) continue
    const facet = facets.get(entry.course)
    if (facet) facet.count += 1
    else facets.set(entry.course, { code: entry.course, title: entry.courseTitle, count: 1 })
  }
  for (const code of selected) {
    if (!facets.has(code)) facets.set(code, { code, title: null, count: 0 })
  }
  return [...facets.values()].sort((a, b) => compareText(a.code, b.code))
}

export function deriveProfessors(entries: readonly AgendaEntry[], selected: readonly string[]): ProfessorFacet[] {
  const counts = new Map<string, number>()
  for (const entry of entries) {
    if (entry.kind !== 'activity' || !entry.instructor) continue
    counts.set(entry.instructor, (counts.get(entry.instructor) ?? 0) + 1)
  }
  for (const name of selected) {
    if (!counts.has(name)) counts.set(name, 0)
  }
  return [...counts]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => compareText(a.name, b.name))
}

export function entriesForSubject(entries: readonly AgendaEntry[], code: string): AgendaEntry[] {
  return entries.filter((entry) => entry.kind === 'activity' && entry.course === code)
}

export function entriesForProfessor(entries: readonly AgendaEntry[], name: string): AgendaEntry[] {
  return entries.filter((entry) => entry.kind === 'activity' && entry.instructor === name)
}

export function timingColumns(entries: readonly AgendaEntry[], today: string, showPast: boolean): BoardColumn[] {
  const order: TimingBucket[] = showPast ? ['today', 'next7', 'after7', 'past'] : ['today', 'next7', 'after7']
  const buckets = new Map<TimingBucket, AgendaEntry[]>(order.map((bucket) => [bucket, []]))
  for (const entry of entries) {
    buckets.get(timingBucket(entry.date, today))?.push(entry)
  }
  return order.map((bucket) => ({
    id: bucket,
    title: TIMING_TITLES[bucket],
    subtitle: null,
    entries: buckets.get(bucket) ?? [],
  }))
}

/** One column per subject code (sorted), then a campus column when there are events. */
export function subjectColumns(entries: readonly AgendaEntry[]): BoardColumn[] {
  const subjects = new Map<string, BoardColumn>()
  const campus: AgendaEntry[] = []
  for (const entry of entries) {
    if (entry.kind === 'event') {
      campus.push(entry)
      continue
    }
    const code = entry.course ?? 'Other'
    const column = subjects.get(code)
    if (column) column.entries.push(entry)
    else subjects.set(code, { id: code, title: code, subtitle: entry.courseTitle, entries: [entry] })
  }
  const columns = [...subjects.values()].sort((a, b) => compareText(a.title, b.title))
  if (campus.length > 0) {
    columns.push({ id: 'campus', title: CAMPUS_EVENT_LABEL, subtitle: null, entries: campus })
  }
  return columns
}

export function countByDate(items: readonly AgendaItem[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const item of items) counts.set(item.date, (counts.get(item.date) ?? 0) + 1)
  return counts
}

/** All phases of an activity, looked up across every loaded item so the details show each one. */
export function phasesForEntry(entry: AgendaEntry, allItems: readonly AgendaItem[]): AgendaItem[] {
  if (entry.kind === 'event') return [entry.item]
  if (!entry.activityId) return entry.phases
  const all = allItems.filter((item) => item.activityId === entry.activityId)
  return all.length > 0 ? sortPhases(all) : entry.phases
}

export function courseLabel(entry: AgendaEntry): string {
  if (entry.kind === 'event') return CAMPUS_EVENT_LABEL
  if (entry.course && entry.courseTitle) return `${entry.course} · ${entry.courseTitle}`
  return entry.course ?? 'Course not listed'
}
