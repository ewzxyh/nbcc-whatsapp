import { Check } from 'lucide-react'
import { Badge } from '@/components/ui/badge'

// Status is always a text label, never colour alone.
export function ConfiguredBadge({ configured }: { configured: boolean }) {
  return configured ? (
    <Badge variant="default">
      <Check aria-hidden="true" />
      Configurado
    </Badge>
  ) : (
    <Badge variant="outline">Não configurado</Badge>
  )
}
