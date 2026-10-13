import { CalendarDays, Kanban, Tags, type LucideIcon } from 'lucide-react'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

export type ViewId = 'subjects' | 'calendar' | 'board'

const VIEWS: { id: ViewId; label: string; icon: LucideIcon }[] = [
  { id: 'subjects', label: 'Subjects', icon: Tags },
  { id: 'calendar', label: 'Calendar', icon: CalendarDays },
  { id: 'board', label: 'Board', icon: Kanban },
]

interface ViewSwitcherProps {
  value: ViewId
  onChange: (view: ViewId) => void
}

export function ViewSwitcher({ value, onChange }: ViewSwitcherProps) {
  return (
    <div className="space-y-2">
      <p id="view-switcher-label" className="text-base font-medium text-foreground">
        View
      </p>
      <ToggleGroup
        type="single"
        value={value}
        onValueChange={(next) => {
          if (next) onChange(next as ViewId)
        }}
        aria-labelledby="view-switcher-label"
        className="grid w-full grid-cols-3 gap-2"
      >
        {VIEWS.map(({ id, label, icon: Icon }) => (
          <ToggleGroupItem
            key={id}
            value={id}
            className="flex h-auto min-h-12 w-full min-w-0 flex-row items-center justify-center gap-2 rounded-xl border border-input bg-background px-2 py-2 text-base font-medium text-foreground transition-colors data-[state=off]:hover:bg-surface data-[state=on]:border-primary data-[state=on]:bg-accent data-[state=on]:text-accent-foreground sm:min-h-16 sm:flex-col sm:gap-1 sm:px-3"
          >
            <Icon aria-hidden="true" className="size-5" />
            <span>{label}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}
