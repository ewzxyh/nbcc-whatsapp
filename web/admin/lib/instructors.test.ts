import { describe, expect, it } from 'vitest'
import { buildCourseInstructors, buildInstructorPatch, instructorRowsFrom } from './instructors'
import { agendaFormFromSettings, buildAgendaPatch, buildWhatsAppPatch, secretPatchValue, whatsappFormFromSettings } from './settings-form'
import { generateWebhookSecret } from './secrets'
import type { AdminSettings } from './types'

const SETTINGS: AdminSettings = {
  nbccFeedUrl: { secret: true, configured: true },
  grapnelApiKey: { secret: true, configured: false },
  webhookSecret: { secret: true, configured: true },
  grapnelInstanceId: '11111111-2222-4333-8444-555555555555',
  grapnelGroupJid: '120363000000000000@g.us',
  grapnelGroupName: 'Grupo de teste',
  publicBaseUrl: 'https://nbcc.example.com',
  sendEnabled: false,
  pollIntervalMinutes: 5,
  agendaCacheMinutes: 60,
  courseInstructors: { 'Linux I': 'Sample Instructor' },
}

describe('course instructors payload', () => {
  it('drops empty names, trims, and keeps the map', () => {
    expect(
      buildCourseInstructors([
        { code: 'Linux I', title: '', instructor: '  Jane Doe  ' },
        { code: 'Cloud I', title: '', instructor: '   ' },
        { code: ' Net I ', title: '', instructor: 'John Smith' },
      ]),
    ).toEqual({ 'Linux I': 'Jane Doe', 'Net I': 'John Smith' })
  })

  it('sends the full map under courseInstructors', () => {
    expect(buildInstructorPatch([{ code: 'Linux I', title: '', instructor: 'Jane' }])).toEqual({
      courseInstructors: { 'Linux I': 'Jane' },
    })
  })

  it('keeps saved names for courses that are no longer in the feed', () => {
    const rows = instructorRowsFrom(
      [{ code: 'Linux I', title: 'Linux Fundamentals', instructor: null, activities: 3 }],
      { 'Linux I': 'Sample Instructor', 'Old I': 'Former Instructor' },
    )
    expect(rows).toEqual([
      { code: 'Linux I', title: 'Linux Fundamentals', instructor: 'Sample Instructor' },
      { code: 'Old I', title: '', instructor: 'Former Instructor' },
    ])
    expect(buildCourseInstructors(rows)).toEqual({ 'Linux I': 'Sample Instructor', 'Old I': 'Former Instructor' })
  })
})

describe('secret actions', () => {
  it('keep omits the key, replace sends the trimmed value, remove sends null', () => {
    expect(secretPatchValue('keep', 'ignored')).toBeUndefined()
    expect(secretPatchValue('replace', '  new-value  ')).toBe('new-value')
    expect(secretPatchValue('remove', '')).toBeNull()
  })

  it('generates 64 lowercase hex characters', () => {
    expect(generateWebhookSecret()).toMatch(/^[0-9a-f]{64}$/)
    expect(generateWebhookSecret()).not.toBe(generateWebhookSecret())
  })
})

describe('agenda patch', () => {
  it('sends nothing when nothing changed', () => {
    const baseline = agendaFormFromSettings(SETTINGS)
    expect(buildAgendaPatch(baseline, baseline)).toEqual({})
  })

  it('sends only the feed and the changed cache interval', () => {
    const baseline = agendaFormFromSettings(SETTINGS)
    expect(
      buildAgendaPatch(
        { nbccFeedUrl: ' https://x ', nbccFeedUrlAction: 'replace', agendaCacheMinutes: '120' },
        baseline,
      ),
    ).toEqual({ nbccFeedUrl: 'https://x', agendaCacheMinutes: 120 })
  })
})

describe('WhatsApp patch', () => {
  it('sends only the keys that changed and never the secrets left on keep', () => {
    const baseline = whatsappFormFromSettings(SETTINGS)
    expect(buildWhatsAppPatch({ ...baseline, sendEnabled: true }, baseline)).toEqual({ sendEnabled: true })
    expect(
      buildWhatsAppPatch({ ...baseline, publicBaseUrl: 'https://nbcc.example.com/' }, baseline),
    ).toEqual({})
  })

  it('sends the webhook secret only when it is replaced or removed', () => {
    const baseline = whatsappFormFromSettings(SETTINGS)
    expect(
      buildWhatsAppPatch({ ...baseline, webhookSecret: 'x'.repeat(32), webhookSecretAction: 'replace' }, baseline),
    ).toEqual({ webhookSecret: 'x'.repeat(32) })
    expect(buildWhatsAppPatch({ ...baseline, webhookSecretAction: 'remove' }, baseline)).toEqual({
      webhookSecret: null,
    })
  })
})
