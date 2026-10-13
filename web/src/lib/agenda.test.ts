import { describe, expect, it } from 'vitest'
import type { AgendaItem } from './api'
import {
  addDays,
  applyFilters,
  countByDate,
  courseLabel,
  deriveProfessors,
  deriveSubjects,
  describePhase,
  entriesForSubject,
  formatDayLabel,
  formatEventTime,
  formatLongDate,
  formatShortDay,
  formatTime,
  formatUpdated,
  groupByDate,
  groupEntries,
  monctonDate,
  monthGrid,
  monthRange,
  phasesForEntry,
  shiftMonth,
  subjectColumns,
  omittedEventsNotice,
  OMITTED_EVENTS_NOTICE,
  timingBucket,
  timingColumns,
} from './agenda'
import { FIXTURE_ITEMS, FIXTURE_TODAY, fixtureResponse } from '../test/fixtures'

const TODAY = '2026-10-13'
const ALL_ON = { query: '', subjects: [], instructors: [], showCampus: true, showPast: true }

function item(overrides: Partial<AgendaItem> & Pick<AgendaItem, 'id'>): AgendaItem {
  return {
    type: 'activity',
    phase: 'due',
    title: 'Lab07',
    course: 'Lab 101',
    courseTitle: 'Sample Course',
    instructor: 'Sample Instructor',
    activityId: 'act-1',
    location: null,
    allDay: false,
    date: TODAY,
    startAt: `${TODAY}T23:59:00-03:00`,
    endAt: null,
    endDate: null,
    ...overrides,
  }
}

describe('filtering', () => {
  const items = FIXTURE_ITEMS

  it('keeps only the selected subjects and hides campus events', () => {
    const result = applyFilters(items, { ...ALL_ON, subjects: ['Cloud I'] }, TODAY)
    expect(result.length).toBeGreaterThan(0)
    expect(result.every((i) => i.course === 'Cloud I')).toBe(true)
  })

  it('keeps only the selected professors', () => {
    const result = applyFilters(items, { ...ALL_ON, instructors: ['Sample Instructor Two'] }, TODAY)
    expect(result.every((i) => i.instructor === 'Sample Instructor Two')).toBe(true)
  })

  it('hides campus events when the campus toggle is off', () => {
    const result = applyFilters(items, { ...ALL_ON, showCampus: false }, TODAY)
    expect(result.some((i) => i.type === 'event')).toBe(false)
    expect(result.some((i) => i.type === 'activity')).toBe(true)
  })

  it('hides past items unless show past is on', () => {
    const hidden = applyFilters(items, { ...ALL_ON, showPast: false }, TODAY)
    expect(hidden.every((i) => i.date >= TODAY)).toBe(true)
    const shown = applyFilters(items, { ...ALL_ON, showPast: true }, TODAY)
    expect(shown.some((i) => i.date < TODAY)).toBe(true)
  })

  it('searches title, course, course title, instructor and location case-insensitively', () => {
    const search = (query: string) => applyFilters(items, { ...ALL_ON, query }, TODAY)
    expect(search('LAB07').every((i) => i.title === 'Lab07')).toBe(true)
    expect(search('sample web').every((i) => i.course === 'Web II')).toBe(true)
    expect(search('two').every((i) => i.instructor === 'Sample Instructor Two')).toBe(true)
    expect(search('library').map((i) => i.title)).toEqual(['Sample Career Fair'])
    expect(search('cloud i').length).toBeGreaterThan(0)
  })

  it('requires every search term to match', () => {
    const result = applyFilters(items, { ...ALL_ON, query: 'lab07 sample' }, TODAY)
    expect(result.length).toBeGreaterThan(0)
    expect(result.every((i) => i.title === 'Lab07')).toBe(true)
    expect(applyFilters(items, { ...ALL_ON, query: 'lab07 cloud' }, TODAY)).toEqual([])
  })

  it('ignores accents when searching', () => {
    const accented = [item({ id: 'x', type: 'event', phase: null, course: null, courseTitle: null, instructor: null, activityId: null, title: 'Café Session', location: 'Sample Campus - Café' })]
    expect(applyFilters(accented, { ...ALL_ON, query: 'cafe' }, TODAY)).toHaveLength(1)
  })
})

