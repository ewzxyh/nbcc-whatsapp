import { memo } from 'react'
import { MapPin, Clock } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/agenda/CopyButton'
import { PhaseIcon, PhaseTimer, TimeLeftMeter } from '@/components/agenda/DeadlineMeter'
import { ProgressControl } from '@/components/agenda/ProgressControl'
import {
  CAMPUS_EVENT_LABEL,
  describePhase,
  formatEventTime,
  formatShortDay,
  type AgendaEntry,
} from '@/lib/agenda'
import { openingInstant } from '@/lib/deadlines'
import { useOpeningAt } from '@/hooks/useOpenings'
import { assessmentKind } from '@/lib/share'
import { progressKeyOf } from '@/lib/progress'

interface EntryCardProps {
  entry: AgendaEntry
  showDate?: boolean
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

function EntryCardBase({ entry, showDate = false, onSelect }: EntryCardProps) {
  const kind = entry.kind === 'activity' ? assessmentKind(entry.title) : null
  const knownOpening = useOpeningAt(entry.kind === 'activity' ? entry.activityId : null)
  const openedAt = knownOpening ?? (entry.kind === 'activity' ? openingInstant(entry.phases) : null)

  return (
    <li>
      <article className="rounded-2xl border border-input bg-card transition-colors">
        <button
          type="button"
          onClick={(event) => onSelect(entry, event.currentTarget)}
          className="block w-full rounded-2xl p-4 text-left hover:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background focus-visible:outline-none"
        >
          <span className="flex flex-wrap items-center gap-2">
            {entry.kind === 'event' ? (
              <Badge variant="outline" className="border-input text-foreground">
                {CAMPUS_EVENT_LABEL}
              </Badge>
            ) : (
              <Badge className="bg-accent text-accent-foreground">{entry.course ?? 'Course'}</Badge>
            )}
            {kind && (
              <Badge variant="outline" className="border-input text-foreground">
                {kind}
              </Badge>
            )}
            {showDate && <span className="text-base text-muted-foreground">{formatShortDay(entry.date)}</span>}
          </span>
          <span className="mt-2 block text-lg leading-snug font-semibold break-words text-foreground">
            {entry.title}
          </span>
          <span className="mt-3 flex flex-col gap-2 text-base text-muted-foreground">
            {entry.kind === 'activity' ? (
              entry.phases.map((phase) => (
                <span key={phase.id} className="flex flex-col gap-1">
                  <span className="flex flex-wrap items-center gap-x-2 gap-y-1.5 tabular-nums">
                    <PhaseIcon phase={phase.phase} />
                    <span>{describePhase(phase, entry.date)}</span>
                    <PhaseTimer item={phase} />
                  </span>
                  {phase.phase === 'due' && <TimeLeftMeter item={phase} openedAt={openedAt} />}
                </span>
              ))
            ) : (
              <>
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1.5 tabular-nums">
                  <Clock aria-hidden="true" className="size-4 shrink-0" />
                  <span>{formatEventTime(entry.item)}</span>
                  <PhaseTimer item={entry.item} />
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

        <div className="flex flex-col gap-3 border-t border-border p-4 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
          <ProgressControl storageKey={progressKeyOf(entry)} title={entry.title} />
          <CopyButton entry={entry} />
        </div>
      </article>
    </li>
  )
}

export const EntryCard = memo(EntryCardBase)
