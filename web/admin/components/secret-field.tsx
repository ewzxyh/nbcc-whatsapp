import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import type { SecretAction } from '../lib/rules'
import { ConfirmDialog } from './confirm-dialog'
import { ConfiguredBadge } from './status-badge'
import { PasswordField } from './password-field'

interface SecretFieldProps {
  id: string
  label: string
  help?: ReactNode
  configured: boolean
  action: SecretAction
  value: string
  error?: string | null
  removeTitle: string
  onActionChange: (action: SecretAction) => void
  onValueChange: (value: string) => void
  onGenerate?: () => void
  generateLabel?: string
}

// A secret is never shown after saving. The page only learns `configured`.
// keep: nothing sent. replace: the typed value is sent. remove: null is sent (after the user confirms).
export function SecretField({
  id,
  label,
  help,
  configured,
  action,
  value,
  error,
  removeTitle,
  onActionChange,
  onValueChange,
  onGenerate,
  generateLabel = 'Gerar novo segredo',
}: SecretFieldProps) {
  const [confirmRemove, setConfirmRemove] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const keepRef = useRef<HTMLButtonElement>(null)
  const removeRef = useRef<HTMLButtonElement>(null)
  const undoRef = useRef<HTMLButtonElement>(null)
  const focusAfterReplace = useRef(false)
  const focusAfterKeep = useRef(false)
  const showInput = action === 'replace'

  useEffect(() => {
    if (showInput && focusAfterReplace.current) {
      focusAfterReplace.current = false
      inputRef.current?.focus()
    }
    if (action === 'keep' && focusAfterKeep.current) {
      focusAfterKeep.current = false
      keepRef.current?.focus()
    }
  }, [showInput, action])

  function startReplace() {
    if (action === 'replace') {
      inputRef.current?.focus()
      return
    }
    focusAfterReplace.current = true
    onActionChange('replace')
  }

  function cancelReplace() {
    focusAfterKeep.current = true
    onValueChange('')
    onActionChange('keep')
  }

  return (
    <div role="group" aria-label={label} className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        {showInput ? null : <p className="text-base font-medium text-foreground">{label}</p>}
        <ConfiguredBadge configured={configured} />
      </div>
      {help && !showInput ? <p className="text-sm text-muted-foreground">{help}</p> : null}
      {action === 'remove' ? (
        <p className="text-sm font-medium text-foreground">Será removido ao salvar alterações.</p>
      ) : null}
      {showInput ? (
        <PasswordField
          ref={inputRef}
          id={id}
          label={label}
          value={value}
          onChange={(event) => onValueChange(event.target.value)}
          autoComplete="off"
          help={help}
          error={error}
          revealLabel="Mostrar valor"
        />
      ) : null}
      <div className="flex flex-wrap gap-3">
        {action === 'keep' && !configured ? (
          <Button ref={keepRef} type="button" variant="outline" onClick={startReplace}>
            Configurar
          </Button>
        ) : null}
        {action === 'keep' && configured ? (
          <>
            <Button ref={keepRef} type="button" variant="outline" onClick={startReplace}>
              Substituir
            </Button>
            <Button ref={removeRef} type="button" variant="outline" onClick={() => setConfirmRemove(true)}>
              Remover
            </Button>
          </>
        ) : null}
        {action === 'replace' ? (
          <Button type="button" variant="outline" onClick={cancelReplace}>
            Cancelar
          </Button>
        ) : null}
        {action === 'remove' ? (
          <Button ref={undoRef} type="button" variant="outline" onClick={() => {
            focusAfterKeep.current = true
            onActionChange('keep')
          }}>
            Desfazer remoção
          </Button>
        ) : null}
        {onGenerate && action !== 'remove' ? (
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              focusAfterReplace.current = false
              onGenerate()
            }}
          >
            {generateLabel}
          </Button>
        ) : null}
      </div>
      <ConfirmDialog
        open={confirmRemove}
        onOpenChange={setConfirmRemove}
        title={removeTitle}
        description="Você pode salvar um novo valor depois. O valor atual só sai ao salvar alterações."
        confirmLabel="Remover"
        destructive
        onCloseAutoFocus={(event) => {
          const target = undoRef.current ?? removeRef.current
          if (target) {
            event.preventDefault()
            target.focus()
          }
        }}
        onConfirm={() => {
          onValueChange('')
          onActionChange('remove')
        }}
      />
    </div>
  )
}
