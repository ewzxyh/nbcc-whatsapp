import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@/components/ui/button'
import { useDrafts } from '../drafts'
import { ConfirmDialog } from '../components/confirm-dialog'
import { ErrorSummary, type ErrorSummaryItem } from '../components/error-summary'
import { LoadErrorState, LoadingState } from '../components/load-state'
import { Notice } from '../components/notice'
import { PageHeader } from '../components/page-header'
import { SecretField } from '../components/secret-field'
import { TextField } from '../components/text-field'
import { UnsavedChangesGuard } from '../components/unsaved-changes-guard'
import { useAsync, useDocumentTitle, useFormDraft } from '../hooks'
import { isSessionError } from '../lib/api'
import { formatCalendarDate } from '../lib/dates'
import { messageForError } from '../lib/errors'
import { errorText, formMessageForSaveError, summaryFromErrors } from '../lib/form-errors'
import { omissionReasonLabel, plural } from '../lib/labels'
import { agendaFormSchema } from '../lib/schemas'
import {
  agendaFormFromSettings,
  buildAgendaPatch,
  sanitizeAgendaForDraft,
  type AgendaFormValues,
} from '../lib/settings-form'
import type { SecretAction } from '../lib/rules'
import type { AdminSettings, AdminStatus, FeedTestResult } from '../lib/types'
import { useAdminApi } from '../use-admin-api'

const DRAFT_KEY = 'agenda'
const FORM_FIELDS = ['nbccFeedUrl', 'agendaCacheMinutes'] as const

function describeFeedTest(result: FeedTestResult): string {
  return `Conexão funcionando. ${plural(result.events, 'evento', 'eventos')}, ${plural(
    result.courses,
    'disciplina',
    'disciplinas',
  )} e ${plural(result.omittedEvents, 'evento omitido', 'eventos omitidos')}.`
}

