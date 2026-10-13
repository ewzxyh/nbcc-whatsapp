import { useMemo } from 'react'
import { DayGroups } from '@/components/agenda/DayGroups'
import { EmptyAgenda } from '@/components/agenda/AgendaStates'
import { ChoiceGroup } from '@/components/app/ChoiceGroup'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { tabValue } from '@/lib/tabValue'
import {
  entriesForProfessor,
  entriesForSubject,
  groupByDate,
  type AgendaEntry,
  type ProfessorFacet,
  type SubjectFacet,
} from '@/lib/agenda'

export type SubjectsGroup = 'subject' | 'professor'

interface SubjectsViewProps {
  entries: AgendaEntry[]
  today: string
  subjects: SubjectFacet[]
  professors: ProfessorFacet[]
  catalogHasProfessors: boolean
  group: SubjectsGroup
  onGroupChange: (group: SubjectsGroup) => void
  tab: string
  onTabChange: (tab: string) => void
  filtersActive: boolean
  onClearFilters: () => void
  onSelect: (entry: AgendaEntry, trigger: HTMLElement) => void
}

interface TabDefinition {
  value: string
  label: string
  entries: AgendaEntry[]
}

const TAB_CLASS =
  'h-11 flex-none rounded-full border border-input bg-background px-4 text-base text-foreground data-active:border-primary data-active:bg-accent data-active:font-semibold data-active:text-accent-foreground'

export default function SubjectsView({
  entries,
  today,
  subjects,
  professors,
  catalogHasProfessors,
  group,
  onGroupChange,
  tab,
  onTabChange,
  filtersActive,
  onClearFilters,
  onSelect,
}: SubjectsViewProps) {
  const tabs = useMemo<TabDefinition[]>(() => {
    // Tabs with no items under the current filters are hidden, so the active filters decide what is shown.
    const groupTabs: TabDefinition[] =
      group === 'subject'
        ? subjects.map((subject) => ({
            value: tabValue('subject', subject.code),
            label: subject.code,
            entries: entriesForSubject(entries, subject.code),
          }))
        : professors.map((professor) => ({
            value: tabValue('professor', professor.name),
            label: professor.name,
            entries: entriesForProfessor(entries, professor.name),
          }))
    return [
      { value: 'all', label: 'All', entries },
      ...groupTabs.filter((definition) => definition.entries.length > 0),
    ]
  }, [entries, subjects, professors, group])

  const activeTab = tabs.some((definition) => definition.value === tab) ? tab : 'all'

  return (
    <div className="space-y-6">
      <ChoiceGroup<SubjectsGroup>
        id="subjects-group"
        label="Group by"
        value={group}
        options={[
          { value: 'subject', label: 'Subject' },
          { value: 'professor', label: 'Professor' },
        ]}
        onChange={onGroupChange}
      />

      {group === 'professor' && !catalogHasProfessors ? (
        <p className="text-base text-muted-foreground">No professors listed yet</p>
      ) : (
        <Tabs value={activeTab} onValueChange={onTabChange} className="gap-6">
          <TabsList className="h-auto w-full flex-wrap justify-start gap-2 bg-transparent p-0">
            {tabs.map((definition) => (
              <TabsTrigger key={definition.value} value={definition.value} className={TAB_CLASS}>
                {`${definition.label} (${definition.entries.length})`}
              </TabsTrigger>
            ))}
          </TabsList>
          {tabs.map((definition) => (
            <TabsContent key={definition.value} value={definition.value}>
              {definition.entries.length > 0 ? (
                <DayGroups groups={groupByDate(definition.entries)} today={today} onSelect={onSelect} />
              ) : (
                <EmptyAgenda filtersActive={filtersActive} onClearFilters={onClearFilters} />
              )}
            </TabsContent>
          ))}
        </Tabs>
      )}
    </div>
  )
}
