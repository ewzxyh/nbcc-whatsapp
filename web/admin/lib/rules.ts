// Client-side mirror of the backend rules. Each validator returns an error CODE (see errors.ts) or null.
// The backend stays authoritative; its 422 field codes are shown through the same copy table.

export type SecretAction = 'keep' | 'replace' | 'remove'

export const FEED_HOST = 'elearning.nbcc.ca'
export const FEED_PATH = '/d2l/le/calendar/feed/user/feed.ics'
export const PASSWORD_MIN_LENGTH = 12
export const PASSWORD_MAX_LENGTH = 256
export const WEBHOOK_SECRET_MIN_LENGTH = 32
export const MINUTES_MIN = 1
export const MINUTES_MAX = 1440
export const GROUP_NAME_MAX_LENGTH = 200
export const INSTRUCTOR_MAX_ENTRIES = 100
export const INSTRUCTOR_NAME_MAX_LENGTH = 100
export const INSTRUCTOR_CODE_MAX_LENGTH = 40

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const GROUP_JID_PATTERN = /^\d{5,25}(?:-\d{5,20})?@g\.us$/
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const CONTROL_CHARACTERS = /[\u0000-\u001f\u007f]/
const SETUP_CODE_PATTERN = /^[A-Z0-9]{12}$/

export function codePointLength(value: string): number {
  return Array.from(value).length
}

export function validateSecretReplacement(value: string, validate: (value: string) => string | null): string | null {
  const trimmed = value.trim()
  if (trimmed === '') return 'required'
  return validate(trimmed)
}

// Feed URL: https, host elearning.nbcc.ca, the exact feed path, a non-empty token, no userinfo, no fragment.
export function validateFeedUrl(raw: string): string | null {
  const value = raw.trim()
  if (value === '') return 'required'
  if (value.includes('#')) return 'invalid_feed_url'
  let url: URL
  try {
    url = new URL(value)
  } catch {
    return 'invalid_feed_url'
  }
  const valid =
    url.protocol === 'https:' &&
    url.hostname === FEED_HOST &&
    url.port === '' &&
    url.pathname === FEED_PATH &&
    url.username === '' &&
    url.password === '' &&
    (url.searchParams.get('token') ?? '').trim() !== ''
  return valid ? null : 'invalid_feed_url'
}

export function validateEmail(raw: string): string | null {
  const value = raw.trim()
  if (value === '') return 'required'
  return EMAIL_PATTERN.test(value) ? null : 'invalid_email'
}

export function validatePassword(value: string, email: string): string | null {
  if (value === '') return 'required'
  const length = codePointLength(value)
  if (length < PASSWORD_MIN_LENGTH) return 'too_short'
  if (length > PASSWORD_MAX_LENGTH) return 'too_long'
  if (email.trim() !== '' && value.toLowerCase() === email.trim().toLowerCase()) return 'same_as_email'
  return null
}

// The setup code is 12 characters (about 60 bits). The user may type it with or without dashes.
export function normalizeSetupCode(raw: string): string {
  const cleaned = raw.replace(/[^A-Za-z0-9]/g, '').toUpperCase()
  if (cleaned.length !== 12) return cleaned
  return `${cleaned.slice(0, 4)}-${cleaned.slice(4, 8)}-${cleaned.slice(8)}`
}

export function validateSetupCode(raw: string): string | null {
  if (raw.trim() === '') return 'required'
  const cleaned = raw.replace(/[^A-Za-z0-9]/g, '').toUpperCase()
  return SETUP_CODE_PATTERN.test(cleaned) ? null : 'invalid_setup_code'
}

// Optional fields: an empty value is allowed (the server decides), a non-empty value must match.
export function validateOptionalUuid(raw: string): string | null {
  const value = raw.trim()
  return value === '' || UUID_PATTERN.test(value) ? null : 'invalid_uuid'
}

export function validateOptionalGroupJid(raw: string): string | null {
  const value = raw.trim()
  return value === '' || GROUP_JID_PATTERN.test(value) ? null : 'invalid_group_jid'
}

export function validateGroupName(raw: string): string | null {
  return codePointLength(raw.trim()) > GROUP_NAME_MAX_LENGTH ? 'too_long' : null
}

export function validateMinutes(raw: string): string | null {
  const value = raw.trim()
  if (value === '') return 'required'
  if (!/^\d+$/.test(value)) return 'out_of_range'
  const minutes = Number(value)
  return minutes >= MINUTES_MIN && minutes <= MINUTES_MAX ? null : 'out_of_range'
}

export function normalizePublicBaseUrl(raw: string): string {
  const value = raw.trim()
  return value.endsWith('/') ? value.slice(0, -1) : value
}

// Public base URL: https host only, default port, no path, query or fragment. Empty clears it.
export function validatePublicBaseUrl(raw: string): string | null {
  const value = raw.trim()
  if (value === '') return null
  if (/[?#]/.test(value)) return 'invalid_https_url'
  let url: URL
  try {
    url = new URL(normalizePublicBaseUrl(value))
  } catch {
    return 'invalid_https_url'
  }
  const valid =
    url.protocol === 'https:' &&
    url.hostname !== '' &&
    url.port === '' &&
    url.username === '' &&
    url.password === '' &&
    (url.pathname === '/' || url.pathname === '')
  return valid ? null : 'invalid_https_url'
}

export function validateWebhookSecret(raw: string): string | null {
  const value = raw.trim()
  if (value === '') return 'required'
  return codePointLength(value) < WEBHOOK_SECRET_MIN_LENGTH ? 'too_short' : null
}

export function validateInstructorName(raw: string): string | null {
  const value = raw.trim()
  if (value === '') return null
  if (CONTROL_CHARACTERS.test(value)) return 'invalid_instructor_map'
  return codePointLength(value) > INSTRUCTOR_NAME_MAX_LENGTH ? 'too_long' : null
}

export function validateInstructorCode(code: string): string | null {
  if (CONTROL_CHARACTERS.test(code)) return 'invalid_instructor_map'
  return code.length > INSTRUCTOR_CODE_MAX_LENGTH ? 'too_long' : null
}
