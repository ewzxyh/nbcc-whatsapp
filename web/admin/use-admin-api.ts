import { useMemo } from 'react'
import { useSession } from './session'
import type {
  ActivityEntry,
  AdminSettings,
  AdminStatus,
  CheckResult,
  CourseSummary,
  FeedTestResult,
  GrapnelTestResult,
  PasswordChangeResult,
  SettingsPatch,
  WebhookResult,
} from './lib/types'

// Typed calls to /api/admin. Every call goes through the session context (CSRF token, idle timer, 401 handling).
export function useAdminApi() {
  const { request } = useSession()
  return useMemo(
    () => ({
      status: () => request<AdminStatus>('GET', '/status'),
      settings: () => request<AdminSettings>('GET', '/settings'),
      saveSettings: (patch: SettingsPatch) => request<AdminSettings>('PUT', '/settings', patch),
      testFeed: () => request<FeedTestResult>('POST', '/settings/test-feed'),
      testGrapnel: () => request<GrapnelTestResult>('POST', '/settings/test-grapnel'),
      registerWebhook: () => request<WebhookResult>('POST', '/webhook/register'),
      courses: () => request<CourseSummary[]>('GET', '/courses'),
      check: () => request<CheckResult>('POST', '/check'),
      activity: () => request<ActivityEntry[]>('GET', '/activity'),
      changePassword: (currentPassword: string, newPassword: string) =>
        request<PasswordChangeResult>('POST', '/password', { currentPassword, newPassword }),
      revokeOtherSessions: () => request<void>('POST', '/sessions/revoke-others'),
    }),
    [request],
  )
}
