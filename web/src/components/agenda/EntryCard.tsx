import { memo } from 'react'
import { Clock, Lock, LockOpen, MapPin, type LucideIcon } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
  CAMPUS_EVENT_LABEL,
  describePhase,
  formatEventTime,
  formatShortDay,
  type AgendaEntry,
} from '@/lib/agenda'
import type { Phase } from '@/lib/api'

const PHASE_ICONS: Record<Phase, LucideIcon> = {
  available: LockOpen,
  due: Clock,
  closes: Lock,
}

interface EntryCardProps {
  entry: AgendaEntry
  showDate?: boolean
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

function EntryCardBase({ entry, showDate = false, onSelect }: EntryCardProps) {
  return (
    <li>
      <button
        type="button"
        onClick={(event) => onSelect(entry, event.currentTarget)}
        className="block w-full rounded-2xl border border-input bg-card p-4 text-left transition-colors hover:border-primary hover:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background focus-visible:outline-none"
      >
        <span className="flex flex-wrap items-center gap-2">
          {entry.kind === 'event' ? (
            <Badge variant="outline" className="border-input text-foreground">
              {CAMPUS_EVENT_LABEL}
            </Badge>
          ) : (
            <Badge className="bg-accent text-accent-foreground">{entry.course ?? 'Course'}</Badge>
          )}
          {showDate && <span className="text-base text-muted-foreground">{formatShortDay(entry.date)}</span>}
        </span>
        <span className="mt-2 block text-lg leading-snug font-semibold break-words text-foreground">
          {entry.title}
        </span>
        <span className="mt-3 flex flex-col gap-1.5 text-base text-muted-foreground tabular-nums">
          {entry.kind === 'activity' ? (
            entry.phases.map((phase) => {
              const Icon = PHASE_ICONS[phase.phase ?? 'due']
              return (
                <span key={phase.id} className="flex items-center gap-2">
                  <Icon aria-hidden="true" className="size-4 shrink-0" />
                  <span>{describePhase(phase, entry.date)}</span>
                </span>
              )
            })
          ) : (
            <>
              <span className="flex items-center gap-2">
                <Clock aria-hidden="true" className="size-4 shrink-0" />
                <span>{formatEventTime(entry.item)}</span>
              </span>
              {entry.item.location && (
                <span className="flex items-start gap-2">
                  <MapPin aria-hidden="true" className="mt-1 size-4 shrink-0" />
                  <span className="break-words">{entry.item.location}</span>
                </span>
              )}
            </>
          )}
        </span>
      </button>
    </li>
  )
}

export const EntryCard = memo(EntryCardBase)
