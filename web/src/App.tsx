import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { DangerCircleIcon, InfoCircleIcon, RefreshIcon } from '@solar-icons/react/linear'
import { ItemDetailsSheet } from '@/components/agenda/ItemDetailsSheet'
import { AgendaSkeleton } from '@/components/agenda/AgendaStates'
import { AppHeader } from '@/components/app/AppHeader'
import { QuickLinks } from '@/components/app/QuickLinks'
import { FilterPanel } from '@/components/app/FilterPanel'
import { SearchField } from '@/components/app/SearchField'
import { SiteFooter } from '@/components/app/SiteFooter'
import { ViewSwitcher, type ViewId } from '@/components/app/ViewSwitcher'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useAgenda } from '@/hooks/useAgenda'
import { useMediaQuery } from '@/hooks/useMediaQuery'
import { NowContext, useMinuteClock } from '@/hooks/useNow'
import { OpeningsContext } from '@/hooks/useOpenings'
import { openingsByActivity } from '@/lib/deadlines'
import {
  monctonDate,
  applyFilters,
  deriveProfessors,
  deriveSubjects,
  formatCount,
  groupEntries,
  monthOf,
  monthRange,
  omittedEventsNotice,
  shiftMonth,
  type AgendaEntry,
} from '@/lib/agenda'
import type { AgendaItem } from '@/lib/api'
import type { BoardGroup } from '@/components/views/BoardView'
import type { SubjectsGroup } from '@/components/views/SubjectsView'

const CalendarView = lazy(() => import('@/components/views/CalendarView'))
const SubjectsView = lazy(() => import('@/components/views/SubjectsView'))
const BoardView = lazy(() => import('@/components/views/BoardView'))

const NO_ITEMS: AgendaItem[] = []
const NO_SELECTION: string[] = []

