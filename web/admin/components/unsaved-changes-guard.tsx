import { useEffect, useRef } from 'react'
import { useBlocker } from 'react-router'
import { useSession } from '../session'
import { ConfirmDialog } from './confirm-dialog'

interface UnsavedChangesGuardProps {
  dirty: boolean
  // Called when the user chooses to leave. Discards the draft for this form.
  onDiscard: () => void
}

// Blocks in-app navigation while a form is dirty and asks first. Session expiry is not blocked (the user
// is already signed out), and the draft is kept in memory so a re-login can continue.
export function UnsavedChangesGuard({ dirty, onDiscard }: UnsavedChangesGuardProps) {
  const { endingRef } = useSession()
  const dirtyRef = useRef(dirty)
  dirtyRef.current = dirty
  const leavingRef = useRef(false)

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirtyRef.current && !endingRef.current && currentLocation.pathname !== nextLocation.pathname,
  )

  useEffect(() => {
    if (!dirty) return
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [dirty])

  const blocked = blocker.state === 'blocked'

  return (
    <ConfirmDialog
      open={blocked}
      onOpenChange={(open) => {
        if (!open && !leavingRef.current && blocker.state === 'blocked') blocker.reset()
      }}
      title="Sair sem salvar?"
      description="Você tem alterações não salvas. Se sair agora, elas serão perdidas."
      confirmLabel="Sair sem salvar"
      cancelLabel="Continuar editando"
      destructive
      onConfirm={() => {
        leavingRef.current = true
        onDiscard()
        if (blocker.state === 'blocked') blocker.proceed()
      }}
    />
  )
}
