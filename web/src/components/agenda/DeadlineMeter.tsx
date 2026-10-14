import { ClockCircleIcon, HourglassIcon, LockKeyholeIcon, LockKeyholeUnlockedIcon } from '@solar-icons/react/linear'
import { Badge } from '@/components/ui/badge'
import { countdownFor, itemInstant, litSegments, PROGRESS_SEGMENTS, timeLeftShare } from '@/lib/deadlines'
import type { AgendaItem } from '@/lib/api'
import { useNow } from '@/hooks/useNow'
import { cn } from '@/lib/utils'

/** Countdown badge for one phase or event, e.g. "Due in 2 d 4 h". Text is updated once a minute. */
export function PhaseTimer({ item }: { item: AgendaItem }) {
  const now = useNow()
  const { text, passed } = countdownFor(item, now)
  return (
    <Badge
      variant="outline"
      className={cn('gap-1.5 border-input bg-background tabular-nums', passed ? 'text-muted-foreground' : 'text-foreground')}
    >
      <HourglassIcon aria-hidden="true" className="size-4" />
      <span>{text}</span>
    </Badge>
  )
}

const SEGMENT_COLORS = [
  'bg-progress-1 border-progress-1',
  'bg-progress-2 border-progress-2',
  'bg-progress-3 border-progress-3',
  'bg-progress-4 border-progress-4',
  'bg-progress-5 border-progress-5',
]

/**
 * Five small squares from red (left) to green (right). Squares are filled from the left in proportion to the time
 * left in the window: many filled means plenty of time, one red square means the due time is close.
 * The percentage is announced as text, so colour is not the only signal.
 */
export function TimeLeftMeter({ item, openedAt }: { item: AgendaItem; openedAt: number | null }) {
  const now = useNow()
  const dueAt = itemInstant(item)
  const share = timeLeftShare(dueAt, now, openedAt)
  const lit = litSegments(share)
  const percent = Math.round(share * 100)
  const passed = dueAt <= now

  return (
    <span className="mt-1.5 flex flex-wrap items-center gap-2">
      <span aria-hidden="true" className="flex gap-1">
        {Array.from({ length: PROGRESS_SEGMENTS }, (_, index) => {
          const filled = index < lit
          return (
            <span
              key={index}
              className={cn(
                'size-3 rounded-[3px] border',
                filled ? SEGMENT_COLORS[index] : 'border-progress-track bg-transparent',
              )}
            />
          )
        })}
      </span>
      <span className="text-sm text-muted-foreground">
        {passed ? 'No time left' : `${percent}% of time left`}
      </span>
    </span>
  )
}

export function PhaseIcon({ phase }: { phase: AgendaItem['phase'] }) {
  if (phase === 'available') return <LockKeyholeUnlockedIcon aria-hidden="true" className="size-4.5 shrink-0" />
  if (phase === 'closes') return <LockKeyholeIcon aria-hidden="true" className="size-4.5 shrink-0" />
  return <ClockCircleIcon aria-hidden="true" className="size-4.5 shrink-0" />
}
