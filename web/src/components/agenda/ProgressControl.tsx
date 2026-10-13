import { useSyncExternalStore } from 'react'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import {
  EMPTY_PROGRESS,
  getProgressSnapshot,
  setProgress,
  statusFor,
  subscribeProgress,
  type ProgressStatus,
} from '@/lib/progress'

const ITEM_CLASS =
  'h-11 min-w-0 flex-1 rounded-xl border border-input bg-background px-3 text-base font-medium text-foreground transition-colors hover:bg-surface data-[state=on]:border-primary data-[state=on]:bg-accent data-[state=on]:text-accent-foreground sm:flex-none'

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
      className="flex w-full flex-wrap gap-2 sm:w-auto"
    >
      {OPTIONS.map((option) => (
        <ToggleGroupItem key={option.value} value={option.value} className={ITEM_CLASS}>
          {option.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