describe('grouping', () => {
  it('combines the phases of one activity into one card, ordered available, due, closes', () => {
    const entries = groupEntries(FIXTURE_ITEMS.filter((i) => i.activityId === 'act-lab07'))
    expect(entries).toHaveLength(1)
    const [entry] = entries
    expect(entry.kind).toBe('activity')
    if (entry.kind !== 'activity') return
    expect(entry.phases.map((p) => p.phase)).toEqual(['available', 'due', 'closes'])
  })

  it('places an activity card on its due date', () => {
    const [entry] = groupEntries(FIXTURE_ITEMS.filter((i) => i.activityId === 'act-lab07'))
    expect(entry.date).toBe('2026-10-14')
  })

  it('never groups campus events, even when they share a title', () => {
    const twins = [
      item({ id: 'e-a', type: 'event', phase: null, course: null, courseTitle: null, instructor: null, activityId: null, title: 'Sample Fair', location: 'A', startAt: `${TODAY}T10:00:00-03:00` }),
      item({ id: 'e-b', type: 'event', phase: null, course: null, courseTitle: null, instructor: null, activityId: null, title: 'Sample Fair', location: 'B', startAt: `${TODAY}T11:00:00-03:00` }),
    ]
    expect(groupEntries(twins)).toHaveLength(2)
  })

  it('returns one entry per item when phases are not grouped', () => {
    const phases = FIXTURE_ITEMS.filter((i) => i.activityId === 'act-lab07')
    expect(groupEntries(phases, false)).toHaveLength(phases.length)
  })

  it('groups entries by day in chronological order with all-day items first', () => {
    const items = [
      item({ id: 't1', date: TODAY, startAt: `${TODAY}T17:00:00-03:00`, activityId: 'a1', title: 'B' }),
      item({ id: 't2', date: TODAY, allDay: true, startAt: null, activityId: 'a2', title: 'A' }),
      item({ id: 't3', date: '2026-10-12', startAt: '2026-10-12T08:00:00-03:00', activityId: 'a3', title: 'C' }),
    ]
    const days = groupByDate(groupEntries(items))
    expect(days.map((d) => d.date)).toEqual(['2026-10-12', TODAY])
    expect(days[1].entries.map((e) => e.title)).toEqual(['A', 'B'])
  })

  it('returns the phases of an activity from all loaded items for the details view', () => {
    const calendarPhase = groupEntries(FIXTURE_ITEMS.filter((i) => i.id === 'f06'), false)[0]
    const phases = phasesForEntry(calendarPhase, FIXTURE_ITEMS)
    expect(phases.map((p) => p.phase)).toEqual(['available', 'due', 'closes'])
  })
})

describe('facets', () => {
  it('derives subjects with counts of cards, sorted by code', () => {
    const entries = groupEntries(FIXTURE_ITEMS)
    const subjects = deriveSubjects(entries, [])
    expect(subjects.map((s) => s.code)).toEqual(['Cloud I', 'Lab 101', 'Web II'])
    const lab = subjects.find((s) => s.code === 'Lab 101')
    expect(lab?.count).toBe(entries.filter((e) => e.kind === 'activity' && e.course === 'Lab 101').length)
    expect(lab?.title).toBe('Sample Course')
  })

  it('keeps selected subjects with zero count so they can be deselected', () => {
    const subjects = deriveSubjects([], ['Lab 101'])
    expect(subjects).toEqual([{ code: 'Lab 101', title: null, count: 0 }])
  })

  it('derives professors and ignores activities without one', () => {
    const professors = deriveProfessors(groupEntries(FIXTURE_ITEMS), [])
    expect(professors.map((p) => p.name)).toEqual(['Sample Instructor', 'Sample Instructor Two'])
  })

  it('returns no professors when none are listed', () => {
    const none = FIXTURE_ITEMS.map((i) => ({ ...i, instructor: null }))
    expect(deriveProfessors(groupEntries(none), [])).toEqual([])
  })

  it('filters entries by subject for a tab', () => {
    const entries = groupEntries(FIXTURE_ITEMS)
    expect(entriesForSubject(entries, 'Web II').every((e) => e.kind === 'activity' && e.course === 'Web II')).toBe(true)
  })

  it('counts items per date', () => {
    const counts = countByDate(FIXTURE_ITEMS.filter((i) => i.date === '2026-10-13'))
    expect(counts.get('2026-10-13')).toBe(2)
  })
})

describe('board columns', () => {
  const entries = groupEntries(FIXTURE_ITEMS)

  it('places entries in timing buckets and hides the past column by default', () => {
    const columns = timingColumns(entries, TODAY, false)
    expect(columns.map((c) => c.title)).toEqual(['Today', 'Next 7 days', 'After 7 days'])
    expect(columns[0].entries.every((e) => e.date === TODAY)).toBe(true)
  })

  it('adds the past column when show past is on', () => {
    const columns = timingColumns(entries, TODAY, true)
    expect(columns.at(-1)?.title).toBe('Past')
    expect(columns.at(-1)?.entries.every((e) => e.date < TODAY)).toBe(true)
  })

  it('creates one column per subject with campus events last', () => {
    const columns = subjectColumns(entries)
    expect(columns.map((c) => c.title)).toEqual(['Cloud I', 'Lab 101', 'Web II', 'Campus event'])
  })
})

describe('timing buckets', () => {
  it('uses today as the boundary for every bucket', () => {
    expect(timingBucket('2026-10-12', TODAY)).toBe('past')
    expect(timingBucket(TODAY, TODAY)).toBe('today')
    expect(timingBucket('2026-10-14', TODAY)).toBe('next7')
    expect(timingBucket('2026-10-20', TODAY)).toBe('next7')
    expect(timingBucket('2026-10-21', TODAY)).toBe('after7')
  })
})

