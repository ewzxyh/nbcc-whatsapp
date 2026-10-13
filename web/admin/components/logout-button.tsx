import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { useDrafts } from '../drafts'
import { useSession } from '../session'
import { ConfirmDialog } from './confirm-dialog'

// Signs out. If unsaved edits exist, the user is asked first, because signing out clears them.
export function LogoutButton({ variant = 'outline' }: { variant?: 'outline' | 'ghost' }) {
  const { logout } = useSession()
  const drafts = useDrafts()
  const [askFirst, setAskFirst] = useState(false)
  const [busy, setBusy] = useState(false)

  async function signOut() {
    setBusy(true)
    await logout()
    setBusy(false)
  }

  return (
    <>
      <Button
        variant={variant}
        disabled={busy}
        onClick={() => {
          if (drafts.hasAny()) setAskFirst(true)
          else void signOut()
        }}
      >
        Sair
      </Button>
      <ConfirmDialog
        open={askFirst}
        onOpenChange={setAskFirst}
        title="Sair sem salvar?"
        description="Você tem alterações não salvas. Se sair agora, elas serão perdidas."
        confirmLabel="Sair sem salvar"
        cancelLabel="Continuar editando"
        destructive
        onConfirm={() => void signOut()}
      />
    </>
  )
}
