import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '../components/confirm-dialog'
import { ErrorSummary, type ErrorSummaryItem } from '../components/error-summary'
import { LogoutButton } from '../components/logout-button'
import { Notice } from '../components/notice'
import { PageHeader } from '../components/page-header'
import { PasswordField } from '../components/password-field'
import { TextField } from '../components/text-field'
import { useDocumentTitle } from '../hooks'
import { isApiError, isSessionError } from '../lib/api'
import { messageForError } from '../lib/errors'
import { errorText, formMessageForSaveError, summaryFromErrors } from '../lib/form-errors'
import { emailChangeSchema, passwordChangeSchema } from '../lib/schemas'
import { useSession } from '../session'
import { useAdminApi } from '../use-admin-api'

interface PasswordValues {
  currentPassword: string
  newPassword: string
  confirmPassword: string
}

interface EmailValues {
  currentPassword: string
  newEmail: string
}

interface Outcome {
  tone: 'success' | 'error'
  text: string
}

const PASSWORD_FIELDS = ['currentPassword', 'newPassword', 'confirmPassword'] as const
const EMAIL_FIELDS = ['currentPassword', 'newEmail'] as const

export function SecurityPage() {
  useDocumentTitle('Segurança')
  const api = useAdminApi()
  const { email, completeAuth, setEmail } = useSession()
  const [passwordFormError, setPasswordFormError] = useState<string | null>(null)
  const [passwordOutcome, setPasswordOutcome] = useState<Outcome | null>(null)
  const [emailFormError, setEmailFormError] = useState<string | null>(null)
  const [emailOutcome, setEmailOutcome] = useState<Outcome | null>(null)
  const [revokeOpen, setRevokeOpen] = useState(false)
  const [revokeOutcome, setRevokeOutcome] = useState<Outcome | null>(null)

  const passwordForm = useForm<PasswordValues>({
    resolver: zodResolver(passwordChangeSchema(email)),
    defaultValues: { currentPassword: '', newPassword: '', confirmPassword: '' },
  })
  const emailForm = useForm<EmailValues>({
    resolver: zodResolver(emailChangeSchema),
    defaultValues: { currentPassword: '', newEmail: '' },
  })

  const submitPassword = passwordForm.handleSubmit(async (values) => {
    setPasswordFormError(null)
    setPasswordOutcome(null)
    try {
      const result = await api.changePassword(values.currentPassword, values.newPassword)
      // The password response carries only the rotated token. The e-mail stays the one already in the session.
      completeAuth({ ...result, email })
      passwordForm.reset()
      setPasswordOutcome({
        tone: 'success',
        text: 'Senha alterada. Os outros dispositivos foram desconectados.',
      })
    } catch (error) {
      if (isSessionError(error)) return
      if (isApiError(error) && error.code === 'invalid_credentials') {
        passwordForm.setError('currentPassword', { type: 'server', message: 'invalid_credentials' })
        return
      }
      setPasswordFormError(formMessageForSaveError(error, passwordForm.setError, PASSWORD_FIELDS))
    }
  })

  const submitEmail = emailForm.handleSubmit(async (values) => {
    setEmailFormError(null)
    setEmailOutcome(null)
    const next = values.newEmail.trim().toLowerCase()
    try {
      await api.changeEmail(values.currentPassword, next)
      setEmail(next)
      emailForm.reset()
      setEmailOutcome({ tone: 'success', text: 'E-mail de acesso alterado.' })
    } catch (error) {
      if (isSessionError(error)) return
      if (isApiError(error) && error.code === 'invalid_credentials') {
        emailForm.setError('currentPassword', { type: 'server', message: 'invalid_credentials' })
        return
      }
      if (isApiError(error) && error.fields.email) {
        // The server reports e-mail problems under `email`. The form shows them on the new e-mail field.
        emailForm.setError('newEmail', { type: 'server', message: error.fields.email })
        return
      }
      setEmailFormError(formMessageForSaveError(error, emailForm.setError, EMAIL_FIELDS))
    }
  })

  async function revokeOthers() {
    setRevokeOutcome(null)
    try {
      await api.revokeOtherSessions()
      setRevokeOutcome({ tone: 'success', text: 'Outras sessões foram encerradas.' })
    } catch (error) {
      if (!isSessionError(error)) setRevokeOutcome({ tone: 'error', text: messageForError(error) })
    }
  }

  const passwordErrors = passwordForm.formState
  const emailErrors = emailForm.formState
  const passwordItems: ErrorSummaryItem[] = [
    ...(passwordErrors.submitCount > 0 ? summaryFromErrors(passwordErrors.errors) : []),
    ...(passwordFormError ? [{ message: passwordFormError }] : []),
  ]
  const emailItems: ErrorSummaryItem[] = [
    ...(emailErrors.submitCount > 0 ? summaryFromErrors(emailErrors.errors, undefined, 'email-') : []),
    ...(emailFormError ? [{ message: emailFormError }] : []),
  ]

  return (
    <>
      <PageHeader title="Segurança" description="Senha, e-mail de acesso e sessões abertas." />

      <section aria-labelledby="password-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
        <h2 id="password-heading" className="text-lg font-semibold text-foreground">
          Alterar senha
        </h2>
        <p className="text-base text-muted-foreground">
          Use pelo menos 12 caracteres. Os outros dispositivos serão desconectados.
        </p>
        {passwordItems.length > 0 ? <ErrorSummary items={passwordItems} /> : null}
        {passwordOutcome ? <Notice tone={passwordOutcome.tone}>{passwordOutcome.text}</Notice> : null}
        <form className="space-y-6" noValidate onSubmit={submitPassword}>
          <PasswordField
            id="currentPassword"
            label="Senha atual"
            autoComplete="current-password"
            {...passwordForm.register('currentPassword')}
            error={errorText(passwordErrors.errors.currentPassword, 'currentPassword')}
          />
          <PasswordField
            id="newPassword"
            label="Nova senha"
            autoComplete="new-password"
            help="Não use seu e-mail como senha."
            {...passwordForm.register('newPassword')}
            error={errorText(passwordErrors.errors.newPassword, 'newPassword')}
          />
          <PasswordField
            id="confirmPassword"
            label="Confirmar nova senha"
            autoComplete="new-password"
            {...passwordForm.register('confirmPassword')}
            error={errorText(passwordErrors.errors.confirmPassword, 'confirmPassword')}
          />
          <Button type="submit" disabled={passwordErrors.isSubmitting} aria-busy={passwordErrors.isSubmitting}>
            Alterar senha
          </Button>
        </form>
      </section>

      <section aria-labelledby="email-heading" className="space-y-5 rounded-2xl border border-border bg-card p-5">
        <h2 id="email-heading" className="text-lg font-semibold text-foreground">
          E-mail de acesso
        </h2>
        <p className="text-base text-muted-foreground">
          O e-mail atual é {email}. O novo e-mail será usado para entrar.
        </p>
        {emailItems.length > 0 ? <ErrorSummary items={emailItems} /> : null}
        {emailOutcome ? <Notice tone={emailOutcome.tone}>{emailOutcome.text}</Notice> : null}
        <form className="space-y-6" noValidate onSubmit={submitEmail}>
          <PasswordField
            id="email-currentPassword"
            label="Senha atual"
            autoComplete="current-password"
            {...emailForm.register('currentPassword')}
            error={errorText(emailErrors.errors.currentPassword, 'currentPassword')}
          />
          <TextField
            id="email-newEmail"
            label="Novo e-mail de acesso"
            type="email"
            inputMode="email"
            autoComplete="email"
            autoCapitalize="none"
            spellCheck={false}
            {...emailForm.register('newEmail')}
            error={errorText(emailErrors.errors.newEmail, 'newEmail')}
          />
          <Button type="submit" disabled={emailErrors.isSubmitting} aria-busy={emailErrors.isSubmitting}>
            Alterar e-mail
          </Button>
        </form>
      </section>

      <section aria-labelledby="sessions-heading" className="space-y-4 rounded-2xl border border-border bg-card p-5">
        <h2 id="sessions-heading" className="text-lg font-semibold text-foreground">
          Sessões
        </h2>
        <p className="text-base text-muted-foreground">
          Encerra o acesso em outros navegadores e computadores. Esta sessão continua ativa.
        </p>
        <Button variant="outline" onClick={() => setRevokeOpen(true)}>
          Encerrar outras sessões
        </Button>
        {revokeOutcome ? <Notice tone={revokeOutcome.tone}>{revokeOutcome.text}</Notice> : null}
      </section>

      <section aria-labelledby="signout-heading" className="space-y-4 rounded-2xl border border-border bg-card p-5">
        <h2 id="signout-heading" className="text-lg font-semibold text-foreground">
          Sair
        </h2>
        <p className="text-base text-muted-foreground">
          Use esta opção ao terminar em um computador compartilhado.
        </p>
        <div>
          <LogoutButton />
        </div>
      </section>

      <ConfirmDialog
        open={revokeOpen}
        onOpenChange={setRevokeOpen}
        title="Encerrar outras sessões?"
        description="Os outros navegadores e computadores precisarão entrar de novo. Esta sessão continua ativa."
        confirmLabel="Encerrar outras sessões"
        destructive
        onConfirm={() => void revokeOthers()}
      />
    </>
  )
}
