// All times in the admin are shown in Saint John (Atlantic) time, whatever the browser zone is.
export const MONCTON_TIME_ZONE = 'America/Moncton'
export const TIMEZONE_NOTE = 'Horário de Saint John (Atlântico)'

const dateTimeFormatter = new Intl.DateTimeFormat('pt-BR', {
  timeZone: MONCTON_TIME_ZONE,
  day: 'numeric',
  month: 'long',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
})

// Calendar dates (YYYY-MM-DD) are already Moncton days. Format them in UTC at noon so no zone shifts them.
const calendarDateFormatter = new Intl.DateTimeFormat('pt-BR', {
  timeZone: 'UTC',
  weekday: 'long',
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return dateTimeFormatter.format(date)
}

export function formatCalendarDate(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return '—'
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3]), 12))
  if (Number.isNaN(date.getTime())) return '—'
  return calendarDateFormatter.format(date)
}
