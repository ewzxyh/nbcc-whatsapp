import { useMemo } from 'react'
import { DayGroups } from '@/components/agenda/DayGroups'
import { EmptyAgenda } from '@/components/agenda/AgendaStates'
import { groupByDate, type AgendaEntry } from '@/lib/agenda'

interface ListViewProps {
  entries: AgendaEntry[]
  today: string
  filtersActive: boolean
  onClearFilters: () => void
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

export default function ListView({ entries, today, filtersActive, onClearFilters, onSelect }: ListViewProps) {
  const groups = useMemo(() => groupByDate(entries), [entries])

  if (groups.length === 0) {
    return <EmptyAgenda filtersActive={filtersActive} onClearFilters={onClearFilters} />
  }
  return <DayGroups groups={groups} today={today} onSelect={onSelect} />
}
