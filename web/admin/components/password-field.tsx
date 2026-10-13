import { useState, type ComponentProps, type ReactNode, type Ref } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

type PasswordFieldProps = Omit<ComponentProps<'input'>, 'id' | 'type'> & {
  id: string
  label: string
  help?: ReactNode
  error?: string | null
  revealLabel?: string
  ref?: Ref<HTMLInputElement>
}

// Show/hide is a toggle button with aria-pressed. Its text stays the same, so the state is announced once.
export function PasswordField({
  id,
  label,
  help,
  error,
  revealLabel = 'Mostrar senha',
  ref,
  ...props
}: PasswordFieldProps) {
  const [visible, setVisible] = useState(false)
  const helpId = help ? `${id}-help` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [helpId, errorId].filter(Boolean).join(' ') || undefined
  return (
    <div className="space-y-2">
      <Label htmlFor={id} className="text-base">
        {label}
      </Label>
      {help ? (
        <p id={helpId} className="text-sm text-muted-foreground">
          {help}
        </p>
      ) : null}
      <Input
        id={id}
        ref={ref}
        type={visible ? 'text' : 'password'}
        autoCapitalize="off"
        autoCorrect="off"
        spellCheck={false}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        {...props}
      />
      {error ? (
        <p id={errorId} className="text-sm font-medium text-destructive">
          {error}
        </p>
      ) : null}
      <div className="flex justify-end">
        <Button
          type="button"
          variant="ghost"
          aria-pressed={visible}
          onClick={() => setVisible((current) => !current)}
        >
          {revealLabel}
        </Button>
      </div>
    </div>
  )
}
