// Pure helpers for the idle timer. The server times out 60 minutes after the last request.
// The browser warns at 55 minutes without activity and then logs out at 60.

export const IDLE_TIMEOUT_MS = 60 * 60_000
export const WARNING_AFTER_MS = 55 * 60_000
// User activity sends a lightweight extend at most this often, so the server idle timer follows the user.
export const KEEP_ALIVE_THROTTLE_MS = 60_000

export type IdlePhase = 'active' | 'warning' | 'expired'

export function idlePhase(now: number, lastTouch: number): IdlePhase {
  const idle = now - lastTouch
  if (idle >= IDLE_TIMEOUT_MS) return 'expired'
  if (idle >= WARNING_AFTER_MS) return 'warning'
  return 'active'
}

// Whole minutes left before expiry, rounded up. Used for the countdown text (updated once a minute).
export function minutesUntilExpiry(now: number, lastTouch: number): number {
  const remaining = IDLE_TIMEOUT_MS - (now - lastTouch)
  return Math.max(0, Math.ceil(remaining / 60_000))
}

export function shouldKeepAlive(now: number, lastTouch: number): boolean {
  return now - lastTouch >= KEEP_ALIVE_THROTTLE_MS
}

export function countdownText(minutes: number): string {
  const unit = minutes === 1 ? 'minuto' : 'minutos'
  return `Sua sessão será encerrada em ${minutes} ${unit}.`
}
