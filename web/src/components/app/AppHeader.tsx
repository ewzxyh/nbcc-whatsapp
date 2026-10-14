import { RefreshIcon } from '@solar-icons/react/linear'
import { Button } from '@/components/ui/button'
import { formatUpdated, TIMEZONE_NOTE } from '@/lib/agenda'
import { cn } from '@/lib/utils'

interface AppHeaderProps {
  updatedAt: string | undefined
  now: number
  refreshing: boolean
  onRefresh: () => void
}

export function AppHeader({ updatedAt, now, refreshing, onRefresh }: AppHeaderProps) {
  return (
    <header className="space-y-4">
      <div className="space-y-1">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">NBCC Agenda</h1>
        <p className="text-base text-muted-foreground">Upcoming work and campus events for NBCC students</p>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3 rounded-2xl border border-border bg-surface px-4 py-3">
        <div className="space-y-0.5 text-base">
          <p className="font-medium text-foreground">{TIMEZONE_NOTE}</p>
          {updatedAt && (
            <p className="text-muted-foreground">
              <time dateTime={updatedAt}>{formatUpdated(updatedAt, now)}</time>
            </p>
          )}
        </div>
        <Button variant="outline" onClick={onRefresh}>
          <RefreshIcon aria-hidden="true" className={cn(refreshing && 'animate-spin')} />
          Refresh
        </Button>
      </div>
    </header>
  )
}
