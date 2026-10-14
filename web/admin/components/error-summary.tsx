import { useEffect, useRef } from 'react'
import { DangerCircleIcon } from '@solar-icons/react/linear'

export interface ErrorSummaryItem {
  // Input id for a field error. Omit for a form-level message.
  id?: string
  label?: string
  message: string
}

interface ErrorSummaryProps {
  title?: string
  items: ErrorSummaryItem[]
}

// Shown at the top of a form after a failed submit. Each field error links to its input (WCAG 3.3.1).
// It takes focus when it appears, so screen reader users hear it.
export function ErrorSummary({ title = 'Revise os campos destacados', items }: ErrorSummaryProps) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    ref.current?.focus()
  }, [])

  function focusField(id: string) {
    const element = document.getElementById(id)
    if (element) {
      element.focus()
      element.scrollIntoView({ block: 'center' })
    }
  }

  return (
    <div
      ref={ref}
      role="alert"
      tabIndex={-1}
      className="space-y-2 rounded-2xl border border-destructive/40 bg-destructive-subtle px-4 py-3 text-base text-foreground"
    >
      <p className="flex items-center gap-2 font-semibold">
        <DangerCircleIcon aria-hidden="true" className="size-5 shrink-0 text-destructive" />
        {title}
      </p>
      <ul className="list-disc space-y-1 pl-6">
        {items.map((item, index) => (
          <li key={`${item.id ?? 'form'}-${index}`} className="break-words">
            {item.id ? (
              <a
                href={`#${item.id}`}
                className="font-medium text-foreground underline underline-offset-4"
                onClick={(event) => {
                  event.preventDefault()
                  focusField(item.id as string)
                }}
              >
                {item.label ? `${item.label}: ` : ''}
                {item.message}
              </a>
            ) : (
              item.message
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}
