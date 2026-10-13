import { normalizePublicBaseUrl, type SecretAction } from './rules'
import type { AdminSettings, SettingsPatch } from './types'

// Form values are strings (inputs). Only changed fields are sent, so two pages never overwrite each other.
// Secret fields never carry a stored value into the form. Their action decides what is sent.

// Keep: omit the key. Replace: send the new value. Remove: send null.
export function secretPatchValue(action: SecretAction, value: string): string | null | undefined {
  if (action === 'replace') return value.trim()
  if (action === 'remove') return null
  return undefined
}

export interface AgendaFormValues {
  nbccFeedUrl: string
  nbccFeedUrlAction: SecretAction
  agendaCacheMinutes: string
}

export function agendaFormFromSettings(settings: AdminSettings): AgendaFormValues {
  return {
    nbccFeedUrl: '',
    nbccFeedUrlAction: 'keep',
    agendaCacheMinutes: String(settings.agendaCacheMinutes),
  }
}

export function buildAgendaPatch(values: AgendaFormValues, baseline: AgendaFormValues): SettingsPatch {
  const patch: SettingsPatch = {}
  const feedUrl = secretPatchValue(values.nbccFeedUrlAction, values.nbccFeedUrl)
  if (feedUrl !== undefined) patch.nbccFeedUrl = feedUrl
  if (values.agendaCacheMinutes.trim() !== baseline.agendaCacheMinutes.trim()) {
    patch.agendaCacheMinutes = Number(values.agendaCacheMinutes.trim())
  }
  return patch
}

// Drafts live in memory only. Secret values are never kept there.
export function sanitizeAgendaForDraft(values: AgendaFormValues): AgendaFormValues {
  return { ...values, nbccFeedUrl: '' }
}

export interface WhatsAppFormValues {
  sendEnabled: boolean
  pollIntervalMinutes: string
  grapnelApiKey: string
  grapnelApiKeyAction: SecretAction
  grapnelInstanceId: string
  grapnelGroupJid: string
  grapnelGroupName: string
  publicBaseUrl: string
  webhookSecret: string
  webhookSecretAction: SecretAction
}

export function whatsappFormFromSettings(settings: AdminSettings): WhatsAppFormValues {
  return {
    sendEnabled: settings.sendEnabled,
    pollIntervalMinutes: String(settings.pollIntervalMinutes),
    grapnelApiKey: '',
    grapnelApiKeyAction: 'keep',
    grapnelInstanceId: settings.grapnelInstanceId,
    grapnelGroupJid: settings.grapnelGroupJid,
    grapnelGroupName: settings.grapnelGroupName,
    publicBaseUrl: settings.publicBaseUrl,
    webhookSecret: '',
    webhookSecretAction: 'keep',
  }
}

function changedText(next: string, previous: string): boolean {
  return next.trim() !== previous.trim()
}

export function buildWhatsAppPatch(values: WhatsAppFormValues, baseline: WhatsAppFormValues): SettingsPatch {
  const patch: SettingsPatch = {}
  if (values.sendEnabled !== baseline.sendEnabled) patch.sendEnabled = values.sendEnabled
  if (changedText(values.pollIntervalMinutes, baseline.pollIntervalMinutes)) {
    patch.pollIntervalMinutes = Number(values.pollIntervalMinutes.trim())
  }
  const apiKey = secretPatchValue(values.grapnelApiKeyAction, values.grapnelApiKey)
  if (apiKey !== undefined) patch.grapnelApiKey = apiKey
  if (changedText(values.grapnelInstanceId, baseline.grapnelInstanceId)) {
    patch.grapnelInstanceId = values.grapnelInstanceId.trim()
  }
  if (changedText(values.grapnelGroupJid, baseline.grapnelGroupJid)) {
    patch.grapnelGroupJid = values.grapnelGroupJid.trim()
  }
  if (changedText(values.grapnelGroupName, baseline.grapnelGroupName)) {
    patch.grapnelGroupName = values.grapnelGroupName.trim()
  }
  if (normalizePublicBaseUrl(values.publicBaseUrl) !== normalizePublicBaseUrl(baseline.publicBaseUrl)) {
    patch.publicBaseUrl = normalizePublicBaseUrl(values.publicBaseUrl)
  }
  const webhookSecret = secretPatchValue(values.webhookSecretAction, values.webhookSecret)
  if (webhookSecret !== undefined) patch.webhookSecret = webhookSecret
  return patch
}

export function sanitizeWhatsAppForDraft(values: WhatsAppFormValues): WhatsAppFormValues {
  return { ...values, grapnelApiKey: '', webhookSecret: '' }
}