export default function App() {
  const [view, setView] = useState<ViewId>('subjects')
  const [query, setQuery] = useState('')
  const [showCampus, setShowCampus] = useState(true)
  const [showPast, setShowPast] = useState(false)
  const [subjectsGroup, setSubjectsGroup] = useState<SubjectsGroup>('subject')
  const [subjectsTab, setSubjectsTab] = useState('all')
  const [boardGroup, setBoardGroup] = useState<BoardGroup>('timing')
  const [revision, setRevision] = useState(0)
  const [monthOverride, setMonthOverride] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [focusCalendarHeading, setFocusCalendarHeading] = useState(false)
  const [detail, setDetail] = useState<AgendaEntry | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const triggerRef = useRef<HTMLElement | null>(null)
  const desktop = useMediaQuery('(min-width: 768px)')
  const now = useMinuteClock()

  const defaultQuery = useAgenda(undefined, revision)
  const today = defaultQuery.data?.today ?? ''
  const month = monthOverride ?? (today ? monthOf(today) : null)
  const calendarQuery = useAgenda(month ? monthRange(month) : undefined, revision, view === 'calendar' && month !== null)

  const allItems = defaultQuery.data?.items ?? NO_ITEMS
  const openings = useMemo(() => openingsByActivity(allItems), [allItems])
  const calendarItemsAll = calendarQuery.data?.items ?? NO_ITEMS

  const filters = useMemo(
    () => ({ query, subjects: NO_SELECTION, instructors: NO_SELECTION, showCampus, showPast }),
    [query, showCampus, showPast],
  )
  const filteredItems = useMemo(() => applyFilters(allItems, filters, today), [allItems, filters, today])
  const entries = useMemo(() => groupEntries(filteredItems), [filteredItems])
  const calendarItems = useMemo(
    () => applyFilters(calendarItemsAll, filters, today),
    [calendarItemsAll, filters, today],
  )

  // Subject and professor tabs live in the Subjects view, so their facets come from the entries that the other filters keep.
  const subjectFacets = useMemo(() => deriveSubjects(entries, NO_SELECTION), [entries])
  const professorFacets = useMemo(() => deriveProfessors(entries, NO_SELECTION), [entries])
  const catalogHasProfessors = useMemo(() => allItems.some((item) => item.instructor !== null), [allItems])

  const filtersActive = query.trim() !== '' || !showCampus || showPast
  const clearFilters = useCallback(() => {
    setQuery('')
    setShowCampus(true)
    setShowPast(false)
    // The clear button can unmount when filters reset, so focus moves to the search field instead.
    document.getElementById('agenda-search')?.focus()
  }, [])

  const openDetail = useCallback((entry: AgendaEntry, trigger: HTMLElement) => {
    triggerRef.current = trigger
    setDetail(entry)
    setDetailOpen(true)
  }, [])

  const refresh = useCallback(() => setRevision((value) => value + 1), [])

  // The agenda is cached per session, so after midnight the "Today" group would stay on the old date until a refresh.
  useEffect(() => {
    if (today && monctonDate(new Date(now).toISOString()) !== today) refresh()
  }, [now, today, refresh])

  const onHeadingFocused = useCallback(() => setFocusCalendarHeading(false), [])

  const goToMonth = (next: string) => {
    setMonthOverride(next)
    setSelectedDate(today && monthOf(today) === next ? today : null)
    setFocusCalendarHeading(true)
  }
  const effectiveSelectedDate =
    selectedDate ?? (today && month && monthOf(today) === month ? today : null)

  const isCalendar = view === 'calendar'
  const loading = isCalendar
    ? month === null || (calendarQuery.status === 'loading' && !calendarQuery.data)
    : defaultQuery.status === 'loading' && !defaultQuery.data
  const activeError = isCalendar && month !== null ? calendarQuery.error : defaultQuery.error
  const refreshing = defaultQuery.status === 'loading' || calendarQuery.status === 'loading'
  const hadError = useRef(false)
  useEffect(() => {
    if (activeError) {
      hadError.current = true
    } else if (hadError.current) {
      hadError.current = false
      if (!document.activeElement || document.activeElement === document.body) {
        document.getElementById('agenda')?.focus()
      }
    }
  }, [activeError])
  const shownCount = isCalendar ? calendarItems.length : filteredItems.length
  const hasData = isCalendar ? month !== null && calendarQuery.data !== undefined : defaultQuery.data !== undefined
  const activeData = isCalendar ? calendarQuery.data : defaultQuery.data
  const omittedNotice = hasData ? omittedEventsNotice(activeData?.omittedEvents) : null

  return (
    <NowContext.Provider value={now}>
     <OpeningsContext.Provider value={openings}>
      <a
        href="#agenda"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-xl focus:bg-primary focus:px-4 focus:py-3 focus:text-base focus:font-medium focus:text-primary-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
      >
        Skip to agenda
      </a>

      {/* Every view uses the same width, so the Board columns fill the same space as the list views. */}
      <div className="mx-auto flex min-h-dvh w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 md:py-8">
        <div className="mx-auto w-full max-w-5xl space-y-6">
          <AppHeader
            updatedAt={defaultQuery.data?.updatedAt}
            now={now}
            refreshing={refreshing}
            onRefresh={refresh}
          />

          <section aria-label="Search and filters" className="space-y-6 rounded-2xl border border-border p-4 sm:p-5">
            <SearchField value={query} onChange={setQuery} />
            <FilterPanel
              showCampus={showCampus}
              onShowCampusChange={setShowCampus}
              showPast={showPast}
              onShowPastChange={setShowPast}
              filtersActive={filtersActive}
              onClearFilters={clearFilters}
            />
          </section>
        </div>

        <main
          id="agenda"
          tabIndex={-1}
          aria-busy={loading}
          className="mx-auto w-full max-w-5xl space-y-6 outline-none"
        >
          <ViewSwitcher value={view} onChange={setView} />

          <p role="status" aria-live="polite" className="text-base text-muted-foreground">
            {loading || !hasData ? '' : `${formatCount(shownCount, 'item')} shown`}
          </p>

          {activeError && (
            <div className="space-y-3">
              <Alert variant="destructive" className="border-destructive bg-destructive-subtle">
                <DangerCircleIcon aria-hidden="true" />
                <AlertTitle>Agenda unavailable</AlertTitle>
                <AlertDescription>{activeError.message}</AlertDescription>
              </Alert>
              <Button variant="outline" onClick={refresh}>
                <RefreshIcon aria-hidden="true" />
                Try again
              </Button>
            </div>
          )}

          {omittedNotice && (
            <Alert role="status" className="border-input">
              <InfoCircleIcon aria-hidden="true" />
              <AlertDescription className="text-foreground">{omittedNotice}</AlertDescription>
            </Alert>
          )}

          {loading ? (
            <AgendaSkeleton />
          ) : hasData ? (
            <Suspense fallback={<AgendaSkeleton />}>
              {view === 'calendar' && month && (
                <CalendarView
                  month={month}
                  today={today}
                  items={calendarItems}
                  selectedDate={effectiveSelectedDate}
                  onSelectDate={setSelectedDate}
                  onPreviousMonth={() => goToMonth(shiftMonth(month, -1))}
                  onNextMonth={() => goToMonth(shiftMonth(month, 1))}
                  onToday={() => {
                    setMonthOverride(monthOf(today))
                    setSelectedDate(today)
                    setFocusCalendarHeading(true)
                  }}
                  focusHeading={focusCalendarHeading}
                  onHeadingFocused={onHeadingFocused}
                  filtersActive={filtersActive}
                  onClearFilters={clearFilters}
                  onSelect={openDetail}
                />
              )}
              {view === 'subjects' && (
                <SubjectsView
                  entries={entries}
                  today={today}
                  subjects={subjectFacets}
                  professors={professorFacets}
                  catalogHasProfessors={catalogHasProfessors}
                  group={subjectsGroup}
                  onGroupChange={setSubjectsGroup}
                  tab={subjectsTab}
                  onTabChange={setSubjectsTab}
                  filtersActive={filtersActive}
                  onClearFilters={clearFilters}
                  onSelect={openDetail}
                />
              )}
              {view === 'board' && (
                <BoardView
                  entries={entries}
                  today={today}
                  showPast={showPast}
                  group={boardGroup}
                  onGroupChange={setBoardGroup}
                  filtersActive={filtersActive}
                  onClearFilters={clearFilters}
                  onSelect={openDetail}
                />
              )}
            </Suspense>
          ) : null}
        </main>

        <div className="mx-auto w-full max-w-5xl space-y-12">
          <QuickLinks />
          <SiteFooter />
        </div>
      </div>

      <ItemDetailsSheet
        entry={detail}
        open={detailOpen}
        onOpenChange={setDetailOpen}
        allItems={allItems}
        triggerRef={triggerRef}
        desktop={desktop}
      />
     </OpeningsContext.Provider>
    </NowContext.Provider>
  )
}
