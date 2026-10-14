import { describe, expect, it } from 'vitest'
import type { AgendaItem } from './api'
import {
  countdownFor,
  DEFAULT_WINDOW_MS,
  durationText,
  itemInstant,
  litSegments,
  openingInstant,
  openingsByActivity,
  timeLeftShare,
  zonedInstant,
} from './deadlines'

const DAY = 86_400_000
const HOUR = 3_600_000

const item = (overrides: Partial<AgendaItem>): AgendaItem => ({
  id: 'x',
  type: 'activity',
  phase: 'due',
  title: 'Quiz 3',
  course: 'CS101',
  courseTitle: 'Intro',
  instructor: 'A. Teacher',
  activityId: 'a1',
  location: null,
  allDay: false,
  date: '2026-10-14',
  startAt: '2026-10-14T23:59:00-03:00',
  endAt: null,
  endDate: null,
  ...overrides,
})

describe('zonedInstant (Saint John wall clock)', () => {
  it('uses Atlantic daylight time in October', () => {
    expect(zonedInstant('2026-10-13', 0, 0)).toBe(Date.parse('2026-10-13T03:00:00Z'))
  })

  it('uses Atlantic standard time in January', () => {
    expect(zonedInstant('2027-01-10', 12, 0)).toBe(Date.parse('2027-01-10T16:00:00Z'))
  })
})

describe('itemInstant', () => {
  it('uses the start time for timed items', () => {
    expect(itemInstant(item({}))).toBe(Date.parse('2026-10-14T23:59:00-03:00'))
  })

  it('runs an all-day due date to midnight at the end of its day', () => {
    const due = itemInstant(item({ allDay: true, startAt: null }))
    expect(due).toBe(zonedInstant('2026-10-15', 0, 0))
  })

  it('opens an all-day available phase at the start of the day', () => {
    const opens = itemInstant(item({ phase: 'available', allDay: true, startAt: null }))
    expect(opens).toBe(zonedInstant('2026-10-14', 0, 0))
  })
})

describe('countdownFor', () => {
  const due = Date.parse('2026-10-14T23:59:00-03:00')

  it('uses start wording for assessments while retaining the original Due instant', () => {
    const assessment = item({ title: 'Sample Quiz - Requires Respondus LockDown Browser' })
    expect(countdownFor(assessment, due - HOUR, true)).toEqual({ text: 'Starts in 1 h 0 min', passed: false })
    expect(countdownFor(assessment, due, true)).toEqual({ text: 'Started', passed: true })
    expect(countdownFor({ ...assessment, phase: 'available' }, due - HOUR, true).text).toBe('Opens in 1 h 0 min')
    expect(itemInstant(assessment)).toBe(due)
  })

  it('counts down in days and hours', () => {
    expect(countdownFor(item({}), due - 2 * DAY - 4 * HOUR)).toEqual({ text: 'Due in 2 d 4 h', passed: false })
  })

  it('counts down in hours and minutes', () => {
    expect(countdownFor(item({}), due - 3 * HOUR - 20 * 60_000)).toEqual({ text: 'Due in 3 h 20 min', passed: false })
  })

  it('reports a passed due date', () => {
    expect(countdownFor(item({}), due + HOUR)).toEqual({ text: 'Past due', passed: true })
  })

  it('uses phase wording for opening and closing', () => {
    const opens = item({ phase: 'available' })
    expect(countdownFor(opens, due - 30 * 60_000).text).toBe('Opens in 30 min')
    const closes = item({ phase: 'closes' })
    expect(countdownFor(closes, due + 1).text).toBe('Closed')
  })
})

describe('durationText', () => {
  it('never shows zero minutes', () => {
    expect(durationText(0)).toBe('1 min')
    expect(durationText(45 * 60_000)).toBe('45 min')
  })
})

describe('time-left meter', () => {
  const due = Date.parse('2026-10-14T23:59:00-03:00')

  it('uses the opening instant as the start of the window', () => {
    const opened = due - 10 * DAY
    expect(timeLeftShare(due, due - 5 * DAY, opened)).toBeCloseTo(0.5, 5)
  })

  it('falls back to the default window when nothing opens', () => {
    expect(timeLeftShare(due, due - DEFAULT_WINDOW_MS / 4, null)).toBeCloseTo(0.25, 5)
  })

  it('is empty once the due time has passed', () => {
    expect(timeLeftShare(due, due + 1, null)).toBe(0)
    expect(litSegments(0)).toBe(0)
  })

  it('lights at least one square while time is left and at most five', () => {
    expect(litSegments(0.01)).toBe(1)
    expect(litSegments(0.5)).toBe(3)
    expect(litSegments(1)).toBe(5)
    expect(litSegments(3)).toBe(5)
  })

  it('reads the opening instant from the available phase', () => {
    const phases = [item({ phase: 'due' }), item({ phase: 'available', startAt: '2026-10-01T09:00:00-03:00' })]
    expect(openingInstant(phases)).toBe(Date.parse('2026-10-01T09:00:00-03:00'))
    expect(openingInstant([item({ phase: 'due' })])).toBeNull()
  })
})

describe('openingsByActivity', () => {
  it('maps each assessment to its opening instant, including hidden past phases', () => {
    const items = [
      item({ id: 'o', phase: 'available', activityId: 'a1', date: '2026-10-01', startAt: null, allDay: true }),
      item({ id: 'd', phase: 'due', activityId: 'a1' }),
      item({ id: 'x', phase: 'due', activityId: 'a2' }),
    ]
    const openings = openingsByActivity(items)
    expect(openings.get('a1')).toBe(zonedInstant('2026-10-01', 0, 0))
    expect(openings.has('a2')).toBe(false)
  })
})
