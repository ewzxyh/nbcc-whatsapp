import type { ReactNode } from 'react'
import { CheckIcon } from '@solar-icons/react/linear'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

const CHIP_CLASS =
  'group/chip h-11 min-w-11 shrink-0 rounded-full border border-input bg-background px-4 text-base font-medium text-foreground transition-colors data-[state=off]:hover:bg-surface data-[state=on]:border-primary data-[state=on]:bg-accent data-[state=on]:text-accent-foreground'

export function ToggleChip({ value, children }: { value: string; children: ReactNode }) {
  return (
    <ToggleGroupItem value={value} className={CHIP_CLASS}>
      <CheckIcon aria-hidden="true" className="hidden size-4.5 group-data-[state=on]/chip:block" />
      <span>{children}</span>
    </ToggleGroupItem>
  )
}

interface ChoiceGroupProps<T extends string> {
  id: string
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
}

export function ChoiceGroup<T extends string>({ id, label, value, options, onChange }: ChoiceGroupProps<T>) {
  const labelId = `${id}-label`
  return (
    <div className="space-y-2">
      <p id={labelId} className="text-base font-medium text-foreground">
        {label}
      </p>
      <ToggleGroup
        type="single"
        value={value}
        onValueChange={(next) => {
          if (next) onChange(next as T)
        }}
        aria-labelledby={labelId}
        className="flex flex-wrap gap-2"
      >
        {options.map((option) => (
          <ToggleChip key={option.value} value={option.value}>
            {option.label}
          </ToggleChip>
        ))}
      </ToggleGroup>
    </div>
  )
}
