import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'
import { IdleDialog } from './components/idle-dialog'
import { useDrafts } from './drafts'
import { apiRequest, isSessionError, type HttpMethod } from './lib/api'
import { messageForCode } from './lib/errors'
import { idlePhase, minutesUntilExpiry, shouldKeepAlive } from './lib/session-timer'
import type { AuthResponse, ExtendResult, SessionResponse } from './lib/types'

export type SessionPhase = 'loading' | 'error' | 'setup' | 'login' | 'authenticated'

interface SessionState {
  phase: SessionPhase
  email: string
  notice: string | null
  warningOpen: boolean
  minutesLeft: number
}

export interface SessionValue {
  phase: SessionPhase
  email: string
  notice: string | null
  warningOpen: boolean
  minutesLeft: number
  // Calls /api/admin/<path>. Sends the CSRF token on writes and ends the session on 401.
  request: <T>(method: HttpMethod, path: string, body?: unknown) => Promise<T>
  completeAuth: (auth: AuthResponse) => void
  setEmail: (email: string) => void
  endSession: (notice: string | null) => void
  logout: () => Promise<void>
  extendSession: () => Promise<boolean>
  retryBootstrap: () => void
  // True while the session is ending, so route guards may leave a dirty form without asking.
  endingRef: RefObject<boolean>
}

const SessionContext = createContext<SessionValue | null>(null)

const SESSION_EXPIRED_NOTICE = messageForCode('session_expired')
const IDLE_CHECK_MS = 15_000

export function SessionProvider({ children }: { children: ReactNode }) {
  const drafts = useDrafts()
  const [state, setState] = useState<SessionState>({
    phase: 'loading',
    email: '',
    notice: null,
    warningOpen: false,
    minutesLeft: 60,
  })
  const csrfRef = useRef<string | null>(null)
  const lastTouchRef = useRef(Date.now())
  const endingRef = useRef(false)
  const warningRef = useRef(false)
  const extendingRef = useRef(false)
  warningRef.current = state.warningOpen

  const endSession = useCallback((notice: string | null) => {
    endingRef.current = true
    csrfRef.current = null
    setState((prev) => ({
      ...prev,
      phase: prev.phase === 'authenticated' ? 'login' : prev.phase,
      notice,
      warningOpen: false,
    }))
  }, [])

  const request = useCallback(
    <T,>(method: HttpMethod, path: string, body?: unknown): Promise<T> =>
      apiRequest<T>(path, { method, body, csrfToken: csrfRef.current }).then(
        (result) => {
          lastTouchRef.current = Date.now()
          return result
        },
        (error: unknown) => {
          if (isSessionError(error)) endSession(SESSION_EXPIRED_NOTICE)
          throw error
        },
      ),
    [endSession],
  )

  const loadSession = useCallback(async () => {
    setState((prev) => ({ ...prev, phase: 'loading' }))
    try {
      const session = await apiRequest<SessionResponse>('/session')
      if (session.state === 'authenticated' && session.csrfToken) {
        csrfRef.current = session.csrfToken
        lastTouchRef.current = Date.now()
        endingRef.current = false
        setState((prev) => ({
          ...prev,
          phase: 'authenticated',
          email: session.email,
          notice: null,
          warningOpen: false,
        }))
        return
      }
      csrfRef.current = null
      setState((prev) => ({
        ...prev,
        phase: session.state === 'setup' ? 'setup' : 'login',
        email: session.email,
        notice: null,
        warningOpen: false,
      }))
    } catch {
      setState((prev) => ({ ...prev, phase: 'error' }))
    }
  }, [])

  useEffect(() => {
    void loadSession()
  }, [loadSession])

  const completeAuth = useCallback((auth: AuthResponse) => {
    csrfRef.current = auth.csrfToken
    lastTouchRef.current = Date.now()
    endingRef.current = false
    setState((prev) => ({ ...prev, phase: 'authenticated', email: auth.email ?? prev.email, notice: null, warningOpen: false }))
  }, [])

  const setEmail = useCallback((email: string) => {
    setState((prev) => ({ ...prev, email }))
  }, [])

  const extendSession = useCallback(async (): Promise<boolean> => {
    if (extendingRef.current) return false
    extendingRef.current = true
    try {
      await request<ExtendResult>('POST', '/session/extend')
      setState((prev) => ({ ...prev, warningOpen: false }))
      return true
    } catch {
      return false
    } finally {
      extendingRef.current = false
    }
  }, [request])

  const logout = useCallback(async () => {
    try {
      await apiRequest('/logout', { method: 'POST', csrfToken: csrfRef.current })
    } catch {
      // The local session ends either way.
    }
    csrfRef.current = null
    endingRef.current = true
    drafts.clearAll()
    setState((prev) => ({ ...prev, phase: 'login', notice: null, warningOpen: false }))
  }, [drafts])

  const retryBootstrap = useCallback(() => {
    void loadSession()
  }, [loadSession])

  // Idle timer: warn at 55 minutes of inactivity, end the session at 60.
  useEffect(() => {
    if (state.phase !== 'authenticated') return
    const check = () => {
      const now = Date.now()
      const last = lastTouchRef.current
      const phase = idlePhase(now, last)
      if (phase === 'expired') {
        endSession(SESSION_EXPIRED_NOTICE)
        return
      }
      const warningOpen = phase === 'warning'
      const minutesLeft = minutesUntilExpiry(now, last)
      setState((prev) =>
        prev.warningOpen === warningOpen && prev.minutesLeft === minutesLeft
          ? prev
          : { ...prev, warningOpen, minutesLeft },
      )
    }
    check()
    const timer = window.setInterval(check, IDLE_CHECK_MS)
    return () => window.clearInterval(timer)
  }, [state.phase, endSession])

  // User activity keeps the server idle timer in step, at most once a minute, and never behind the warning.
  useEffect(() => {
    if (state.phase !== 'authenticated') return
    const onActivity = () => {
      if (warningRef.current || !shouldKeepAlive(Date.now(), lastTouchRef.current)) return
      void extendSession()
    }
    const events = ['pointerdown', 'keydown', 'input'] as const
    for (const name of events) window.addEventListener(name, onActivity, { passive: true })
    return () => {
      for (const name of events) window.removeEventListener(name, onActivity)
    }
  }, [state.phase, extendSession])

  const value = useMemo<SessionValue>(
    () => ({
      phase: state.phase,
      email: state.email,
      notice: state.notice,
      warningOpen: state.warningOpen,
      minutesLeft: state.minutesLeft,
      request,
      completeAuth,
      setEmail,
      endSession,
      logout,
      extendSession,
      retryBootstrap,
      endingRef,
    }),
    [state, request, completeAuth, setEmail, endSession, logout, extendSession, retryBootstrap],
  )

  return (
    <SessionContext.Provider value={value}>
      {children}
      {state.phase === 'authenticated' ? (
        <IdleDialog
          open={state.warningOpen}
          minutesLeft={state.minutesLeft}
          onContinue={extendSession}
          onLogout={logout}
        />
      ) : null}
    </SessionContext.Provider>
  )
}

export function useSession(): SessionValue {
  const value = useContext(SessionContext)
  if (!value) throw new Error('useSession must be used inside SessionProvider')
  return value
}
