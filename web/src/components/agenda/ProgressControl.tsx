import { useSyncExternalStore } from 'react'
import { CheckIcon } from '@solar-icons/react/linear'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  EMPTY_PROGRESS,
  getProgressSnapshot,
  setProgress,
  statusFor,
  subscribeProgress,
  type ProgressStatus,
} from '@/lib/progress'

// Selected state is shown by a check mark and bolder text as well as colour (WCAG 1.4.1).
// Phones stack the options at full width, because three columns at 320 px cannot hold 16 px labels and a check mark.
const ITEM_CLASS =
  'group/item h-auto min-h-11 min-w-0 flex-1 justify-start gap-2 rounded-xl border border-input bg-background px-3 py-2 text-left text-base leading-snug font-medium whitespace-normal text-foreground transition-colors hover:bg-surface data-[state=on]:border-2 data-[state=on]:border-primary data-[state=on]:bg-accent data-[state=on]:font-semibold data-[state=on]:text-accent-foreground sm:flex-none sm:justify-center sm:text-center'

const OPTIONS: { value: ProgressStatus; label: string }[] = [
  { value: 'not_started', label: 'Not started' },
  { value: 'in_progress', label: 'In progress' },
  { value: 'done', label: 'Done' },
]

/** Three-way progress selector saved in this browser for 365 days. */
export function ProgressControl({ storageKey, title }: { storageKey: string; title: string }) {
  const map = useSyncExternalStore(subscribeProgress, getProgressSnapshot, () => EMPTY_PROGRESS)
  const status = statusFor(map, storageKey)
  return (
    <ToggleGroup
      type="single"
      value={status}
      onValueChange={(next) => setProgress(storageKey, (next || 'not_started') as ProgressStatus)}
      aria-label={`Progress for ${title}`}
      className="grid w-full grid-cols-1 gap-2 sm:flex sm:w-auto"
    >
      {OPTIONS.map((option) => (
        <ToggleGroupItem key={option.value} value={option.value} className={ITEM_CLASS}>
          <CheckIcon aria-hidden="true" className="hidden size-4.5 shrink-0 group-data-[state=on]/item:block" />
          <span>{option.label}</span>
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
