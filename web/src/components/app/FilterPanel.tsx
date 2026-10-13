import type { ReactNode } from 'react'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup } from '@/components/ui/toggle-group'
import type { ProfessorFacet, SubjectFacet } from '@/lib/agenda'
import { ToggleChip } from './ChoiceGroup'

interface FilterPanelProps {
  subjects: SubjectFacet[]
  catalogHasSubjects: boolean
  selectedSubjects: string[]
  onSubjectsChange: (codes: string[]) => void
  professors: ProfessorFacet[]
  catalogHasProfessors: boolean
  selectedProfessors: string[]
  onProfessorsChange: (names: string[]) => void
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

function FilterGroup({
  labelId,
  label,
  emptyText,
  selected,
  onChange,
  children,
}: {
  labelId: string
  label: string
  emptyText: string | null
  selected: string[]
  onChange: (values: string[]) => void
  children: ReactNode
}) {
  return (
    <div className="space-y-2">
      <p id={labelId} className="text-base font-medium text-foreground">
        {label}
      </p>
      {emptyText ? (
        <p className="text-base text-muted-foreground">{emptyText}</p>
      ) : (
        <ToggleGroup
          type="multiple"
          value={selected}
          onValueChange={onChange}
          aria-labelledby={labelId}
          className="flex flex-wrap gap-2"
        >
          {children}
        </ToggleGroup>
      )}
    </div>
  )
}

export function FilterPanel({
  subjects,
  catalogHasSubjects,
  selectedSubjects,
  onSubjectsChange,
  professors,
  catalogHasProfessors,
  selectedProfessors,
  onProfessorsChange,
  showCampus,
  onShowCampusChange,
  showPast,
  onShowPastChange,
  filtersActive,
  onClearFilters,
}: FilterPanelProps) {
  const subjectsEmpty = !catalogHasSubjects
    ? 'No subjects listed yet'
    : subjects.length === 0
      ? 'No subjects match this search.'
      : null
  const professorsEmpty = !catalogHasProfessors
    ? 'No professors listed yet'
    : professors.length === 0
      ? 'No professors match this search.'
      : null

  return (
    <div className="space-y-6">
      <FilterGroup
        labelId="subject-filter-label"
        label="Subjects"
        emptyText={subjectsEmpty}
        selected={selectedSubjects}
        onChange={onSubjectsChange}
      >
        {subjects.map((subject) => (
          <ToggleChip key={subject.code} value={subject.code}>
            {`${subject.code} (${subject.count})`}
          </ToggleChip>
        ))}
      </FilterGroup>

      <FilterGroup
        labelId="professor-filter-label"
        label="Professors"
        emptyText={professorsEmpty}
        selected={selectedProfessors}
        onChange={onProfessorsChange}
      >
        {professors.map((professor) => (
          <ToggleChip key={professor.name} value={professor.name}>
            {`${professor.name} (${professor.count})`}
          </ToggleChip>
        ))}
      </FilterGroup>

      <div className="space-y-1">
        <SwitchRow id="show-campus" label="Show campus events" checked={showCampus} onCheckedChange={onShowCampusChange} />
        <SwitchRow id="show-past" label="Show past" checked={showPast} onCheckedChange={onShowPastChange} />
      </div>

      {filtersActive && (
        <Button variant="outline" onClick={onClearFilters}>
          <X aria-hidden="true" />
          Clear filters
        </Button>
      )}
    </div>
  )
}