export function AgendaPage() {
  useDocumentTitle('Agenda pública')
  const api = useAdminApi()
  const drafts = useDrafts()
  const loaded = useAsync(async () => {
    const [settings, status] = await Promise.all([api.settings(), api.status()])
    return { settings, status }
  })
  const data = loaded.data
  const baseline = useMemo<AgendaFormValues | null>(
    () => (data ? agendaFormFromSettings(data.settings) : null),
    [data],
  )
  const form = useForm<AgendaFormValues>({
    resolver: zodResolver(agendaFormSchema),
    defaultValues: { nbccFeedUrl: '', nbccFeedUrlAction: 'keep', agendaCacheMinutes: '' },
  })
  const dirty = useFormDraft(DRAFT_KEY, form, baseline, sanitizeAgendaForDraft)
  const { register, handleSubmit, setError, setValue, clearErrors, watch, formState } = form
  const { errors, isSubmitting, submitCount } = formState
  const action = watch('nbccFeedUrlAction')
  const feedValue = watch('nbccFeedUrl')

  const [formError, setFormError] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)
  const [testMessage, setTestMessage] = useState<string | null>(null)
  const [testError, setTestError] = useState<string | null>(null)
  const [pendingValues, setPendingValues] = useState<AgendaFormValues | null>(null)

  async function save(values: AgendaFormValues) {
    setSaved(null)
    setFormError(null)
    if (!baseline) return
    const patch = buildAgendaPatch(values, baseline)
    if (Object.keys(patch).length === 0) {
      setSaved('Nenhuma alteração para salvar.')
      return
    }
    try {
      await api.saveSettings(patch)
      drafts.clear(DRAFT_KEY)
      setSaved('Alterações salvas.')
      loaded.reload()
    } catch (error) {
      if (isSessionError(error)) return
      setFormError(formMessageForSaveError(error, setError, FORM_FIELDS))
    }
  }

  const onSubmit = handleSubmit(async (values) => {
    // While sending is on, a new feed can make the group receive a message. Ask before saving.
    if (values.nbccFeedUrlAction === 'replace' && data?.settings.sendEnabled === true) {
      setPendingValues(values)
      return
    }
    await save(values)
  })

  async function runTest() {
    setTesting(true)
    setTestMessage(null)
    setTestError(null)
    try {
      const result = await api.testFeed()
      setTestMessage(describeFeedTest(result))
    } catch (error) {
      if (!isSessionError(error)) setTestError(messageForError(error))
    } finally {
      setTesting(false)
    }
  }

  function changeFeedAction(next: SecretAction) {
    clearErrors('nbccFeedUrl')
    setValue('nbccFeedUrlAction', next, { shouldDirty: true })
  }

  const items: ErrorSummaryItem[] = [
    ...(submitCount > 0 ? summaryFromErrors(errors) : []),
    ...(formError ? [{ message: formError }] : []),
  ]

  const header = (
    <PageHeader
      title="Agenda pública"
      description="Calendário NBCC, atualização da agenda e eventos que a agenda não pôde mostrar."
      showTimezone
    />
  )

  if (!data) {
    return (
      <>
        {header}
        {loaded.error ? (
          <LoadErrorState message={messageForError(loaded.error)} onRetry={loaded.reload} />
        ) : (
          <LoadingState />
        )}
      </>
    )
  }

  const status: AdminStatus = data.status
  const settings: AdminSettings = data.settings
  const omitted = status.agenda.omittedEvents

  return (
    <>
      {header}
      <UnsavedChangesGuard dirty={dirty} onDiscard={() => drafts.clear(DRAFT_KEY)} />
      <ConfirmDialog
        open={pendingValues !== null}
        onOpenChange={(open) => {
          if (!open) setPendingValues(null)
        }}
        title="Salvar a troca do link?"
        description="Trocar o link pode gerar uma mensagem para o grupo, pois a agenda comparada muda."
        confirmLabel="Salvar alterações"
        onConfirm={() => {
          const values = pendingValues
          setPendingValues(null)
          if (values) void save(values)
        }}
      />

      <form className="space-y-6" noValidate onSubmit={onSubmit}>
        {items.length > 0 ? <ErrorSummary items={items} /> : null}
        {saved ? <Notice tone="success">{saved}</Notice> : null}

        <section aria-labelledby="calendar-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
          <h2 id="calendar-heading" className="text-lg font-semibold text-foreground">
            Calendário NBCC
          </h2>
          <SecretField
            id="nbccFeedUrl"
            label="Link do calendário NBCC"
            help="O link privado do calendário do Brightspace. Ele fica criptografado e não é exibido depois de salvo."
            configured={settings.nbccFeedUrl.configured}
            action={action}
            value={feedValue}
            error={errorText(errors.nbccFeedUrl, 'nbccFeedUrl')}
            removeTitle="Remover o link do calendário NBCC?"
            onActionChange={changeFeedAction}
            onValueChange={(next) => setValue('nbccFeedUrl', next, { shouldDirty: true })}
          />
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void runTest()}
              disabled={testing || !settings.nbccFeedUrl.configured}
              aria-busy={testing}
            >
              Testar conexão
            </Button>
            {action !== 'keep' ? (
              <p className="text-sm text-muted-foreground">O teste usa o link salvo. Salve a alteração antes de testar.</p>
            ) : null}
          </div>
          {!settings.nbccFeedUrl.configured ? (
            <p className="text-sm text-muted-foreground">Configure o link para testar a conexão.</p>
          ) : null}
          {testMessage ? <Notice tone="success">{testMessage}</Notice> : null}
          {testError ? <Notice tone="error">{testError}</Notice> : null}
        </section>

        <section aria-labelledby="cache-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
          <h2 id="cache-heading" className="text-lg font-semibold text-foreground">
            Atualização da agenda
          </h2>
          <TextField
            id="agendaCacheMinutes"
            label="Atualização da agenda pública (minutos)"
            help="Padrão: 60 minutos. Entre 1 e 1440."
            inputMode="numeric"
            {...register('agendaCacheMinutes')}
            error={errorText(errors.agendaCacheMinutes, 'agendaCacheMinutes')}
          />
          <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
            Salvar alterações
          </Button>
        </section>
      </form>

      <section id="eventos-omitidos" aria-labelledby="omitted-heading" className="space-y-4 rounded-2xl border border-border bg-card p-5">
        <h2 id="omitted-heading" className="text-lg font-semibold text-foreground">
          Eventos omitidos
        </h2>
        {omitted.length === 0 ? (
          <p className="text-base text-muted-foreground">Nenhum evento foi omitido.</p>
        ) : (
          <>
            <p className="text-base text-muted-foreground">
              Corrija o evento no Brightspace. A agenda é atualizada automaticamente.
            </p>
            <ul className="divide-y divide-border">
              {omitted.map((event, index) => (
                <li key={`${event.date}-${index}`} className="py-3">
                  <p className="font-medium break-words text-foreground">{event.title}</p>
                  <p className="text-sm text-muted-foreground">
                    {formatCalendarDate(event.date)} · {omissionReasonLabel(event.reason)}
                  </p>
                </li>
              ))}
            </ul>
          </>
        )}
      </section>
    </>
  )
}
