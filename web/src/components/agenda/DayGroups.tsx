import { EntryCard } from './EntryCard'
import { formatDayLabel, formatShortDay, type AgendaEntry, type DayGroup } from '@/lib/agenda'

interface DayGroupsProps {
  groups: DayGroup[]
  today: string
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

export function DayGroups({ groups, today, onSelect }: DayGroupsProps) {
  return (
    <div className="space-y-8">
      {groups.map((group) => {
        const label = formatDayLabel(group.date, today)
        const short = formatShortDay(group.date)
        const headingId = `day-${group.date}`
        return (
          <section key={group.date} aria-labelledby={headingId} className="space-y-3">
            <h2 id={headingId} className="text-xl font-semibold text-foreground">
              {label}
              {label !== short && (
                <>
                  {' '}
                  <span className="text-base font-normal text-muted-foreground">{short}</span>
                </>
              )}
            </h2>
            <ul className="space-y-3">
              {group.entries.map((entry) => (
                <EntryCard key={entry.key} entry={entry} onSelect={onSelect} />
              ))}
            </ul>
          </section>
        )
      })}
    </div>
  )
}
