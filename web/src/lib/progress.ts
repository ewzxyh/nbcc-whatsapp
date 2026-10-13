// Per-browser progress for assessments and events. Stored in localStorage; each record expires 365 days after
// it was last changed. Nothing here is sent to the server. Storage can be blocked (private windows, disabled
// site data), in which case progress lasts only for the open page.

import type { AgendaEntry } from './agenda'

export type ProgressStatus = 'not_started' | 'in_progress' | 'done'

export const PROGRESS_STORAGE_KEY = 'nbcc-agenda-progress'
export const PROGRESS_TTL_MS = 365 * 24 * 60 * 60 * 1000

type StoredStatus = Exclude<ProgressStatus, 'not_started'>
type ProgressMap = Record<string, { status: StoredStatus; expiresAt: number }>

const EMPTY: ProgressMap = {}
const listeners = new Set<() => void>()
let snapshot: ProgressMap | null = null
let storageListening = false

function isStoredStatus(value: unknown): value is StoredStatus {
  return value === 'in_progress' || value === 'done'
}

/** Reads stored records, dropping malformed ones and any that have expired. */
function readStorage(now: number): ProgressMap {
  let raw: string | null
  try {
    raw = globalThis.localStorage?.getItem(PROGRESS_STORAGE_KEY) ?? null
  } catch {
    return EMPTY
  }
  if (!raw) return EMPTY
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return EMPTY
  }
  if (typeof parsed !== 'object' || parsed === null) return EMPTY
  const result: ProgressMap = {}
  for (const [key, value] of Object.entries(parsed)) {
    if (typeof value !== 'object' || value === null) continue
    const record = value as { status?: unknown; expiresAt?: unknown }
    if (!isStoredStatus(record.status) || typeof record.expiresAt !== 'number') continue
    if (record.expiresAt <= now) continue
    result[key] = { status: record.status, expiresAt: record.expiresAt }
  }
  return result
}

function current(): ProgressMap {
  snapshot ??= readStorage(Date.now())
  return snapshot
}

function notify() {
  for (const listener of listeners) listener()
}

function listenForOtherTabs() {
  if (storageListening || typeof globalThis.addEventListener !== 'function') return
  storageListening = true
  globalThis.addEventListener('storage', (event: StorageEvent) => {
    // A null key means another tab called clear(), which affects every record.
    if (event.key !== PROGRESS_STORAGE_KEY && event.key !== null) return
    snapshot = null
    notify()
  })
}

export function subscribeProgress(listener: () => void): () => void {
  listenForOtherTabs()
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function getProgressSnapshot(): ProgressMap {
  return current()
}

export const EMPTY_PROGRESS: ProgressMap = EMPTY

export function statusFor(map: ProgressMap, key: string): ProgressStatus {
  return map[key]?.status ?? 'not_started'
}

/** Sets one item's status. "not_started" removes the record. Storage failures keep the change for this page only. */
export function setProgress(key: string, status: ProgressStatus, now = Date.now()): void {
  // Re-read storage so a change made in another tab since the last read is kept.
  const next: ProgressMap = { ...readStorage(now) }
  if (status === 'not_started') delete next[key]
  else next[key] = { status, expiresAt: now + PROGRESS_TTL_MS }
  snapshot = next
  try {
    globalThis.localStorage?.setItem(PROGRESS_STORAGE_KEY, JSON.stringify(next))
  } catch {
    // Quota or blocked storage: the in-memory copy still reflects the change.
  }
  notify()
}

/** Test helper: forgets the in-memory copy so the next read goes back to storage. */
export function resetProgressCache(): void {
  snapshot = null
}

/** Storage key for one card. Phases of one assessment share a key, so their progress is one value. */
export function progressKeyOf(entry: AgendaEntry): string {
  return entry.kind === 'activity' ? `activity:${entry.activityId ?? entry.key}` : `event:${entry.item.id}`
}
