import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { CircleAlert, Info, RefreshCw } from 'lucide-react'
import { ItemDetailsSheet } from '@/components/agenda/ItemDetailsSheet'
import { AgendaSkeleton } from '@/components/agenda/AgendaStates'
import { AppHeader } from '@/components/app/AppHeader'
import { FilterPanel } from '@/components/app/FilterPanel'
import { SearchField } from '@/components/app/SearchField'
import { SiteFooter } from '@/components/app/SiteFooter'
import { ViewSwitcher, type ViewId } from '@/components/app/ViewSwitcher'
import ListView from '@/components/views/ListView'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useAgenda } from '@/hooks/useAgenda'
import { useMediaQuery } from '@/hooks/useMediaQuery'
import {
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

export default function App() {
  const [view, setView] = useState<ViewId>('list')
  const [query, setQuery] = useState('')
  const [subjects, setSubjects] = useState<string[]>([])
  const [professors, setProfessors] = useState<string[]>([])
  const [showCampus, setShowCampus] = useState(true)
  const [showPast, setShowPast] = useState(false)
  const [subjectsGroup, setSubjectsGroup] = useState<SubjectsGroup>('subject')
  const [subjectsTab, setSubjectsTab] = useState('all')
  const [boardGroup, setBoardGroup] = useState<BoardGroup>('timing')
  const [revision, setRevision] = useState(0)
  const [monthOverride, setMonthOverride] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [detail, setDetail] = useState<AgendaEntry | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const triggerRef = useRef<HTMLElement | null>(null)
  const desktop = useMediaQuery('(min-width: 768px)')

  const defaultQuery = useAgenda(undefined, revision)
  const today = defaultQuery.data?.today ?? ''
  const month = monthOverride ?? (today ? monthOf(today) : null)
  const calendarQuery = useAgenda(month ? monthRange(month) : undefined, revision, view === 'calendar' && month !== null)

  const allItems = defaultQuery.data?.items ?? NO_ITEMS
  const calendarItemsAll = calendarQuery.data?.items ?? NO_ITEMS

  const filters = useMemo(
    () => ({ query, subjects, instructors: professors, showCampus, showPast }),
    [query, subjects, professors, showCampus, showPast],
  )
  const filteredItems = useMemo(() => applyFilters(allItems, filters, today), [allItems, filters, today])
  const entries = useMemo(() => groupEntries(filteredItems), [filteredItems])
  const calendarItems = useMemo(
    () => applyFilters(calendarItemsAll, filters, today),
    [calendarItemsAll, filters, today],
  )

  // Facets ignore the subject and professor selections so badges stay visible to switch between.
  const facetEntries = useMemo(
    () =>
      groupEntries(
        applyFilters(allItems, { ...filters, subjects: [], instructors: [] }, today),
      ),
    [allItems, filters, today],
  )
  const subjectFacets = useMemo(() => deriveSubjects(facetEntries, subjects), [facetEntries, subjects])
  const professorFacets = useMemo(() => deriveProfessors(facetEntries, professors), [facetEntries, professors])
  const catalogHasSubjects = useMemo(() => allItems.some((item) => item.course !== null), [allItems])
  const catalogHasProfessors = useMemo(() => allItems.some((item) => item.instructor !== null), [allItems])

  const filtersActive = query.trim() !== '' || subjects.length > 0 || professors.length > 0
  const clearFilters = useCallback(() => {
    setQuery('')
    setSubjects([])
    setProfessors([])
  }, [])

  const openDetail = useCallback((entry: AgendaEntry, trigger: HTMLElement) => {
    triggerRef.current = trigger
    setDetail(entry)
    setDetailOpen(true)
  }, [])

  const refresh = useCallback(() => setRevision((value) => value + 1), [])

  const goToMonth = (next: string) => {
    setMonthOverride(next)
    setSelectedDate(today && monthOf(today) === next ? today : null)
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
  const now = Date.now()

  return (
    <>
      <a
        href="#agenda"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-xl focus:bg-primary focus:px-4 focus:py-3 focus:text-base focus:font-medium focus:text-primary-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
      >
        Skip to agenda
      </a>

      <div className="mx-auto flex min-h-dvh w-full max-w-5xl flex-col gap-6 px-4 py-6 sm:px-6 md:py-8">
        <AppHeader
          updatedAt={defaultQuery.data?.updatedAt}
          now={now}
          refreshing={refreshing}
          onRefresh={refresh}
        />

        <section aria-label="Search and filters" className="space-y-6 rounded-2xl border border-border p-4 sm:p-5">
          <SearchField value={query} onChange={setQuery} />
          <FilterPanel
            subjects={subjectFacets}
            catalogHasSubjects={catalogHasSubjects}
            selectedSubjects={subjects}
            onSubjectsChange={setSubjects}
            professors={professorFacets}
            catalogHasProfessors={catalogHasProfessors}
            selectedProfessors={professors}
            onProfessorsChange={setProfessors}
            showCampus={showCampus}
            onShowCampusChange={setShowCampus}
            showPast={showPast}
            onShowPastChange={setShowPast}
            filtersActive={filtersActive}
            onClearFilters={clearFilters}
          />
        </section>

        <main id="agenda" tabIndex={-1} aria-busy={loading} className="space-y-6 outline-none">
          <ViewSwitcher value={view} onChange={setView} />

          <p role="status" aria-live="polite" className="text-base text-muted-foreground">
            {loading || !hasData ? '' : `${formatCount(shownCount, 'item')} shown`}
          </p>

          {activeError && (
            <div className="space-y-3">
              <Alert variant="destructive" className="border-destructive bg-destructive-subtle">
                <CircleAlert aria-hidden="true" />
                <AlertTitle>Agenda unavailable</AlertTitle>
                <AlertDescription>{activeError.message}</AlertDescription>
              </Alert>
              <Button variant="outline" onClick={refresh}>
                <RefreshCw aria-hidden="true" />
                Try again
              </Button>
            </div>
          )}

          {omittedNotice && (
            <Alert role="status" className="border-input">
              <Info aria-hidden="true" />
              <AlertDescription className="text-foreground">{omittedNotice}</AlertDescription>
            </Alert>
          )}

          {loading ? (
            <AgendaSkeleton />
          ) : hasData ? (
            <Suspense fallback={<AgendaSkeleton />}>
              {view === 'list' && (
                <ListView
                  entries={entries}
                  today={today}
                  filtersActive={filtersActive}
                  onClearFilters={clearFilters}
                  onSelect={openDetail}
                />
              )}
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
                  }}
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

        <SiteFooter />
      </div>

      <ItemDetailsSheet
        entry={detail}
        open={detailOpen}
        onOpenChange={setDetailOpen}
        allItems={allItems}
        triggerRef={triggerRef}
        desktop={desktop}
      />
    </>
  )
}
