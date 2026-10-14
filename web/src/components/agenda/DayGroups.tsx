import { EntryCard } from './EntryCard'
import { formatDayLabel, formatShortDay, isLockDownAssessment, type AgendaEntry, type DayGroup } from '@/lib/agenda'
import { cn } from '@/lib/utils'

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
        const isToday = group.date === today
        const assessmentDay = group.entries.some(isLockDownAssessment)
        return (
          <section
            key={group.date}
            aria-labelledby={headingId}
            className={cn('space-y-3', assessmentDay ? 'rounded-2xl border border-destructive bg-destructive-subtle p-3 sm:p-4' : isToday && 'rounded-2xl border border-today-border bg-today p-3 sm:p-4')}
          >
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