describe('Saint John formatting', () => {
  it('formats 2026-10-14T23:59 at -03:00 as 11:59 PM on Oct 14', () => {
    expect(formatTime('2026-10-14T23:59:00-03:00')).toBe('11:59 PM')
    expect(monctonDate('2026-10-14T23:59:00-03:00')).toBe('2026-10-14')
    expect(formatShortDay('2026-10-14')).toBe('Wed, Oct 14')
  })

  it('shows the Moncton day for a UTC instant that is still the previous day in Saint John', () => {
    expect(monctonDate('2026-10-14T02:30:00Z')).toBe('2026-10-13')
    expect(formatTime('2026-10-14T02:30:00Z')).toBe('11:30 PM')
  })

  it('labels all-day items as All day', () => {
    const allDay = item({ id: 'd', allDay: true, startAt: null, type: 'event', phase: null, course: null, courseTitle: null, instructor: null, activityId: null, date: '2026-10-22', endDate: null, location: null })
    expect(formatEventTime(allDay)).toBe('All day')
    expect(formatEventTime({ ...allDay, endDate: '2026-10-23' })).toBe('All day until Oct 23')
  })

  it('formats relative day labels', () => {
    expect(formatDayLabel(TODAY, TODAY)).toBe('Today')
    expect(formatDayLabel('2026-10-14', TODAY)).toBe('Tomorrow')
    expect(formatDayLabel('2026-10-15', TODAY)).toBe('Thu, Oct 15')
  })

  it('formats long dates and phase lines', () => {
    expect(formatLongDate('2026-10-14')).toBe('Wednesday, October 14, 2026')
    const due = item({ id: 'p', date: '2026-10-14', startAt: '2026-10-14T23:59:00-03:00' })
    expect(describePhase(due, '2026-10-14')).toBe('Due 11:59 PM')
    expect(describePhase({ ...due, phase: 'available', date: '2026-10-01', startAt: '2026-10-01T09:00:00-03:00' }, '2026-10-14')).toBe('Opens Oct 1, 9:00 AM')
  })

  it('describes the card course and falls back to the campus label', () => {
    const entries = groupEntries(FIXTURE_ITEMS.filter((i) => i.id === 'f04' || i.id === 'e02'))
    const activity = entries.find((e) => e.kind === 'activity')
    const event = entries.find((e) => e.kind === 'event')
    if (!activity || !event) throw new Error('expected one activity and one event')
    expect(courseLabel(activity)).toBe('Cloud I · Sample Cloud Course')
    expect(courseLabel(event)).toBe('Campus event')
  })

  it('describes how long ago the feed was updated', () => {
    const updated = '2026-10-13T11:20:00-03:00'
    const at = Date.parse(updated)
    expect(formatUpdated(updated, at + 12 * 60_000)).toBe('Updated 12 min ago')
    expect(formatUpdated(updated, at + 30_000)).toBe('Updated just now')
    expect(formatUpdated(updated, at + 3 * 3_600_000)).toBe('Updated 3 hours ago')
  })
})

describe('omitted campus events notice', () => {
  it('shows nothing when no campus events were omitted', () => {
    expect(omittedEventsNotice(0)).toBeNull()
  })

  it('shows nothing when the count is missing', () => {
    expect(omittedEventsNotice(undefined)).toBeNull()
  })

  it('shows the notice only when the count is above zero', () => {
    expect(omittedEventsNotice(1)).toBe(OMITTED_EVENTS_NOTICE)
    expect(omittedEventsNotice(7)).toBe(OMITTED_EVENTS_NOTICE)
  })

  it('states the outcome without internal detail', () => {
    expect(OMITTED_EVENTS_NOTICE).toContain('All course deadlines are still listed.')
    expect(OMITTED_EVENTS_NOTICE).not.toMatch(/http|feed|token|uid/i)
  })
})

describe('calendar date math', () => {
  it('adds days across month and year boundaries', () => {
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-10-13', -13)).toBe('2026-09-30')
  })

  it('builds month ranges, including leap years', () => {
    expect(monthRange('2026-02')).toEqual({ from: '2026-02-01', to: '2026-02-28' })
    expect(monthRange('2028-02')).toEqual({ from: '2028-02-01', to: '2028-02-29' })
  })

  it('moves between months', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2027-01', -1)).toBe('2026-12')
  })

  it('lays out weeks starting on Sunday with blanks outside the month', () => {
    const weeks = monthGrid('2026-10')
    // October 1, 2026 is a Thursday.
    expect(weeks[0]).toEqual([null, null, null, null, '2026-10-01', '2026-10-02', '2026-10-03'])
    expect(weeks.flat().filter(Boolean)).toHaveLength(31)
    expect(weeks.every((w) => w.length === 7)).toBe(true)
  })

  it('uses the fixture window for the default range', () => {
    const response = fixtureResponse()
    expect(response.today).toBe(FIXTURE_TODAY)
    expect(response.items.length).toBe(FIXTURE_ITEMS.length)
  })
})
