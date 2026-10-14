import { CheckIcon } from '@solar-icons/react/linear'
import { Badge } from '@/components/ui/badge'

// Status is always a text label, never colour alone.
export function ConfiguredBadge({ configured }: { configured: boolean }) {
  return configured ? (
    <Badge variant="default">
      <CheckIcon aria-hidden="true" />
      Configurado
    </Badge>
  ) : (
    <Badge variant="outline">Não configurado</Badge>
  )
}
