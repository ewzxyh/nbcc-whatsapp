import { memo } from 'react'
import { ClipboardCheckIcon, ClockCircleIcon, LockKeyholeIcon, MapPointIcon } from '@solar-icons/react/linear'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/agenda/CopyButton'
import { PhaseIcon, PhaseTimer, TimeLeftMeter } from '@/components/agenda/DeadlineMeter'
import { ProgressControl } from '@/components/agenda/ProgressControl'
import {
  CAMPUS_EVENT_LABEL,
  describePhase,
  displayTitle,
  formatEventTime,
  formatShortDay,
  isLockDownAssessment,
  type AgendaEntry,
} from '@/lib/agenda'
import { illustrationFor, openingInstant } from '@/lib/deadlines'
import { useNow } from '@/hooks/useNow'
import { useOpeningAt } from '@/hooks/useOpenings'
import { assessmentKind } from '@/lib/share'
import { progressKeyOf } from '@/lib/progress'
import { cn } from '@/lib/utils'

interface EntryCardProps {
  entry: AgendaEntry
  showDate?: boolean
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

function EntryCardBase({ entry, showDate = false, onSelect }: EntryCardProps) {
  const assessment = isLockDownAssessment(entry)
  const illustration = illustrationFor(entry, useNow())
  const kind = entry.kind === 'activity' ? assessmentKind(entry.title) : null
  const knownOpening = useOpeningAt(entry.kind === 'activity' ? entry.activityId : null)
  const openedAt = knownOpening ?? (entry.kind === 'activity' ? openingInstant(entry.phases) : null)

  return (
    <li>
      <article className={cn('rounded-2xl border border-input bg-card transition-colors', assessment && 'border-destructive bg-destructive-subtle')}>
        <button
          type="button"
          onClick={(event) => onSelect(entry, event.currentTarget)}
          className={cn('block w-full rounded-2xl p-4 text-left hover:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background focus-visible:outline-none', assessment && 'hover:bg-destructive/5 focus-visible:ring-destructive')}
        >
          <span className={cn('block', illustration && 'flex flex-wrap items-center gap-4')}>
            <span className={cn('block', illustration && 'min-w-0 flex-[1_1_20rem]')}>
              <span className="flex flex-wrap items-center gap-2">
                {entry.kind === 'event' ? (
                  <Badge variant="outline" className="border-input text-foreground">
                    {CAMPUS_EVENT_LABEL}
                  </Badge>
                ) : (
                  <Badge className={assessment ? 'bg-destructive text-primary-foreground' : 'bg-accent text-accent-foreground'}>{entry.course ?? 'Course'}</Badge>
                )}
                {kind && (
                  <Badge variant="outline" className={assessment ? 'border-destructive text-destructive' : 'border-input text-foreground'}>
                    {kind}
                  </Badge>
                )}
                {assessment && (
                  <Badge variant="outline" className="border-destructive bg-destructive-subtle whitespace-normal text-destructive">
                    <LockKeyholeIcon aria-hidden="true" className="size-4 shrink-0" />
                    LockDown Browser required
                  </Badge>
                )}
                {showDate && <span className="text-base text-muted-foreground">{formatShortDay(entry.date)}</span>}
              </span>
              <span className="mt-2 block text-lg leading-snug font-semibold break-words text-foreground">
                {displayTitle(entry)}
              </span>
              <span className="mt-3 flex flex-col gap-2 text-base text-muted-foreground">
                {entry.kind === 'activity' ? (
                  entry.phases.map((phase) => (
                    <span key={phase.id} className="flex flex-col gap-1">
                      <span className="flex flex-wrap items-center gap-x-2 gap-y-1.5 tabular-nums">
                        <PhaseIcon phase={phase.phase} />
                        <span>{describePhase(phase, entry.date, assessment)}</span>
                        <PhaseTimer item={phase} starts={assessment} />
                      </span>
                      {phase.phase === 'due' && <TimeLeftMeter item={phase} openedAt={openedAt} />}
                    </span>
                  ))
                ) : (
                  <>
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-1.5 tabular-nums">
                      <ClockCircleIcon aria-hidden="true" className="size-4.5 shrink-0" />
                      <span>{formatEventTime(entry.item)}</span>
                      <PhaseTimer item={entry.item} />
                    </span>
                    {entry.item.location && (
                      <span className="flex items-start gap-2">
                        <MapPointIcon aria-hidden="true" className="mt-1 size-4.5 shrink-0" />
                        <span className="break-words">{entry.item.location}</span>
                      </span>
                    )}
                  </>
                )}
              </span>
            </span>
            {illustration && (
              <img src={illustration} alt="" aria-hidden="true" width={illustration === '/assessment-chimp.png' ? 1200 : 1254} height={illustration === '/assessment-chimp.png' ? 917 : 1254} loading="lazy" className={cn('ml-auto h-auto w-28 shrink-0 rounded-xl object-contain sm:w-40 lg:w-52', illustration === '/assessment-chimp.png' && 'mix-blend-multiply')} />
            )}
          </span>
        </button>

        <div className={cn('flex flex-col gap-3 border-t border-border p-4 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between', assessment && 'border-destructive/30')}>
          {assessment ? (
            <Badge className="min-h-11 gap-2 border border-destructive bg-destructive px-4 py-2 text-base font-semibold text-primary-foreground">
              <ClipboardCheckIcon aria-hidden="true" className="size-5 shrink-0" />
              Assessment / Test
            </Badge>
          ) : entry.kind === 'activity' && <ProgressControl storageKey={progressKeyOf(entry)} title={entry.title} />}
          <CopyButton entry={entry} />
        </div>
      </article>
    </li>
  )
}

export const EntryCard = memo(EntryCardBase)
