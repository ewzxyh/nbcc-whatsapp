import { describe, expect, it } from 'vitest'
import { groupEntries } from './agenda'
import { assessmentKind, formatCopyText } from './share'
import type { AgendaItem } from './api'

const phase = (overrides: Partial<AgendaItem>): AgendaItem => ({
  id: 'p',
  type: 'activity',
  phase: 'due',
  title: 'Quiz 3',
  course: 'CS101',
  courseTitle: 'Intro to Programming',
  instructor: 'Jane Doe',
  activityId: 'a1',
  location: null,
  allDay: false,
  date: '2026-10-14',
  startAt: '2026-10-14T23:59:00-03:00',
  endAt: null,
  endDate: null,
  ...overrides,
})

describe('formatCopyText', () => {
  it('includes subject, assessment, professor and due date with time', () => {
    const [entry] = groupEntries([phase({})])
    const text = formatCopyText(entry)
    expect(text).toContain('Subject: CS101 · Intro to Programming')
    expect(text).toContain('Assessment: Quiz 3')
    expect(text).toContain('Professor: Jane Doe')
    expect(text).toContain('Due: Wednesday, October 14, 2026 at 11:59 PM')
  })

  it('lists every phase and marks all-day items', () => {
    const [entry] = groupEntries([
      phase({ id: 'o', phase: 'available', date: '2026-10-01', startAt: null, allDay: true }),
      phase({ id: 'd' }),
    ])
    const text = formatCopyText(entry)
    expect(text).toContain('Opens: Thursday, October 1, 2026 (all day)')
    expect(text).toContain('Due: Wednesday, October 14, 2026 at 11:59 PM')
  })

  it('omits the professor line when none is listed', () => {
    const [entry] = groupEntries([phase({ instructor: null })])
    expect(formatCopyText(entry)).not.toContain('Professor:')
  })

  it('formats campus events with place and time', () => {
    const [entry] = groupEntries([
      phase({ id: 'ev', type: 'event', phase: null, course: null, courseTitle: null, instructor: null, activityId: null, title: 'Career Fair', location: 'Atrium' }),
    ])
    const text = formatCopyText(entry)
    expect(text).toMatch(/^Campus event: Career Fair/)
    expect(text).toContain('Location: Atrium')
  })
})

describe('assessmentKind', () => {
  it('reads the assessment type from the title', () => {
    expect(assessmentKind('Quiz 3')).toBe('Quiz')
    expect(assessmentKind('Lab01')).toBe('Lab')
    expect(assessmentKind('Midterm exam')).toBe('Exam')
    expect(assessmentKind('Essay draft')).toBe('Essay')
    expect(assessmentKind('Final Project Proposal')).toBe('Project')
    expect(assessmentKind('Final exam')).toBe('Exam')
    expect(assessmentKind('Week 4 reading')).toBeNull()
  })
})
