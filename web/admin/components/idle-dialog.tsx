import { useRef, useState } from 'react'
import { AlertDialog } from 'radix-ui'
import { Button } from '@/components/ui/button'
import { countdownText } from '../lib/session-timer'

interface IdleDialogProps {
  open: boolean
  minutesLeft: number
  onContinue: () => Promise<boolean>
  onLogout: () => Promise<void>
}

// Not dismissible with Escape or by clicking outside: the user must choose one of the two actions.
export function IdleDialog({ open, minutesLeft, onContinue, onLogout }: IdleDialogProps) {
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const continueRef = useRef<HTMLButtonElement>(null)

  async function handleContinue() {
    setBusy(true)
    const extended = await onContinue()
    setBusy(false)
    setFailed(!extended)
  }

  return (
    <AlertDialog.Root open={open}>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/40" />
        <AlertDialog.Content
          className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-border bg-card p-6 text-card-foreground"
          onEscapeKeyDown={(event) => event.preventDefault()}
          onOpenAutoFocus={(event) => {
            event.preventDefault()
            continueRef.current?.focus()
          }}
        >
          <AlertDialog.Title className="text-lg font-semibold text-foreground">
            Sua sessão vai expirar
          </AlertDialog.Title>
          <AlertDialog.Description className="mt-2 text-base text-muted-foreground">
            Você ficou sem atividade por um tempo. Para proteger os dados, a sessão será encerrada
            automaticamente.
          </AlertDialog.Description>
          <p className="mt-3 text-base font-medium text-foreground">{countdownText(minutesLeft)}</p>
          {failed ? (
            <p role="alert" className="mt-3 text-sm font-medium text-destructive">
              Não foi possível continuar agora. Tente de novo.
            </p>
          ) : null}
          <div className="mt-6 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button variant="outline" onClick={() => void onLogout()} disabled={busy}>
              Sair
            </Button>
            <Button ref={continueRef} onClick={() => void handleContinue()} disabled={busy}>
              Continuar conectado
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  )
}
