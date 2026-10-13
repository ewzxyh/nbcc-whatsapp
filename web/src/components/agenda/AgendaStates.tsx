import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

export function AgendaSkeleton() {
  return (
    <div role="status" className="space-y-3">
      <span className="sr-only">Loading agenda</span>
      <Skeleton aria-hidden="true" className="h-8 w-40" />
      <Skeleton aria-hidden="true" className="h-32 w-full rounded-2xl" />
      <Skeleton aria-hidden="true" className="h-32 w-full rounded-2xl" />
      <Skeleton aria-hidden="true" className="h-32 w-full rounded-2xl" />
    </div>
  )
}

interface EmptyAgendaProps {
  filtersActive: boolean
  onClearFilters: () => void
  message?: string
}

export function EmptyAgenda({ filtersActive, onClearFilters, message }: EmptyAgendaProps) {
  const text =
    message ?? (filtersActive ? 'Nothing scheduled for this filter.' : 'Nothing scheduled in this period.')
  return (
    <div className="space-y-4 rounded-2xl border border-input p-6 text-center">
      <p className="text-base text-foreground">{text}</p>
      {filtersActive && (
        <Button variant="outline" onClick={onClearFilters}>
          Clear filters
        </Button>
      )}
    </div>
  )
}
