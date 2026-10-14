import { useMemo } from 'react'
import { EmptyAgenda } from '@/components/agenda/AgendaStates'
import { EntryCard } from '@/components/agenda/EntryCard'
import { ChoiceGroup } from '@/components/app/ChoiceGroup'
import { formatCount, isLockDownAssessment, subjectColumns, timingColumns, type AgendaEntry } from '@/lib/agenda'
import { cn } from '@/lib/utils'

export type BoardGroup = 'timing' | 'subject'

interface BoardViewProps {
  entries: AgendaEntry[]
  today: string
  showPast: boolean
  group: BoardGroup
  onGroupChange: (group: BoardGroup) => void
  filtersActive: boolean
  onClearFilters: () => void
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

export default function BoardView({
  entries,
  today,
  showPast,
  group,
  onGroupChange,
  filtersActive,
  onClearFilters,
  onSelect,
}: BoardViewProps) {
  const columns = useMemo(
    () => (group === 'timing' ? timingColumns(entries, today, showPast) : subjectColumns(entries)),
    [entries, today, showPast, group],
  )

  return (
    <div className="space-y-6">
      <ChoiceGroup<BoardGroup>
        id="board-group"
        label="Group by"
        value={group}
        options={[
          { value: 'timing', label: 'Timing' },
          { value: 'subject', label: 'Subject' },
        ]}
        onChange={onGroupChange}
      />

      {entries.length === 0 ? (
        <EmptyAgenda filtersActive={filtersActive} onClearFilters={onClearFilters} />
      ) : (
        <div className="grid w-full grid-cols-[repeat(auto-fit,minmax(min(100%,24rem),1fr))] gap-4">
          {columns.map((column, index) => {
            const headingId = `board-column-${index}`
            return (
              <section
                key={column.id}
                aria-labelledby={headingId}
                className={cn(
                  'flex min-w-0 flex-col gap-3 rounded-2xl border p-3',
                  column.entries.some(isLockDownAssessment) ? 'border-destructive bg-destructive-subtle' : column.id === 'today' ? 'border-today-border bg-today' : 'border-border bg-surface',
                )}
              >
                <div className="flex items-start justify-between gap-2 px-1">
                  <div className="min-w-0">
                    <h2 id={headingId} className="text-lg font-semibold break-words text-foreground">
                      {column.title}
                    </h2>
                    {column.subtitle && (
                      <p className="text-sm break-words text-muted-foreground">{column.subtitle}</p>
                    )}
                  </div>
                  <span className="shrink-0 rounded-full border border-input bg-background px-2.5 py-0.5 text-sm text-foreground tabular-nums">
                    {formatCount(column.entries.length, 'item')}
                  </span>
                </div>
                {column.entries.length > 0 ? (
                  <ul className="space-y-3">
                    {column.entries.map((entry) => (
                      <EntryCard key={entry.key} entry={entry} showDate onSelect={onSelect} />
                    ))}
                  </ul>
                ) : (
                  <p className="px-1 text-base text-muted-foreground">Nothing here.</p>
                )}
              </section>
            )
          })}
        </div>
      )}
    </div>
  )
}
