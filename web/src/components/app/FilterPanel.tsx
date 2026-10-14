import { CloseIcon } from '@solar-icons/react/linear'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

interface FilterPanelProps {
  showCampus: boolean
  onShowCampusChange: (value: boolean) => void
  showPast: boolean
  onShowPastChange: (value: boolean) => void
  filtersActive: boolean
  onClearFilters: () => void
}

function SwitchRow({
  id,
  label,
  checked,
  onCheckedChange,
}: {
  id: string
  label: string
  checked: boolean
  onCheckedChange: (value: boolean) => void
}) {
  return (
    <div className="flex min-h-11 items-center justify-between gap-4">
      <Label htmlFor={id} className="min-h-11 flex-1 cursor-pointer text-base font-normal text-foreground">
        {label}
      </Label>
      <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  )
}

export function FilterPanel({
  showCampus,
  onShowCampusChange,
  showPast,
  onShowPastChange,
  filtersActive,
  onClearFilters,
}: FilterPanelProps) {
  return (
    <div className="space-y-4">
      <div className="space-y-1">
        <SwitchRow id="show-campus" label="Show campus events" checked={showCampus} onCheckedChange={onShowCampusChange} />
        <SwitchRow id="show-past" label="Show past" checked={showPast} onCheckedChange={onShowPastChange} />
      </div>

      {filtersActive && (
        <Button variant="outline" onClick={onClearFilters}>
          <CloseIcon aria-hidden="true" />
          Clear filters
        </Button>
      )}
    </div>
  )
}
