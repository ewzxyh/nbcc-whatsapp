import { useState } from 'react'
import type { UseFormRegisterReturn } from 'react-hook-form'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '../components/confirm-dialog'
import { Notice } from '../components/notice'
import { SecretField } from '../components/secret-field'
import { TextField } from '../components/text-field'
import { isSessionError } from '../lib/api'
import { messageForError } from '../lib/errors'
import { normalizePublicBaseUrl, type SecretAction } from '../lib/rules'
import type { AdminSettings } from '../lib/types'
import { useAdminApi } from '../use-admin-api'

interface WebhookSectionProps {
  settings: AdminSettings
  publicBaseUrlField: UseFormRegisterReturn<'publicBaseUrl'>
  publicBaseUrlError: string | null
  secretAction: SecretAction
  secretValue: string
  secretError: string | null
  onSecretActionChange: (action: SecretAction) => void
  onSecretValueChange: (value: string) => void
  onGenerateSecret: () => void
  // True right after a saved secret replacement: offers to register the webhook again.
  offerRegistration: boolean
}

interface Result {
  tone: 'success' | 'error'
  text: string
}

// The webhook address is built from the SAVED public address. Registering does not turn sending on.
export function WebhookSection({
  settings,
  publicBaseUrlField,
  publicBaseUrlError,
  secretAction,
  secretValue,
  secretError,
  onSecretActionChange,
  onSecretValueChange,
  onGenerateSecret,
  offerRegistration,
}: WebhookSectionProps) {
  const api = useAdminApi()
  const [confirmRegister, setConfirmRegister] = useState(false)
  const [registering, setRegistering] = useState(false)
  const [result, setResult] = useState<Result | null>(null)
  const webhookUrl = settings.publicBaseUrl
    ? `${normalizePublicBaseUrl(settings.publicBaseUrl)}/webhooks/grapnel`
    : null

  async function registerWebhook() {
    setRegistering(true)
    setResult(null)
    try {
      const registered = await api.registerWebhook()
      setResult({ tone: 'success', text: `Webhook registrado. Endereço: ${registered.url}` })
    } catch (error) {
      if (!isSessionError(error)) setResult({ tone: 'error', text: messageForError(error) })
    } finally {
      setRegistering(false)
    }
  }

  return (
    <section aria-labelledby="webhook-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
      <h2 id="webhook-heading" className="text-lg font-semibold text-foreground">
        Webhook e endereço público
      </h2>
      <TextField
        id="publicBaseUrl"
        label="Endereço público do serviço"
        help="Endereço HTTPS, sem caminho. Exemplo: https://seu-dominio.exemplo"
        autoCapitalize="none"
        autoComplete="url"
        spellCheck={false}
        inputMode="url"
        {...publicBaseUrlField}
        error={publicBaseUrlError}
      />
      <SecretField
        id="webhookSecret"
        label="Segredo do webhook"
        help="Use pelo menos 32 caracteres. Gere um novo segredo com o botão ao lado."
        configured={settings.webhookSecret.configured}
        action={secretAction}
        value={secretValue}
        error={secretError}
        removeTitle="Remover o segredo do webhook?"
        onActionChange={onSecretActionChange}
        onValueChange={onSecretValueChange}
        onGenerate={onGenerateSecret}
        generateLabel="Gerar novo segredo"
      />
      <p className="text-base text-muted-foreground">
        {webhookUrl ? (
          <>
            Endereço do webhook: <span className="break-all font-medium text-foreground">{webhookUrl}</span>
          </>
        ) : (
          'Salve o endereço público do serviço para ver o endereço do webhook.'
        )}
      </p>
      <div className="space-y-3">
        {offerRegistration ? (
          <Notice tone="info">
            O segredo foi salvo. Registre o webhook de novo para o Grapnel usar o novo segredo.{' '}
            <Button type="button" onClick={() => setConfirmRegister(true)} className="mt-2 sm:mt-0 sm:ml-3">
              Registrar webhook agora
            </Button>
          </Notice>
        ) : null}
        <Button
          type="button"
          variant="outline"
          onClick={() => setConfirmRegister(true)}
          disabled={registering}
          aria-busy={registering}
        >
          Registrar webhook no Grapnel
        </Button>
        <p className="text-sm text-muted-foreground">Usa o endereço público salvo. Não liga o envio de mensagens.</p>
        {result ? <Notice tone={result.tone}>{result.text}</Notice> : null}
      </div>
      <ConfirmDialog
        open={confirmRegister}
        onOpenChange={setConfirmRegister}
        title="Registrar webhook no Grapnel?"
        description={`O Grapnel passará a enviar eventos para ${webhookUrl ?? 'o endereço público salvo'}. Esta ação não liga o envio de mensagens.`}
        confirmLabel="Registrar webhook"
        onConfirm={() => void registerWebhook()}
      />
    </section>
  )
}
