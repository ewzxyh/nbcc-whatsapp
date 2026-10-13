import { describe, expect, it } from 'vitest'
import {
  countdownText,
  idlePhase,
  minutesUntilExpiry,
  shouldKeepAlive,
} from './session-timer'

const MINUTE = 60_000
const LAST = 1_000_000

describe('idlePhase', () => {
  it('is active until 55 minutes of inactivity', () => {
    expect(idlePhase(LAST + 55 * MINUTE - 1, LAST)).toBe('active')
    expect(idlePhase(LAST, LAST)).toBe('active')
  })

  it('warns from 55 minutes', () => {
    expect(idlePhase(LAST + 55 * MINUTE, LAST)).toBe('warning')
    expect(idlePhase(LAST + 60 * MINUTE - 1, LAST)).toBe('warning')
  })

  it('expires at 60 minutes', () => {
    expect(idlePhase(LAST + 60 * MINUTE, LAST)).toBe('expired')
    expect(idlePhase(LAST + 90 * MINUTE, LAST)).toBe('expired')
  })
})

describe('minutesUntilExpiry', () => {
  it('counts whole minutes left, rounded up', () => {
    expect(minutesUntilExpiry(LAST, LAST)).toBe(60)
    expect(minutesUntilExpiry(LAST + 55 * MINUTE, LAST)).toBe(5)
    expect(minutesUntilExpiry(LAST + 59.5 * MINUTE, LAST)).toBe(1)
  })

  it('never goes below zero', () => {
    expect(minutesUntilExpiry(LAST + 70 * MINUTE, LAST)).toBe(0)
  })
})

describe('shouldKeepAlive', () => {
  it('sends at most one keep-alive a minute', () => {
    expect(shouldKeepAlive(LAST + MINUTE - 1, LAST)).toBe(false)
    expect(shouldKeepAlive(LAST + MINUTE, LAST)).toBe(true)
  })
})

describe('countdownText', () => {
  it('writes the countdown in pt-BR with the right unit', () => {
    expect(countdownText(5)).toBe('Sua sessão será encerrada em 5 minutos.')
    expect(countdownText(1)).toBe('Sua sessão será encerrada em 1 minuto.')
  })
})
