import type { z } from 'zod'
import { describe, expect, it } from 'vitest'
import {
  agendaFormSchema,
  emailChangeSchema,
  instructorFormSchema,
  loginSchema,
  passwordChangeSchema,
  setupSchema,
  whatsappFormSchema,
} from './schemas'

type Issue = { path: string; message: string }

function issuesOf(schema: z.ZodType, value: unknown): Issue[] {
  const result = schema.safeParse(value)
  if (result.success) return []
  return result.error.issues.map((issue) => ({ path: issue.path.join('.'), message: issue.message }))
}

const AGENDA_BASE = { nbccFeedUrl: '', nbccFeedUrlAction: 'keep', agendaCacheMinutes: '60' }
const WHATSAPP_BASE = {
  sendEnabled: false,
  pollIntervalMinutes: '5',
  grapnelApiKey: '',
  grapnelApiKeyAction: 'keep',
  grapnelInstanceId: '',
  grapnelGroupJid: '',
  grapnelGroupName: '',
  publicBaseUrl: '',
  webhookSecret: '',
  webhookSecretAction: 'keep',
}

describe('agendaFormSchema', () => {
  it('ignores the feed field when the user keeps the saved link', () => {
    expect(issuesOf(agendaFormSchema, { ...AGENDA_BASE, nbccFeedUrl: 'garbage' })).toEqual([])
  })

  it('checks a replacement feed link and reports the field', () => {
    expect(
      issuesOf(agendaFormSchema, { ...AGENDA_BASE, nbccFeedUrlAction: 'replace', nbccFeedUrl: 'https://x.test' }),
    ).toEqual([{ path: 'nbccFeedUrl', message: 'invalid_feed_url' }])
  })

  it('requires a value for a replacement', () => {
    expect(issuesOf(agendaFormSchema, { ...AGENDA_BASE, nbccFeedUrlAction: 'replace' })).toEqual([
      { path: 'nbccFeedUrl', message: 'required' },
    ])
  })

  it('checks the cache interval range', () => {
    expect(issuesOf(agendaFormSchema, { ...AGENDA_BASE, agendaCacheMinutes: '0' })).toEqual([
      { path: 'agendaCacheMinutes', message: 'out_of_range' },
    ])
  })
})

describe('whatsappFormSchema', () => {
  it('needs at least 32 characters for a replacement webhook secret', () => {
    expect(
      issuesOf(whatsappFormSchema, { ...WHATSAPP_BASE, webhookSecretAction: 'replace', webhookSecret: 'short' }),
    ).toEqual([{ path: 'webhookSecret', message: 'too_short' }])
  })

  it('checks the group JID and public address formats', () => {
    expect(
      issuesOf(whatsappFormSchema, {
        ...WHATSAPP_BASE,
        grapnelGroupJid: '123@g.us',
        publicBaseUrl: 'http://nbcc.example.com',
      }),
    ).toEqual([
      { path: 'grapnelGroupJid', message: 'invalid_group_jid' },
      { path: 'publicBaseUrl', message: 'invalid_https_url' },
    ])
  })

  it('leaves the send-without-config check to the server', () => {
    expect(issuesOf(whatsappFormSchema, { ...WHATSAPP_BASE, sendEnabled: true })).toEqual([])
  })
})

describe('instructorFormSchema', () => {
  it('limits the number of named instructors to 100', () => {
    const entries = Array.from({ length: 101 }, (_, index) => ({
      code: `C${index}`,
      title: '',
      instructor: `Name ${index}`,
    }))
    expect(issuesOf(instructorFormSchema, { entries })).toEqual([
      { path: 'entries', message: 'invalid_instructor_map' },
    ])
  })

  it('ignores blank rows and rejects control characters in a name', () => {
    const blank = Array.from({ length: 150 }, (_, index) => ({ code: `B${index}`, title: '', instructor: '  ' }))
    expect(issuesOf(instructorFormSchema, { entries: blank })).toEqual([])
    expect(
      issuesOf(instructorFormSchema, { entries: [{ code: 'Linux I', title: '', instructor: 'Ana\u0007' }] }),
    ).toEqual([{ path: 'entries.0.instructor', message: 'invalid_instructor_map' }])
  })
})

describe('account schemas', () => {
  it('requires the e-mail and the password to sign in', () => {
    expect(issuesOf(loginSchema, { email: '', password: '' })).toEqual([
      { path: 'email', message: 'required' },
      { path: 'password', message: 'required' },
    ])
  })

  it('enforces the password rules, the new password differing from the current one, and confirmation', () => {
    const email = 'admin@example.test'
    expect(
      issuesOf(passwordChangeSchema(email), {
        currentPassword: 'current password 1',
        newPassword: 'current password 1',
        confirmPassword: 'current password 1',
      }),
    ).toEqual([{ path: 'newPassword', message: 'same_as_password' }])
    expect(
      issuesOf(passwordChangeSchema(email), {
        currentPassword: 'current password 1',
        newPassword: 'another long phrase',
        confirmPassword: 'different long phrase',
      }),
    ).toEqual([{ path: 'confirmPassword', message: 'mismatch' }])
  })

  it('checks the setup code and the setup password against the e-mail', () => {
    expect(
      issuesOf(setupSchema, {
        email: 'admin@example.test',
        setupCode: 'ABCD',
        password: 'short',
        confirmPassword: 'short',
      }),
    ).toEqual([
      { path: 'setupCode', message: 'invalid_setup_code' },
      { path: 'password', message: 'too_short' },
    ])
  })

  it('requires a chosen setup e-mail and rejects using it as the password', () => {
    expect(issuesOf(setupSchema, { email: '', setupCode: 'ABCD-EFGH-IJKL', password: 'long test password', confirmPassword: 'long test password' })).toEqual([
      { path: 'email', message: 'required' },
    ])
    expect(issuesOf(setupSchema, { email: 'admin@example.test', setupCode: 'ABCD-EFGH-IJKL', password: 'admin@example.test', confirmPassword: 'admin@example.test' })).toEqual([
      { path: 'password', message: 'same_as_email' },
    ])
  })

  it('checks the new e-mail format', () => {
    expect(issuesOf(emailChangeSchema, { currentPassword: 'x', newEmail: 'not-an-email' })).toEqual([
      { path: 'newEmail', message: 'invalid_email' },
    ])
  })
})
