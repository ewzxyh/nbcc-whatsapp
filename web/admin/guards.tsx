import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { AuthShell } from './components/auth-shell'
import { LoadErrorState, LoadingState } from './components/load-state'
import { useSession } from './session'

// Only in-app paths may be used as a return target after sign-in.
function returnPathFrom(state: unknown): string {
  const from =
    typeof state === 'object' && state !== null && 'from' in state
      ? (state as { from: unknown }).from
      : undefined
  if (typeof from !== 'string' || !from.startsWith('/') || from.startsWith('//')) return '/'
  if (from.startsWith('/entrar') || from.startsWith('/configurar')) return '/'
  return from
}

function BootState() {
  const { phase, retryBootstrap } = useSession()
  return (
    <AuthShell>
      {phase === 'error' ? <LoadErrorState onRetry={retryBootstrap} /> : <LoadingState />}
    </AuthShell>
  )
}

// /entrar: only for signed-out users.
export function GuestOnly({ children }: { children: ReactNode }) {
  const { phase } = useSession()
  const location = useLocation()
  if (phase === 'loading' || phase === 'error') return <BootState />
  if (phase === 'setup') return <Navigate to="/configurar" replace />
  if (phase === 'authenticated') return <Navigate to={returnPathFrom(location.state)} replace />
  return children
}

// /configurar: only before the admin account exists.
export function SetupOnly({ children }: { children: ReactNode }) {
  const { phase } = useSession()
  if (phase === 'loading' || phase === 'error') return <BootState />
  if (phase === 'login') return <Navigate to="/entrar" replace />
  if (phase === 'authenticated') return <Navigate to="/" replace />
  return children
}

// Every other admin page. An expired session lands here and sends the user to /entrar with the pathname.
export function RequireAuth({ children }: { children: ReactNode }) {
  const { phase } = useSession()
  const location = useLocation()
  if (phase === 'loading' || phase === 'error') return <BootState />
  if (phase === 'setup') return <Navigate to="/configurar" replace />
  if (phase === 'login') {
    return <Navigate to="/entrar" replace state={{ from: location.pathname }} />
  }
  return children
}
