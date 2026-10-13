import { describe, expect, it } from 'vitest'
import {
  normalizeSetupCode,
  validateEmail,
  validateFeedUrl,
  validateGroupName,
  validateInstructorName,
  validateMinutes,
  validateOptionalGroupJid,
  validateOptionalUuid,
  validatePassword,
  validatePublicBaseUrl,
  validateSecretReplacement,
  validateSetupCode,
  validateWebhookSecret,
} from './rules'

const FEED = 'https://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics'

describe('feed URL', () => {
  it('accepts the NBCC feed with a token', () => {
    expect(validateFeedUrl(`${FEED}?token=sample`)).toBeNull()
  })

  it('rejects other hosts, paths, schemes, ports, userinfo and fragments', () => {
    expect(validateFeedUrl(`http://elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=sample`)).toBe(
      'invalid_feed_url',
    )
    expect(validateFeedUrl(`https://example.com/d2l/le/calendar/feed/user/feed.ics?token=sample`)).toBe(
      'invalid_feed_url',
    )
    expect(validateFeedUrl(`https://elearning.nbcc.ca/d2l/le/other?token=sample`)).toBe('invalid_feed_url')
    expect(validateFeedUrl(`https://user:pw@elearning.nbcc.ca/d2l/le/calendar/feed/user/feed.ics?token=sample`)).toBe(
      'invalid_feed_url',
    )
    expect(validateFeedUrl(`${FEED}?token=sample#frag`)).toBe('invalid_feed_url')
    expect(validateFeedUrl(`https://elearning.nbcc.ca:8443/d2l/le/calendar/feed/user/feed.ics?token=sample`)).toBe(
      'invalid_feed_url',
    )
  })

  it('requires a non-empty token', () => {
    expect(validateFeedUrl(FEED)).toBe('invalid_feed_url')
    expect(validateFeedUrl(`${FEED}?token=`)).toBe('invalid_feed_url')
  })

  it('requires a value', () => {
    expect(validateFeedUrl('   ')).toBe('required')
  })
})

describe('identifiers', () => {
  it('checks the optional UUID', () => {
    expect(validateOptionalUuid('11111111-2222-4333-8444-555555555555')).toBeNull()
    expect(validateOptionalUuid('not-a-uuid')).toBe('invalid_uuid')
    expect(validateOptionalUuid('')).toBeNull()
  })

  it('checks the optional group JID', () => {
    expect(validateOptionalGroupJid('120363000000000000@g.us')).toBeNull()
    expect(validateOptionalGroupJid('12345-67890@g.us')).toBeNull()
    expect(validateOptionalGroupJid('123@g.us')).toBe('invalid_group_jid')
    expect(validateOptionalGroupJid('120363000000000000@s.whatsapp.net')).toBe('invalid_group_jid')
    expect(validateOptionalGroupJid('')).toBeNull()
  })

  it('limits the group name length', () => {
    expect(validateGroupName('x'.repeat(200))).toBeNull()
    expect(validateGroupName('x'.repeat(201))).toBe('too_long')
  })
})

describe('minutes', () => {
  it('accepts whole numbers from 1 to 1440', () => {
    expect(validateMinutes('1')).toBeNull()
    expect(validateMinutes('1440')).toBeNull()
  })

  it('rejects values out of range or not whole', () => {
    expect(validateMinutes('0')).toBe('out_of_range')
    expect(validateMinutes('1441')).toBe('out_of_range')
    expect(validateMinutes('5.5')).toBe('out_of_range')
    expect(validateMinutes('')).toBe('required')
  })
})

describe('passwords', () => {
  const EMAIL = 'admin@example.test'

  it('rejects a password shorter than 12 characters', () => {
    expect(validatePassword('short', EMAIL)).toBe('too_short')
  })

  it('rejects a password longer than 256 characters', () => {
    expect(validatePassword('a'.repeat(257), EMAIL)).toBe('too_long')
    expect(validatePassword('a'.repeat(256), EMAIL)).toBeNull()
  })

  it('rejects a password equal to the e-mail, ignoring case', () => {
    expect(validatePassword('ADMIN@example.TEST', EMAIL)).toBe('same_as_email')
  })

  it('accepts a long passphrase', () => {
    expect(validatePassword('correct horse battery', EMAIL)).toBeNull()
  })

  it('counts characters, not UTF-16 units', () => {
    expect(validatePassword('ação-ação-ação', EMAIL)).toBeNull()
  })
})

describe('webhook secret and public address', () => {
  it('needs at least 32 characters', () => {
    expect(validateWebhookSecret('x'.repeat(31))).toBe('too_short')
    expect(validateWebhookSecret('x'.repeat(32))).toBeNull()
    expect(validateWebhookSecret('')).toBe('required')
  })

  it('replacement values are required when the user chose to replace', () => {
    expect(validateSecretReplacement('  ', (value) => validateWebhookSecret(value))).toBe('required')
  })

  it('accepts https with the default port and no path, and an empty value', () => {
    expect(validatePublicBaseUrl('https://nbcc.example.com')).toBeNull()
    expect(validatePublicBaseUrl('https://nbcc.example.com/')).toBeNull()
    expect(validatePublicBaseUrl('')).toBeNull()
  })

  it('rejects http, paths, queries, fragments and ports', () => {
    expect(validatePublicBaseUrl('http://nbcc.example.com')).toBe('invalid_https_url')
    expect(validatePublicBaseUrl('https://nbcc.example.com/path')).toBe('invalid_https_url')
    expect(validatePublicBaseUrl('https://nbcc.example.com?q=1')).toBe('invalid_https_url')
    expect(validatePublicBaseUrl('https://nbcc.example.com#x')).toBe('invalid_https_url')
    expect(validatePublicBaseUrl('https://nbcc.example.com:8443')).toBe('invalid_https_url')
  })
})

describe('setup code and e-mail', () => {
  it('accepts the code with or without dashes', () => {
    expect(validateSetupCode('abcd-efgh-jkmn')).toBeNull()
    expect(validateSetupCode('ABCDEFGHJKMN')).toBeNull()
    expect(validateSetupCode('ABCD')).toBe('invalid_setup_code')
  })

  it('formats the code with dashes', () => {
    expect(normalizeSetupCode('abcdefghjkmn')).toBe('ABCD-EFGH-JKMN')
    expect(normalizeSetupCode(' abcd-efgh-jkmn ')).toBe('ABCD-EFGH-JKMN')
  })

  it('checks the e-mail format', () => {
    expect(validateEmail('admin@example.test')).toBeNull()
    expect(validateEmail('admin.example.test')).toBe('invalid_email')
    expect(validateEmail('')).toBe('required')
  })
})

describe('instructor names', () => {
  it('limits the name length and rejects control characters', () => {
    expect(validateInstructorName('x'.repeat(100))).toBeNull()
    expect(validateInstructorName('x'.repeat(101))).toBe('too_long')
    expect(validateInstructorName('Ana\u0007')).toBe('invalid_instructor_map')
    expect(validateInstructorName('   ')).toBeNull()
  })
})
