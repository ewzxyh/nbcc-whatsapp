import type { ReactNode } from 'react'
import { TIMEZONE_NOTE } from '../lib/dates'

interface PageHeaderProps {
  title: string
  description?: ReactNode
  showTimezone?: boolean
}

// One h1 per page. The timezone note appears once on pages that show times.
export function PageHeader({ title, description, showTimezone = false }: PageHeaderProps) {
  return (
    <header className="space-y-2">
      <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">{title}</h1>
      {description ? <p className="text-base text-muted-foreground">{description}</p> : null}
      {showTimezone ? <p className="text-sm font-medium text-muted-foreground">{TIMEZONE_NOTE}</p> : null}
    </header>
  )
}
