import { z } from 'zod'
import {
  INSTRUCTOR_MAX_ENTRIES,
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

// Zod schemas for react-hook-form. Issue messages are error codes, translated at display time.

const secretAction = z.enum(['keep', 'replace', 'remove'])

function addIssue(ctx: z.RefinementCtx, path: PropertyKey[], code: string | null) {
  if (code) ctx.addIssue({ code: 'custom', path, message: code })
}

export const loginSchema = z
  .object({ email: z.string(), password: z.string() })
  .superRefine((values, ctx) => {
    addIssue(ctx, ['email'], validateEmail(values.email))
    addIssue(ctx, ['password'], values.password === '' ? 'required' : null)
  })

export const setupSchema = z
  .object({ email: z.string(), setupCode: z.string(), password: z.string(), confirmPassword: z.string() })
  .superRefine((values, ctx) => {
    addIssue(ctx, ['email'], validateEmail(values.email))
    addIssue(ctx, ['setupCode'], validateSetupCode(values.setupCode))
    addIssue(ctx, ['password'], validatePassword(values.password, values.email))
    addIssue(ctx, ['confirmPassword'], values.confirmPassword !== values.password ? 'mismatch' : null)
  })

export function passwordChangeSchema(email: string) {
  return z
    .object({ currentPassword: z.string(), newPassword: z.string(), confirmPassword: z.string() })
    .superRefine((values, ctx) => {
      addIssue(ctx, ['currentPassword'], values.currentPassword === '' ? 'required' : null)
      addIssue(ctx, ['newPassword'], validatePassword(values.newPassword, email))
      if (values.newPassword !== '' && values.newPassword === values.currentPassword) {
        addIssue(ctx, ['newPassword'], 'same_as_password')
      }
      addIssue(ctx, ['confirmPassword'], values.confirmPassword !== values.newPassword ? 'mismatch' : null)
    })
}

export const emailChangeSchema = z
  .object({ currentPassword: z.string(), newEmail: z.string() })
  .superRefine((values, ctx) => {
    addIssue(ctx, ['currentPassword'], values.currentPassword === '' ? 'required' : null)
    addIssue(ctx, ['newEmail'], validateEmail(values.newEmail))
  })

export const agendaFormSchema = z
  .object({
    nbccFeedUrl: z.string(),
    nbccFeedUrlAction: secretAction,
    agendaCacheMinutes: z.string(),
  })
  .superRefine((values, ctx) => {
    if (values.nbccFeedUrlAction === 'replace') {
      addIssue(ctx, ['nbccFeedUrl'], validateSecretReplacement(values.nbccFeedUrl, validateFeedUrl))
    }
    addIssue(ctx, ['agendaCacheMinutes'], validateMinutes(values.agendaCacheMinutes))
  })

export const whatsappFormSchema = z
  .object({
    sendEnabled: z.boolean(),
    pollIntervalMinutes: z.string(),
    grapnelApiKey: z.string(),
    grapnelApiKeyAction: secretAction,
    grapnelInstanceId: z.string(),
    grapnelGroupJid: z.string(),
    grapnelGroupName: z.string(),
    publicBaseUrl: z.string(),
    webhookSecret: z.string(),
    webhookSecretAction: secretAction,
  })
  .superRefine((values, ctx) => {
    addIssue(ctx, ['pollIntervalMinutes'], validateMinutes(values.pollIntervalMinutes))
    if (values.grapnelApiKeyAction === 'replace') {
      addIssue(ctx, ['grapnelApiKey'], values.grapnelApiKey.trim() === '' ? 'required' : null)
    }
    addIssue(ctx, ['grapnelInstanceId'], validateOptionalUuid(values.grapnelInstanceId))
    addIssue(ctx, ['grapnelGroupJid'], validateOptionalGroupJid(values.grapnelGroupJid))
    addIssue(ctx, ['grapnelGroupName'], validateGroupName(values.grapnelGroupName))
    addIssue(ctx, ['publicBaseUrl'], validatePublicBaseUrl(values.publicBaseUrl))
    if (values.webhookSecretAction === 'replace') {
      addIssue(ctx, ['webhookSecret'], validateSecretReplacement(values.webhookSecret, validateWebhookSecret))
    }
  })

export const instructorFormSchema = z
  .object({
    entries: z.array(z.object({ code: z.string(), title: z.string(), instructor: z.string() })),
  })
  .superRefine((values, ctx) => {
    const named = values.entries.filter((entry) => entry.instructor.trim() !== '')
    if (named.length > INSTRUCTOR_MAX_ENTRIES) {
      addIssue(ctx, ['entries'], 'invalid_instructor_map')
    }
    values.entries.forEach((entry, index) => {
      addIssue(ctx, ['entries', index, 'instructor'], validateInstructorName(entry.instructor))
    })
  })
