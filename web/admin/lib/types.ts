// Shapes from ADMIN_CONTRACT.md. Secret values never come back from the API; only `configured`.

export type SessionState = 'setup' | 'login' | 'authenticated'

export interface SessionResponse {
  state: SessionState
  email: string
  csrfToken?: string
  expiresAt?: string
}

export interface AuthResponse {
  state?: 'authenticated'
  csrfToken: string
  expiresAt: string
}

export interface OmittedEvent {
  title: string
  date: string
  reason: string
}

export interface AdminStatus {
  agenda: {
    updatedAt: string | null
    stale: boolean
    cacheMinutes: number
    omittedEvents: OmittedEvent[]
    lastError: string | null
  }
  whatsapp: {
    sendEnabled: boolean
    pendingJobs: number
    failedJobs: number
    lastCheckAt: string | null
    lastError: string | null
  }
  configured: {
    nbccFeed: boolean
    grapnel: boolean
    webhookSecret: boolean
    publicBaseUrl: boolean
  }
}

export interface SecretSetting {
  secret: true
  configured: boolean
}

export interface AdminSettings {
  nbccFeedUrl: SecretSetting
  grapnelApiKey: SecretSetting
  webhookSecret: SecretSetting
  grapnelInstanceId: string
  grapnelGroupJid: string
  grapnelGroupName: string
  publicBaseUrl: string
  sendEnabled: boolean
  pollIntervalMinutes: number
  agendaCacheMinutes: number
  courseInstructors: Record<string, string>
}

// Secret keys: a string sets the value, null clears it, an omitted key keeps it.
export interface SettingsPatch {
  nbccFeedUrl?: string | null
  grapnelApiKey?: string | null
  webhookSecret?: string | null
  grapnelInstanceId?: string
  grapnelGroupJid?: string
  grapnelGroupName?: string
  publicBaseUrl?: string
  sendEnabled?: boolean
  pollIntervalMinutes?: number
  agendaCacheMinutes?: number
  courseInstructors?: Record<string, string>
}

export interface FeedTestResult {
  ok: true
  events: number
  courses: number
  omittedEvents: number
}

export interface GrapnelTestResult {
  ok: true
  groupName: string
}

export interface WebhookResult {
  ok: true
  url: string
}

export interface CourseSummary {
  code: string
  title: string
  instructor: string | null
  activities: number
}

export interface CheckResult {
  feedChanged: boolean
  queued: boolean
  sendEnabled: boolean
}

export interface ActivityEntry {
  at: string
  action: string
  fields: string[]
}

export interface ExtendResult {
  expiresAt: string
}

export interface PasswordChangeResult {
  csrfToken: string
  expiresAt: string
}
