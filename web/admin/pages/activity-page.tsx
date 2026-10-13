import { LoadErrorState, LoadingState } from '../components/load-state'
import { PageHeader } from '../components/page-header'
import { useAsync, useDocumentTitle } from '../hooks'
import { formatDateTime } from '../lib/dates'
import { messageForError } from '../lib/errors'
import { auditActionLabel, fieldLabel } from '../lib/labels'
import { useAdminApi } from '../use-admin-api'

export function ActivityPage() {
  useDocumentTitle('Atividade')
  const api = useAdminApi()
  const loaded = useAsync(() => api.activity())
  const entries = loaded.data

  return (
    <>
      <PageHeader
        title="Atividade"
        description="Ações recentes na administração, da mais nova para a mais antiga. Valores e endereços IP não são registrados."
        showTimezone
      />
      {entries === undefined ? (
        loaded.error ? (
          <LoadErrorState message={messageForError(loaded.error)} onRetry={loaded.reload} />
        ) : (
          <LoadingState />
        )
      ) : entries.length === 0 ? (
        <p className="text-base text-muted-foreground">Nenhuma atividade registrada.</p>
      ) : (
        <ol className="divide-y divide-border rounded-2xl border border-border bg-card">
          {entries.map((entry, index) => (
            <li key={`${entry.at}-${index}`} className="space-y-1 px-5 py-4">
              <p className="text-base font-medium text-foreground">{auditActionLabel(entry.action)}</p>
              <p className="text-sm text-muted-foreground">
                <time dateTime={entry.at}>{formatDateTime(entry.at)}</time>
              </p>
              {entry.fields.length > 0 ? (
                <p className="text-sm break-words text-foreground">
                  Campos alterados: {entry.fields.map((field) => fieldLabel(field)).join(', ')}
                </p>
              ) : null}
            </li>
          ))}
        </ol>
      )}
    </>
  )
}
