import { useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
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
import { messageForError } from '../lib/errors'
import { errorText, formMessageForSaveError, summaryFromErrors } from '../lib/form-errors'
import { generateWebhookSecret } from '../lib/secrets'
import { whatsappFormSchema } from '../lib/schemas'
import {
  buildWhatsAppPatch,
  sanitizeWhatsAppForDraft,
  whatsappFormFromSettings,
  type WhatsAppFormValues,
} from '../lib/settings-form'
import type { SecretAction } from '../lib/rules'
import { useAdminApi } from '../use-admin-api'
import { WebhookSection } from './whatsapp-webhook-section'

const DRAFT_KEY = 'whatsapp'
const FORM_FIELDS = [
  'sendEnabled',
  'pollIntervalMinutes',
  'grapnelApiKey',
  'grapnelInstanceId',
  'grapnelGroupJid',
  'grapnelGroupName',
  'publicBaseUrl',
  'webhookSecret',
] as const

export function WhatsAppPage() {
  useDocumentTitle('WhatsApp e Grapnel')
  const api = useAdminApi()
  const drafts = useDrafts()
  const loaded = useAsync(() => api.settings())
  const settings = loaded.data
  const baseline = useMemo<WhatsAppFormValues | null>(
    () => (settings ? whatsappFormFromSettings(settings) : null),
    [settings],
  )
  const form = useForm<WhatsAppFormValues>({
    resolver: zodResolver(whatsappFormSchema),
    defaultValues: {
      sendEnabled: false,
      pollIntervalMinutes: '',
      grapnelApiKey: '',
      grapnelApiKeyAction: 'keep',
      grapnelInstanceId: '',
      grapnelGroupJid: '',
      grapnelGroupName: '',
      publicBaseUrl: '',
      webhookSecret: '',
      webhookSecretAction: 'keep',
    },
  })
  const dirty = useFormDraft(DRAFT_KEY, form, baseline, sanitizeWhatsAppForDraft)
  const { register, handleSubmit, setError, setValue, clearErrors, watch, control, formState } = form
  const { errors, isSubmitting, submitCount } = formState

  const [formError, setFormError] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)
  const [confirmSend, setConfirmSend] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testMessage, setTestMessage] = useState<string | null>(null)
  const [testError, setTestError] = useState<string | null>(null)
  const [pendingValues, setPendingValues] = useState<WhatsAppFormValues | null>(null)
  const [secretReplaced, setSecretReplaced] = useState(false)

  const apiKeyAction = watch('grapnelApiKeyAction')
  const apiKeyValue = watch('grapnelApiKey')
  const webhookAction = watch('webhookSecretAction')
  const webhookValue = watch('webhookSecret')

  async function save(values: WhatsAppFormValues) {
    setSaved(null)
    setFormError(null)
    if (!baseline) return
    const patch = buildWhatsAppPatch(values, baseline)
    if (Object.keys(patch).length === 0) {
      setSaved('Nenhuma alteração para salvar.')
      return
    }
    try {
      await api.saveSettings(patch)
      drafts.clear(DRAFT_KEY)
      setSecretReplaced(typeof patch.webhookSecret === 'string')
      setSaved('Alterações salvas.')
      loaded.reload()
    } catch (error) {
      if (isSessionError(error)) return
      setFormError(formMessageForSaveError(error, setError, FORM_FIELDS))
    }
  }

  const onSubmit = handleSubmit(async (values) => {
    // Replacing an existing webhook secret makes Grapnel's notices fail until the webhook is registered again.
    if (values.webhookSecretAction === 'replace' && settings?.webhookSecret.configured === true) {
      setPendingValues(values)
      return
    }
    await save(values)
  })

  async function runGrapnelTest() {
    setTesting(true)
    setTestMessage(null)
    setTestError(null)
    try {
      const result = await api.testGrapnel()
      setTestMessage(`Grupo encontrado: ${result.groupName}.`)
    } catch (error) {
      if (!isSessionError(error)) setTestError(messageForError(error))
    } finally {
      setTesting(false)
    }
  }

  function changeAction(field: 'grapnelApiKey' | 'webhookSecret', next: SecretAction) {
    clearErrors(field)
    setValue(`${field}Action`, next, { shouldDirty: true })
  }

  function generateSecret() {
    clearErrors('webhookSecret')
    setValue('webhookSecret', generateWebhookSecret(), { shouldDirty: true })
    setValue('webhookSecretAction', 'replace', { shouldDirty: true })
  }

  const items: ErrorSummaryItem[] = [
    ...(submitCount > 0 ? summaryFromErrors(errors) : []),
    ...(formError ? [{ message: formError }] : []),
  ]

  const header = (
    <PageHeader
      title="WhatsApp e Grapnel"
      description="Envio para o grupo, conexão com o Grapnel e webhook."
      showTimezone
    />
  )

  if (!settings) {
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

  return (
    <>
      {header}
      <UnsavedChangesGuard dirty={dirty} onDiscard={() => drafts.clear(DRAFT_KEY)} />
      <ConfirmDialog
        open={pendingValues !== null}
        onOpenChange={(open) => {
          if (!open) setPendingValues(null)
        }}
        title="Trocar o segredo do webhook?"
        description="Trocar o segredo faz os avisos do Grapnel serem recusados até você registrar o webhook de novo."
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

        <section aria-labelledby="send-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
          <h2 id="send-heading" className="text-lg font-semibold text-foreground">
            Envio para WhatsApp
          </h2>
          <p className="text-base text-muted-foreground">
            Com o envio desligado, o serviço só acompanha o calendário. Nenhuma mensagem é enviada ao grupo.
          </p>
          <Controller
            control={control}
            name="sendEnabled"
            render={({ field }) => (
              <div className="space-y-2">
                <label className="flex min-h-11 cursor-pointer items-center gap-3">
                  <Switch
                    id="sendEnabled"
                    checked={field.value}
                    onCheckedChange={(checked) => {
                      if (checked) setConfirmSend(true)
                      else field.onChange(false)
                    }}
                  />
                  <span className="text-base font-medium text-foreground">Enviar mensagens para o grupo</span>
                </label>
                {errorText(errors.sendEnabled, 'sendEnabled') ? (
                  <p className="text-sm font-medium text-destructive">{errorText(errors.sendEnabled, 'sendEnabled')}</p>
                ) : null}
              </div>
            )}
          />
          <TextField
            id="pollIntervalMinutes"
            label="Verificação do calendário (minutos)"
            help="Padrão: 5 minutos. Mínimo: 1."
            inputMode="numeric"
            {...register('pollIntervalMinutes')}
            error={errorText(errors.pollIntervalMinutes, 'pollIntervalMinutes')}
          />
        </section>

        <section aria-labelledby="grapnel-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
          <h2 id="grapnel-heading" className="text-lg font-semibold text-foreground">
            Grapnel
          </h2>
          <SecretField
            id="grapnelApiKey"
            label="Chave de API do Grapnel"
            configured={settings.grapnelApiKey.configured}
            action={apiKeyAction}
            value={apiKeyValue}
            error={errorText(errors.grapnelApiKey, 'grapnelApiKey')}
            removeTitle="Remover a chave de API do Grapnel?"
            onActionChange={(next) => changeAction('grapnelApiKey', next)}
            onValueChange={(next) => setValue('grapnelApiKey', next, { shouldDirty: true })}
          />
          <TextField
            id="grapnelInstanceId"
            label="ID da instância do WhatsApp"
            autoComplete="off"
            spellCheck={false}
            {...register('grapnelInstanceId')}
            error={errorText(errors.grapnelInstanceId, 'grapnelInstanceId')}
          />
          <TextField
            id="grapnelGroupJid"
            label="ID do grupo do WhatsApp"
            help="Opcional se o nome do grupo estiver definido."
            autoComplete="off"
            spellCheck={false}
            {...register('grapnelGroupJid')}
            error={errorText(errors.grapnelGroupJid, 'grapnelGroupJid')}
          />
          <TextField
            id="grapnelGroupName"
            label="Nome do grupo"
            help="Usado para encontrar o grupo quando o ID não está definido."
            autoComplete="off"
            {...register('grapnelGroupName')}
            error={errorText(errors.grapnelGroupName, 'grapnelGroupName')}
          />
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void runGrapnelTest()}
              disabled={testing}
              aria-busy={testing}
            >
              Testar conexão com Grapnel
            </Button>
            <p className="text-sm text-muted-foreground">Usa as configurações salvas. Não envia mensagens.</p>
          </div>
          {testMessage ? <Notice tone="success">{testMessage}</Notice> : null}
          {testError ? <Notice tone="error">{testError}</Notice> : null}
        </section>

        <WebhookSection
          settings={settings}
          publicBaseUrlField={register('publicBaseUrl')}
          publicBaseUrlError={errorText(errors.publicBaseUrl, 'publicBaseUrl')}
          secretAction={webhookAction}
          secretValue={webhookValue}
          secretError={errorText(errors.webhookSecret, 'webhookSecret')}
          onSecretActionChange={(next) => changeAction('webhookSecret', next)}
          onSecretValueChange={(next) => setValue('webhookSecret', next, { shouldDirty: true })}
          onGenerateSecret={generateSecret}
          offerRegistration={secretReplaced}
        />

        <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
          Salvar alterações
        </Button>
      </form>

      <ConfirmDialog
        open={confirmSend}
        onOpenChange={setConfirmSend}
        title="Ligar o envio para o grupo?"
        description="Depois que você salvar, o serviço poderá enviar mensagens reais do WhatsApp para o grupo. Confira o grupo e as configurações antes de salvar."
        confirmLabel="Ligar envio"
        onConfirm={() => setValue('sendEnabled', true, { shouldDirty: true })}
      />
    </>
  )
}
