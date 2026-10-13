import { useState } from 'react'
import { Link } from 'react-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfiguredBadge } from '../components/status-badge'
import { Notice } from '../components/notice'
import { PageHeader } from '../components/page-header'
import { LoadErrorState, LoadingState } from '../components/load-state'
import { useAsync, useDocumentTitle } from '../hooks'
import { useAdminApi } from '../use-admin-api'
import { isSessionError } from '../lib/api'
import { messageForCode, messageForError } from '../lib/errors'
import { formatDateTime } from '../lib/dates'
import { plural } from '../lib/labels'
import type { AdminStatus, CheckResult } from '../lib/types'

function describeCheck(result: CheckResult): string {
  const feed = result.feedChanged ? 'O calendário mudou.' : 'O calendário não mudou.'
  const queue = result.queued
    ? 'Novas mensagens foram colocadas na fila.'
    : 'Nenhuma mensagem nova foi colocada na fila.'
  const send = result.sendEnabled ? '' : ' O envio para o grupo está desativado.'
  return `Verificação concluída. ${feed} ${queue}${send}`
}

function ConfigurationRow({
  label,
  configured,
  to,
}: {
  label: string
  configured: boolean
  to: string
}) {
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 py-3">
      <span className="text-base font-medium text-foreground">{label}</span>
      <div className="flex flex-wrap items-center gap-4">
        <ConfiguredBadge configured={configured} />
        <Link
          to={to}
          className="inline-flex min-h-11 items-center rounded-xl px-2 text-base font-medium text-primary underline underline-offset-4"
        >
          {configured ? 'Revisar' : 'Configurar'}
          <span className="sr-only"> {label}</span>
        </Link>
      </div>
    </li>
  )
}

export function OverviewPage() {
  useDocumentTitle('Visão geral')
  const api = useAdminApi()
  const status = useAsync(() => api.status())
  const [checking, setChecking] = useState(false)
  const [checkMessage, setCheckMessage] = useState<string | null>(null)
  const [checkError, setCheckError] = useState<string | null>(null)

  async function runCheck() {
    setChecking(true)
    setCheckMessage(null)
    setCheckError(null)
    try {
      const result = await api.check()
      setCheckMessage(describeCheck(result))
      status.reload()
    } catch (error) {
      if (!isSessionError(error)) setCheckError(messageForError(error))
    } finally {
      setChecking(false)
    }
  }

  const header = (
    <PageHeader
      title="Visão geral"
      description="Situação da agenda pública, do envio para o WhatsApp e da configuração."
      showTimezone
    />
  )

  if (!status.data) {
    return (
      <>
        {header}
        {status.error ? (
          <LoadErrorState message={messageForError(status.error)} onRetry={status.reload} />
        ) : (
          <LoadingState />
        )}
      </>
    )
  }

  const data: AdminStatus = status.data
  const { agenda, whatsapp, configured } = data
  const omitted = agenda.omittedEvents.length

  return (
    <>
      {header}
      <section aria-labelledby="agenda-heading" className="space-y-4 rounded-2xl border border-border bg-card p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="agenda-heading" className="text-lg font-semibold text-foreground">
            Agenda pública
          </h2>
          <Badge variant={agenda.stale ? 'outline' : 'default'}>
            {agenda.stale ? 'Desatualizada' : 'Atualizada'}
          </Badge>
        </div>
        <p className="text-sm text-muted-foreground">
          Desatualizada significa que a cópia é mais antiga que o tempo de atualização configurado.
        </p>
        <dl className="grid gap-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
          <dt className="text-base text-muted-foreground">Última atualização</dt>
          <dd className="text-base text-foreground">
            {agenda.updatedAt ? (
              <time dateTime={agenda.updatedAt}>{formatDateTime(agenda.updatedAt)}</time>
            ) : (
              'Ainda não atualizada'
            )}
          </dd>
          <dt className="text-base text-muted-foreground">Eventos omitidos</dt>
          <dd className="text-base text-foreground">
            {omitted === 0 ? (
              'Nenhum'
            ) : (
              <>
                {plural(omitted, 'evento omitido', 'eventos omitidos')}.{' '}
                <Link
                  to="/agenda#eventos-omitidos"
                  className="inline-flex min-h-11 items-center underline underline-offset-4"
                >
                  Ver eventos omitidos
                </Link>
              </>
            )}
          </dd>
        </dl>
        {agenda.lastError ? <Notice tone="error">{messageForCode(agenda.lastError)}</Notice> : null}
      </section>

      <section aria-labelledby="whatsapp-heading" className="space-y-4 rounded-2xl border border-border bg-card p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="whatsapp-heading" className="text-lg font-semibold text-foreground">
            Envio para WhatsApp
          </h2>
          <Badge variant={whatsapp.sendEnabled ? 'default' : 'outline'}>
            {whatsapp.sendEnabled ? 'Ativado' : 'Desativado'}
          </Badge>
        </div>
        <dl className="grid gap-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
          <dt className="text-base text-muted-foreground">Mensagens pendentes</dt>
          <dd className="text-base text-foreground">{whatsapp.pendingJobs}</dd>
          <dt className="text-base text-muted-foreground">Falhas</dt>
          <dd className="text-base text-foreground">{whatsapp.failedJobs}</dd>
          <dt className="text-base text-muted-foreground">Última verificação</dt>
          <dd className="text-base text-foreground">
            {whatsapp.lastCheckAt ? (
              <time dateTime={whatsapp.lastCheckAt}>{formatDateTime(whatsapp.lastCheckAt)}</time>
            ) : (
              'Nenhuma verificação ainda'
            )}
          </dd>
        </dl>
        {whatsapp.lastError ? <Notice tone="error">{messageForCode(whatsapp.lastError)}</Notice> : null}
      </section>

      <section aria-labelledby="configuration-heading" className="rounded-2xl border border-border bg-card p-5">
        <h2 id="configuration-heading" className="text-lg font-semibold text-foreground">
          Configuração
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          O Grapnel fica configurado quando a chave, o ID da instância e o ID do grupo estão salvos.
        </p>
        <ul className="mt-2 divide-y divide-border">
          <ConfigurationRow label="Calendário NBCC" configured={configured.nbccFeed} to="/agenda" />
          <ConfigurationRow label="Grapnel" configured={configured.grapnel} to="/whatsapp" />
          <ConfigurationRow label="Segredo do webhook" configured={configured.webhookSecret} to="/whatsapp" />
          <ConfigurationRow label="Endereço público" configured={configured.publicBaseUrl} to="/whatsapp" />
        </ul>
      </section>

      <section aria-labelledby="check-heading" className="space-y-4">
        <h2 id="check-heading" className="text-lg font-semibold text-foreground">
          Verificação
        </h2>
        <p className="text-base text-muted-foreground">
          Confere o calendário agora e coloca na fila as mensagens novas, se houver.
        </p>
        <Button onClick={() => void runCheck()} disabled={checking} aria-busy={checking}>
          Verificar agora
        </Button>
        {checkMessage ? <Notice tone="success">{checkMessage}</Notice> : null}
        {checkError ? <Notice tone="error">{checkError}</Notice> : null}
      </section>
    </>
  )
}
