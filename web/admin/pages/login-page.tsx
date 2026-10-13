import { useState } from 'react'
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
import { isSessionError } from '../lib/api'
import { messageForError } from '../lib/errors'
import { errorText, summaryFromErrors } from '../lib/form-errors'
import { loginSchema } from '../lib/schemas'
import type { AuthResponse } from '../lib/types'
import { useSession } from '../session'

interface LoginValues {
  email: string
  password: string
}

export function LoginPage() {
  useDocumentTitle('Entrar')
  const { email, notice, request, completeAuth } = useSession()
  const [formError, setFormError] = useState<string | null>(null)
  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email, password: '' },
  })
  const {
    register,
    handleSubmit,
    resetField,
    formState: { errors, isSubmitting, submitCount },
  } = form

  const onSubmit = handleSubmit(async (values) => {
    setFormError(null)
    try {
      const auth = await request<AuthResponse>('POST', '/login', {
        email: values.email.trim(),
        password: values.password,
      })
      completeAuth(auth)
    } catch (error) {
      if (isSessionError(error)) return
      resetField('password')
      setFormError(messageForError(error))
    }
  })

  const items: ErrorSummaryItem[] = [
    ...(submitCount > 0 ? summaryFromErrors(errors) : []),
    ...(formError ? [{ message: formError }] : []),
  ]

  return (
    <AuthShell>
      <PageHeader title="Entrar" description="Use o e-mail e a senha da administração." />
      {notice ? (
        <Notice tone="info" className="mt-6">
          {notice}
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
        <PasswordField
          id="password"
          label="Senha"
          autoComplete="current-password"
          {...register('password')}
          error={errorText(errors.password, 'password')}
        />
        <Button type="submit" className="w-full" disabled={isSubmitting} aria-busy={isSubmitting}>
          Entrar
        </Button>
      </form>
    </AuthShell>
  )
}
