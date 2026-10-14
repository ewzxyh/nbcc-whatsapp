import { CalendarIcon, KanbanIcon, TagIcon } from '@solar-icons/react/linear'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

export type ViewId = 'subjects' | 'calendar' | 'board'

const VIEWS: { id: ViewId; label: string; icon: typeof TagIcon }[] = [
  { id: 'subjects', label: 'Subjects', icon: TagIcon },
  { id: 'calendar', label: 'Calendar', icon: CalendarIcon },
  { id: 'board', label: 'Board', icon: KanbanIcon },
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
            className="flex h-auto min-h-16 w-full min-w-0 flex-col items-center justify-center gap-1 rounded-xl border border-input bg-background px-1 py-2 text-base font-medium text-foreground transition-colors data-[state=off]:hover:bg-surface data-[state=on]:border-2 data-[state=on]:border-primary data-[state=on]:bg-accent data-[state=on]:font-semibold data-[state=on]:text-accent-foreground"
          >
            <Icon aria-hidden="true" className="size-6" />
            <span className="max-w-full truncate">{label}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}
