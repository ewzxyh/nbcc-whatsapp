import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getProgressSnapshot,
  PROGRESS_STORAGE_KEY,
  PROGRESS_TTL_MS,
  progressKeyOf,
  resetProgressCache,
  setProgress,
  statusFor,
} from './progress'

class MemoryStorage {
  private data = new Map<string, string>()
  getItem(key: string) {
    return this.data.get(key) ?? null
  }
  setItem(key: string, value: string) {
    this.data.set(key, value)
  }
  removeItem(key: string) {
    this.data.delete(key)
  }
}

const original = globalThis.localStorage

beforeEach(() => {
  vi.stubGlobal('localStorage', new MemoryStorage())
  resetProgressCache()
})

afterEach(() => {
  vi.unstubAllGlobals()
  if (original) vi.stubGlobal('localStorage', original)
})

describe('progress storage', () => {
  it('stores in progress and done, and clears back to not started', () => {
    setProgress('activity:a1', 'in_progress', 1_000)
    expect(statusFor(getProgressSnapshot(), 'activity:a1')).toBe('in_progress')
    setProgress('activity:a1', 'done', 2_000)
    expect(statusFor(getProgressSnapshot(), 'activity:a1')).toBe('done')
    setProgress('activity:a1', 'not_started', 3_000)
    expect(statusFor(getProgressSnapshot(), 'activity:a1')).toBe('not_started')
    expect(JSON.parse(localStorage.getItem(PROGRESS_STORAGE_KEY) ?? '{}')).toEqual({})
  })

  it('keeps each record for 365 days from its last change', () => {
    const now = 1_000_000
    setProgress('activity:a1', 'done', now)
    const stored = JSON.parse(localStorage.getItem(PROGRESS_STORAGE_KEY) ?? '{}')
    expect(stored['activity:a1'].expiresAt).toBe(now + PROGRESS_TTL_MS)
    expect(PROGRESS_TTL_MS).toBe(365 * 24 * 60 * 60 * 1000)
  })

  it('drops expired and malformed records when reading', () => {
    const now = Date.now()
    localStorage.setItem(
      PROGRESS_STORAGE_KEY,
      JSON.stringify({
        old: { status: 'done', expiresAt: now - 1 },
        fresh: { status: 'in_progress', expiresAt: now + PROGRESS_TTL_MS },
        bad: { status: 'maybe', expiresAt: now + PROGRESS_TTL_MS },
        nothing: 'x',
      }),
    )
    resetProgressCache()
    const snapshot = getProgressSnapshot()
    expect(Object.keys(snapshot)).toEqual(['fresh'])
  })

  it('survives storage that throws, keeping the change for the open page', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('quota')
      },
    })
    resetProgressCache()
    expect(() => setProgress('activity:a1', 'done')).not.toThrow()
    expect(statusFor(getProgressSnapshot(), 'activity:a1')).toBe('done')
  })
})

describe('progressKeyOf', () => {
  it('shares one key between the phases of one assessment', () => {
    expect(progressKeyOf({ kind: 'activity', key: 'k1', activityId: 'abc' } as never)).toBe('activity:abc')
    expect(progressKeyOf({ kind: 'event', item: { id: 'e9' } } as never)).toBe('event:e9')
  })
})
