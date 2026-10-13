import { useState } from 'react'
import { Link } from 'react-router'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@/components/ui/button'
import { ErrorSummary, type ErrorSummaryItem } from '../components/error-summary'
import { AuthShell } from '../components/auth-shell'
import { Notice } from '../components/notice'
import { PageHeader } from '../components/page-header'
import { PasswordField } from '../components/password-field'
import { TextField } from '../components/text-field'
import { useDocumentTitle } from '../hooks'
import { isApiError, isSessionError } from '../lib/api'
import { messageForError } from '../lib/errors'
import { applyServerFieldErrors, errorText, summaryFromErrors } from '../lib/form-errors'
import { normalizeSetupCode } from '../lib/rules'
import { setupSchema } from '../lib/schemas'
import type { AuthResponse } from '../lib/types'
import { useSession } from '../session'

interface SetupValues {
  email: string
  setupCode: string
  password: string
  confirmPassword: string
}

const SETUP_FIELDS = ['email', 'setupCode', 'password', 'confirmPassword'] as const

export function SetupPage() {
  useDocumentTitle('Criar acesso')
  const { email, request, completeAuth } = useSession()
  const [formError, setFormError] = useState<string | null>(null)
  const [alreadyConfigured, setAlreadyConfigured] = useState(false)
  const form = useForm<SetupValues>({
    resolver: zodResolver(setupSchema),
    defaultValues: { email, setupCode: '', password: '', confirmPassword: '' },
  })
  const {
    register,
    handleSubmit,
    setError,
    resetField,
    formState: { errors, isSubmitting, submitCount },
  } = form

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null)
    try {
      const auth = await request<AuthResponse>('POST', '/setup', {
        email: values.email.trim(),
        setupCode: normalizeSetupCode(values.setupCode),
        password: values.password,
      })
      completeAuth(auth)
    } catch (error) {
      if (isSessionError(error)) return
      if (isApiError(error) && error.code === 'invalid_setup_code') {
        setError('setupCode', { type: 'server', message: 'invalid_setup_code' })
        resetField('password')
        resetField('confirmPassword')
        return
      }
      if (isApiError(error) && error.code === 'already_configured') {
        setAlreadyConfigured(true)
        return
      }
      const unmapped = applyServerFieldErrors(error, setError, SETUP_FIELDS)
      setFormError(unmapped.length > 0 ? unmapped.join(' ') : messageForError(error))
    }
  })

  const items: ErrorSummaryItem[] = [
    ...(submitCount > 0 ? summaryFromErrors(errors) : []),
    ...(formError ? [{ message: formError }] : []),
  ]

  return (
    <AuthShell>
      <PageHeader
        title="Criar acesso"
        description="O código de configuração aparece no log do serviço, uma vez por inicialização."
      />
      {alreadyConfigured ? (
        <Notice tone="error" className="mt-6">
          O acesso já foi configurado. Entre com sua senha. <Link to="/entrar" className="underline underline-offset-4">Ir para a entrada</Link>
        </Notice>
      ) : null}
      {items.length > 0 ? (
        <div className="mt-6">
          <ErrorSummary items={items} />
        </div>
      ) : null}
      <form className="mt-6 space-y-6" noValidate onSubmit={onSubmit}>
        <TextField
          id="email"
          label="E-mail de acesso"
          type="email"
          inputMode="email"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          {...register('email')}
          error={errorText(errors.email, 'email')}
        />
        <TextField
          id="setupCode"
          label="Código de configuração"
          className="uppercase"
          autoComplete="off"
          inputMode="text"
          autoCapitalize="characters"
          spellCheck={false}
          help="Use o código como aparece no log, com ou sem traços."
          {...register('setupCode')}
          error={errorText(errors.setupCode, 'setupCode')}
        />
        <PasswordField
          id="password"
          label="Nova senha"
          autoComplete="new-password"
          help="Use pelo menos 12 caracteres. Não use seu e-mail como senha."
          {...register('password')}
          error={errorText(errors.password, 'password')}
        />
        <PasswordField
          id="confirmPassword"
          label="Confirmar nova senha"
          autoComplete="new-password"
          {...register('confirmPassword')}
          error={errorText(errors.confirmPassword, 'confirmPassword')}
        />
        <Button type="submit" className="w-full" disabled={isSubmitting} aria-busy={isSubmitting}>
          Criar acesso
        </Button>
      </form>
    </AuthShell>
  )
}
