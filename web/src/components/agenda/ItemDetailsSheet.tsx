import type { ReactNode, RefObject } from 'react'
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
  courseLabel,
  formatEventTime,
  formatLongDate,
  formatTime,
  phaseLabel,
  phasesForEntry,
  TIMEZONE_NOTE,
  type AgendaEntry,
} from '@/lib/agenda'
import type { AgendaItem } from '@/lib/api'
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
          else document.getElementById("agenda")?.focus()
        }}
        className={cn(
          'flex max-h-[85dvh] flex-col gap-0 p-0',
          desktop ? 'sm:max-w-md' : 'rounded-t-2xl',
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
  const phases = phasesForEntry(entry, allItems)
  const instructor = entry.kind === 'activity' ? entry.instructor : null
  const location = entry.kind === 'event' ? entry.item.location : null

  return (
    <>
      <SheetHeader className="gap-1 p-5 pb-3">
        <SheetTitle className="text-xl leading-snug break-words">{entry.title}</SheetTitle>
        <SheetDescription className="text-base break-words">{courseLabel(entry)}</SheetDescription>
      </SheetHeader>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4">
        <dl className="space-y-5">
          {entry.kind === 'activity' &&
            phases.map((phase) => (
              <DetailRow key={phase.id} label={phaseLabel(phase.phase)}>
                <span className="block">{formatLongDate(phase.date)}</span>
                <span className="block tabular-nums">
                  {phase.allDay || !phase.startAt ? 'All day' : formatTime(phase.startAt)}
                </span>
              </DetailRow>
            ))}
          {entry.kind === 'event' && (
            <DetailRow label="When">
              <span className="block">{formatLongDate(entry.item.date)}</span>
              <span className="block tabular-nums">{formatEventTime(entry.item)}</span>
            </DetailRow>
          )}
          {instructor && <DetailRow label="Instructor">{instructor}</DetailRow>}
          {location && <DetailRow label="Location">{location}</DetailRow>}
        </dl>
        <p className="mt-6 text-sm text-muted-foreground">{TIMEZONE_NOTE}</p>
      </div>

      <SheetFooter className="flex-row justify-end border-t border-border p-4">
        <SheetClose asChild>
          <Button variant="outline">Close</Button>
        </SheetClose>
      </SheetFooter>
    </>
  )
}
