import type { ReactNode } from 'react'
import { CheckCircleIcon, DangerCircleIcon, InfoCircleIcon } from '@solar-icons/react/linear'
import { cn } from '@/lib/utils'

type Tone = 'success' | 'error' | 'info'

interface NoticeProps {
  tone?: Tone
  children: ReactNode
  className?: string
}

// Errors use role="alert". Success and info use role="status". Each has an icon and text, never colour alone.
export function Notice({ tone = 'info', children, className }: NoticeProps) {
  const Icon = tone === 'error' ? DangerCircleIcon : tone === 'success' ? CheckCircleIcon : InfoCircleIcon
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={cn(
        'flex gap-3 rounded-2xl border px-4 py-3 text-base text-foreground',
        tone === 'error' && 'border-destructive/40 bg-destructive-subtle',
        tone === 'success' && 'border-primary/30 bg-accent/40',
        tone === 'info' && 'border-border bg-surface',
        className,
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn('mt-0.5 size-5 shrink-0', tone === 'error' ? 'text-destructive' : 'text-primary')}
      />
      <div className="min-w-0 break-words">{children}</div>
    </div>
  )
}
