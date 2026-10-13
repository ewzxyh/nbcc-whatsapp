import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Notice } from './notice'

export function LoadingState({ label = 'Carregando…' }: { label?: string }) {
  return (
    <div role="status" className="space-y-4">
      <p className="text-base text-muted-foreground">{label}</p>
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-24 w-full" />
    </div>
  )
}

interface LoadErrorStateProps {
  message?: string
  onRetry: () => void
}

export function LoadErrorState({
  message = 'Não foi possível carregar esta página. Verifique sua conexão.',
  onRetry,
}: LoadErrorStateProps) {
  return (
    <div className="space-y-4">
      <Notice tone="error">{message}</Notice>
      <Button variant="outline" onClick={onRetry}>
        Tentar novamente
      </Button>
    </div>
  )
}
