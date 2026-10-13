import { describe, expect, it } from 'vitest'
import { formatCalendarDate, formatDateTime, MONCTON_TIME_ZONE, TIMEZONE_NOTE } from './dates'

describe('formatDateTime (America/Moncton)', () => {
  it('shows the previous Moncton day for an instant just after midnight UTC', () => {
    // 02:59 UTC is 23:59 on 13 October in Saint John (ADT, UTC-3).
    expect(formatDateTime('2026-10-14T02:59:00Z')).toBe('13 de outubro de 2026 às 23:59')
  })

  it('formats an offset timestamp in Saint John time', () => {
    expect(formatDateTime('2026-10-13T18:20:00-03:00')).toBe('13 de outubro de 2026 às 18:20')
  })

  it('converts a UTC instant to the Atlantic offset in October', () => {
    expect(formatDateTime('2026-10-13T21:00:00Z')).toBe('13 de outubro de 2026 às 18:00')
  })

  it('returns a dash for a missing or invalid value', () => {
    expect(formatDateTime(null)).toBe('—')
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime('not a date')).toBe('—')
  })

  it('uses the Moncton zone and the required label', () => {
    expect(MONCTON_TIME_ZONE).toBe('America/Moncton')
    expect(TIMEZONE_NOTE).toBe('Horário de Saint John (Atlântico)')
  })
})

describe('formatCalendarDate', () => {
  it('keeps a calendar date as written, whatever the browser zone', () => {
    expect(formatCalendarDate('2026-10-15')).toBe('quinta-feira, 15 de outubro de 2026')
    expect(formatCalendarDate('2026-10-01')).toBe('quinta-feira, 1 de outubro de 2026')
  })

  it('returns a dash for a value that is not a date', () => {
    expect(formatCalendarDate('15/10/2026')).toBe('—')
  })
})
