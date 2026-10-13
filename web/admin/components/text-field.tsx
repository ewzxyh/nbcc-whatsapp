import type { ComponentProps, ReactNode, Ref } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

type TextFieldProps = Omit<ComponentProps<'input'>, 'id'> & {
  id: string
  label: string
  help?: ReactNode
  error?: string | null
  ref?: Ref<HTMLInputElement>
}

// Label, help text and error are linked to the input with aria-describedby. Errors also set aria-invalid.
export function TextField({ id, label, help, error, ref, ...props }: TextFieldProps) {
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
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        {...props}
      />
      {error ? (
        <p id={errorId} className="text-sm font-medium text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
