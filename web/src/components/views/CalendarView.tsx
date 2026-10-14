import { useEffect, useMemo, useRef } from 'react'
import { AltArrowLeftIcon, AltArrowRightIcon } from '@solar-icons/react/linear'
import { EntryCard } from '@/components/agenda/EntryCard'
import { EmptyAgenda } from '@/components/agenda/AgendaStates'
import { Button } from '@/components/ui/button'
import {
  countByDate,
  formatCount,
  formatLongDate,
  formatMonthTitle,
  groupEntries,
  isLockDownAssessment,
  monthGrid,
  type AgendaEntry,
} from '@/lib/agenda'
import type { AgendaItem } from '@/lib/api'
import { cn } from '@/lib/utils'

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

interface CalendarViewProps {
  month: string
  today: string
  items: AgendaItem[]
  selectedDate: string | null
  onSelectDate: (date: string) => void
  onPreviousMonth: () => void
  onNextMonth: () => void
  onToday: () => void
  filtersActive: boolean
  onClearFilters: () => void
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
  /** Moves focus to the month heading after the user changes month, because the previous control may be unmounted while loading. */
  focusHeading: boolean
  onHeadingFocused: () => void
}

interface DayButtonProps {
  date: string
  count: number
  isToday: boolean
  isSelected: boolean
  onSelect: (date: string) => void
}

function DayButton({ date, count, isToday, isSelected, onSelect }: DayButtonProps) {
  return (
    <button
      type="button"
      aria-label={`${formatLongDate(date)}. ${formatCount(count, 'item')}.`}
      aria-current={isToday ? 'date' : undefined}
      aria-pressed={isSelected}
      onClick={() => onSelect(date)}
      className={cn(
        'flex min-h-14 w-full flex-col items-center justify-center gap-1 rounded-xl border-2 px-0.5 py-1 text-base tabular-nums text-foreground transition-colors outline-none hover:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
        isToday ? 'border-primary font-bold' : 'border-transparent font-medium',
        isSelected && 'border-primary bg-primary text-primary-foreground hover:bg-primary',
      )}
    >
      <span>{Number(date.slice(8))}</span>
      <span
        className={cn(
          'min-w-6 rounded-full px-1.5 text-sm leading-5',
          count > 0 ? 'border border-input bg-background text-foreground' : 'text-muted-foreground',
          isSelected && count === 0 && 'text-primary-foreground',
        )}
      >
        {count}
      </span>
    </button>
  )
}

export default function CalendarView({
  month,
  today,
  items,
  selectedDate,
  onSelectDate,
  onPreviousMonth,
  onNextMonth,
  onToday,
  filtersActive,
  onClearFilters,
  onSelect,
  focusHeading,
  onHeadingFocused,
}: CalendarViewProps) {
  const headingRef = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (!focusHeading) return
    headingRef.current?.focus()
    onHeadingFocused()
  }, [focusHeading, onHeadingFocused])
  const counts = useMemo(() => countByDate(items), [items])
  const weeks = useMemo(() => monthGrid(month), [month])
  const dayEntries = useMemo(
    () => (selectedDate ? groupEntries(items.filter((item) => item.date === selectedDate), false) : []),
    [items, selectedDate],
  )
  const monthTitle = formatMonthTitle(month)

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 ref={headingRef} tabIndex={-1} className="rounded-md text-2xl font-semibold text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background">{monthTitle}</h2>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" aria-label="Previous month" onClick={onPreviousMonth} className="px-3">
            <AltArrowLeftIcon aria-hidden="true" />
            Previous
          </Button>
          <Button variant="outline" aria-label="Go to today" onClick={onToday} className="px-3">
            Today
          </Button>
          <Button variant="outline" aria-label="Next month" onClick={onNextMonth} className="px-3">
            Next
            <AltArrowRightIcon aria-hidden="true" />
          </Button>
        </div>
      </div>

      <table className="w-full table-fixed border-separate border-spacing-1">
        <caption className="sr-only">{`Days in ${monthTitle}. Choose a day to see its items.`}</caption>
        <thead>
          <tr>
            {WEEKDAYS.map((day) => (
              <th key={day} scope="col" className="pb-1 text-center text-sm font-medium text-muted-foreground">
                {day}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {weeks.map((week, weekIndex) => (
            <tr key={weekIndex}>
              {week.map((date, dayIndex) => (
                <td key={dayIndex} className="p-0 align-top">
                  {date && (
                    <DayButton
                      date={date}
                      count={counts.get(date) ?? 0}
                      isToday={date === today}
                      isSelected={date === selectedDate}
                      onSelect={onSelectDate}
                    />
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-sm text-muted-foreground">The number under each date is how many items fall on that day.</p>

      <section aria-labelledby="calendar-day-heading" className={cn('space-y-3', dayEntries.some(isLockDownAssessment) && 'rounded-2xl border border-destructive bg-destructive-subtle p-3 sm:p-4')}>
        {selectedDate ? (
          <>
            <h2 id="calendar-day-heading" className="text-xl font-semibold text-foreground">
              {formatLongDate(selectedDate)}
            </h2>
            {dayEntries.length > 0 ? (
              <ul className="space-y-3">
                {dayEntries.map((entry) => (
                  <EntryCard key={entry.key} entry={entry} onSelect={onSelect} />
                ))}
              </ul>
            ) : (
              <EmptyAgenda
                filtersActive={filtersActive}
                onClearFilters={onClearFilters}
                message={filtersActive ? 'Nothing on this day for this filter.' : 'Nothing scheduled on this day.'}
              />
            )}
          </>
        ) : (
          <>
            <h2 id="calendar-day-heading" className="text-xl font-semibold text-foreground">
              Select a day
            </h2>
            <p className="text-base text-muted-foreground">Choose a date above to see its items.</p>
          </>
        )}
      </section>
    </div>
  )
}
