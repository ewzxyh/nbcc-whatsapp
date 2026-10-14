import type { ReactNode, RefObject } from 'react'
import { ClipboardCheckIcon, LockKeyholeIcon } from '@solar-icons/react/linear'
import { CopyButton } from '@/components/agenda/CopyButton'
import { PhaseIcon, PhaseTimer, TimeLeftMeter } from '@/components/agenda/DeadlineMeter'
import { ProgressControl } from '@/components/agenda/ProgressControl'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  Sheet,
} from '@/components/ui/sheet'
import {
  CAMPUS_EVENT_LABEL,
  courseLabel,
  displayTitle,
  formatEventTime,
  formatLongDate,
  formatTime,
  isLockDownAssessment,
  phaseLabel,
  phasesForEntry,
  TIMEZONE_NOTE,
  type AgendaEntry,
} from '@/lib/agenda'
import type { AgendaItem } from '@/lib/api'
import { openingInstant } from '@/lib/deadlines'
import { useOpeningAt } from '@/hooks/useOpenings'
import { progressKeyOf } from '@/lib/progress'
import { assessmentKind } from '@/lib/share'
import { cn } from '@/lib/utils'

interface ItemDetailsSheetProps {
  entry: AgendaEntry | null
  open: boolean
  onOpenChange: (open: boolean) => void
  allItems: AgendaItem[]
  desktop: boolean
  triggerRef: RefObject<HTMLElement | null>
}

export function ItemDetailsSheet({ entry, open, onOpenChange, allItems, desktop, triggerRef }: ItemDetailsSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side={desktop ? 'right' : 'bottom'}
        showCloseButton={false}
        onCloseAutoFocus={(event) => {
          event.preventDefault()
          const trigger = triggerRef.current
          if (trigger?.isConnected) trigger.focus()
          else document.getElementById('agenda')?.focus()
        }}
        className={cn(
          'flex max-h-[85dvh] flex-col gap-0 p-0',
          desktop ? 'sm:max-w-lg' : 'rounded-t-2xl',
          entry && isLockDownAssessment(entry) && 'border-destructive bg-destructive-subtle',
        )}
      >
        {entry && <EntryDetails entry={entry} allItems={allItems} />}
      </SheetContent>
    </Sheet>
  )
}

function DetailRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <dt className="text-sm font-medium text-muted-foreground">{label}</dt>
      <dd className="text-base text-foreground break-words">{children}</dd>
    </div>
  )
}

function EntryDetails({ entry, allItems }: { entry: AgendaEntry; allItems: AgendaItem[] }) {
  const assessment = isLockDownAssessment(entry)
  const phases = phasesForEntry(entry, allItems)
  const kind = entry.kind === 'activity' ? assessmentKind(entry.title) : null
  const knownOpening = useOpeningAt(entry.kind === 'activity' ? entry.activityId : null)
  const openedAt = knownOpening ?? (entry.kind === 'activity' ? openingInstant(phases) : null)
  const instructor = entry.kind === 'activity' ? entry.instructor : null
  const location = entry.kind === 'event' ? entry.item.location : null

  return (
    <>
      <SheetHeader className="gap-2 p-5 pb-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge className={assessment ? 'bg-destructive text-primary-foreground' : 'bg-accent text-accent-foreground'}>
            {entry.kind === 'activity' ? (entry.course ?? 'Course') : CAMPUS_EVENT_LABEL}
          </Badge>
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
        </div>
        <SheetTitle className="text-xl leading-snug break-words">{displayTitle(entry)}</SheetTitle>
        <SheetDescription className="text-base break-words">{courseLabel(entry)}</SheetDescription>
      </SheetHeader>

      <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-5 pb-4">
        {assessment ? (
          <Badge className="min-h-11 gap-2 border border-destructive bg-destructive px-4 py-2 text-base font-semibold text-primary-foreground">
            <ClipboardCheckIcon aria-hidden="true" className="size-5 shrink-0" />
            Assessment / Test
          </Badge>
        ) : entry.kind === 'activity' && (
          <section aria-labelledby="sheet-progress-label" className="space-y-2">
            <p id="sheet-progress-label" className="text-base font-medium text-foreground">
              Progress
            </p>
            <ProgressControl storageKey={progressKeyOf(entry)} title={entry.title} />
            <p className="text-sm text-muted-foreground">Saved in this browser for 365 days.</p>
          </section>
        )}

        <dl className="space-y-5">
          {entry.kind === 'activity' &&
            phases.map((phase) => (
              <DetailRow key={phase.id} label={phaseLabel(phase.phase, assessment)}>
                <span className="flex items-center gap-2">
                  <PhaseIcon phase={phase.phase} />
                  <span>{formatLongDate(phase.date)}</span>
                </span>
                <span className="mt-1 block tabular-nums">
                  {phase.allDay || !phase.startAt ? 'All day' : formatTime(phase.startAt)}
                </span>
                <span className="mt-2 flex flex-col items-start gap-1">
                  <PhaseTimer item={phase} starts={assessment} />
                  {phase.phase === 'due' && <TimeLeftMeter item={phase} openedAt={openedAt} />}
                </span>
              </DetailRow>
            ))}
          {entry.kind === 'event' && (
            <DetailRow label="When">
              <span className="block">{formatLongDate(entry.item.date)}</span>
              <span className="block tabular-nums">{formatEventTime(entry.item)}</span>
              <span className="mt-2 block">
                <PhaseTimer item={entry.item} />
              </span>
            </DetailRow>
          )}
          {instructor && <DetailRow label="Professor">{instructor}</DetailRow>}
          {location && <DetailRow label="Location">{location}</DetailRow>}
        </dl>
        <p className="text-sm text-muted-foreground">{TIMEZONE_NOTE}</p>
      </div>

      <SheetFooter className="flex-row flex-wrap justify-between gap-2 border-t border-border p-4">
        <CopyButton entry={entry} />
        <SheetClose asChild>
          <Button variant="outline">Close</Button>
        </SheetClose>
      </SheetFooter>
    </>
  )
}
