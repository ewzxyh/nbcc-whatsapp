import { Link } from 'react-router'
import { AuthShell } from '../components/auth-shell'
import { PageHeader } from '../components/page-header'
import { useDocumentTitle } from '../hooks'

export function NotFoundPage() {
  useDocumentTitle('Página não encontrada')
  return (
    <AuthShell>
      <div className="space-y-6">
        <PageHeader
          title="Página não encontrada"
          description="O endereço não existe na administração. Confira o link ou volte para a visão geral."
        />
        <Link
          to="/"
          className="inline-flex min-h-11 items-center rounded-xl bg-primary px-4 text-base font-medium text-primary-foreground hover:bg-primary-hover"
        >
          Ir para a administração
        </Link>
      </div>
    </AuthShell>
  )
}
