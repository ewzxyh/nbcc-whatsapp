import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@/components/ui/button'
import { useDrafts } from '../drafts'
import { ErrorSummary, type ErrorSummaryItem } from '../components/error-summary'
import { LoadErrorState, LoadingState } from '../components/load-state'
import { Notice } from '../components/notice'
import { PageHeader } from '../components/page-header'
import { TextField } from '../components/text-field'
import { UnsavedChangesGuard } from '../components/unsaved-changes-guard'
import { useAsync, useDocumentTitle, useFormDraft } from '../hooks'
import { isSessionError } from '../lib/api'
import { messageForError } from '../lib/errors'
import { errorText, formMessageForSaveError, summaryFromErrors } from '../lib/form-errors'
import { fieldLabel } from '../lib/labels'
import {
  buildCourseInstructors,
  buildInstructorPatch,
  instructorRowsFrom,
  type InstructorFormValues,
} from '../lib/instructors'
import { instructorFormSchema } from '../lib/schemas'
import { useAdminApi } from '../use-admin-api'

const DRAFT_KEY = 'professores'

const keepDraft = (values: InstructorFormValues) => values

function labelForPath(path: string, codes: string[]): string {
  const match = /^entries\.(\d+)\.instructor$/.exec(path)
  if (!match) return fieldLabel(path.split('.').pop() ?? path)
  const code = codes[Number(match[1])] ?? ''
  return code ? `Professor de ${code}` : 'Professor'
}

export function CoursesPage() {
  useDocumentTitle('Professores por disciplina')
  const api = useAdminApi()
  const drafts = useDrafts()
  const loaded = useAsync(async () => {
    const [courses, settings] = await Promise.all([api.courses(), api.settings()])
    return { courses, savedMap: settings.courseInstructors }
  })
  const data = loaded.data
  const baseline = useMemo<InstructorFormValues | null>(
    () => (data ? { entries: instructorRowsFrom(data.courses, data.savedMap) } : null),
    [data],
  )
  const form = useForm<InstructorFormValues>({
    resolver: zodResolver(instructorFormSchema),
    defaultValues: { entries: [] },
  })
  const dirty = useFormDraft(DRAFT_KEY, form, baseline, keepDraft)
  const { register, handleSubmit, watch, formState, setError } = form
  const { errors, isSubmitting, submitCount } = formState
  const watched = watch('entries')

  const [formError, setFormError] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)

  const rows = baseline?.entries ?? []
  const codes = rows.map((row) => row.code)
  const knownCodes = new Set(data?.courses.map((course) => course.code) ?? [])

  const onSubmit = handleSubmit(async (values) => {
    setSaved(null)
    setFormError(null)
    if (!baseline) return
    const before = JSON.stringify(buildCourseInstructors(baseline.entries))
    if (JSON.stringify(buildCourseInstructors(values.entries)) === before) {
      setSaved('Nenhuma alteração para salvar.')
      return
    }
    try {
      await api.saveSettings(buildInstructorPatch(values.entries))
      drafts.clear(DRAFT_KEY)
      setSaved('Professores salvos.')
      loaded.reload()
    } catch (error) {
      if (isSessionError(error)) return
      setFormError(formMessageForSaveError(error, setError, []))
    }
  })

  const items: ErrorSummaryItem[] = [
    ...(submitCount > 0 ? summaryFromErrors(errors, (path) => labelForPath(path, codes)) : []),
    ...(formError ? [{ message: formError }] : []),
  ]

  const header = (
    <PageHeader
      title="Professores por disciplina"
      description="Os nomes aparecem na agenda pública e nos filtros. O calendário NBCC não informa professores, então preencha aqui."
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

  return (
    <>
      {header}
      <UnsavedChangesGuard dirty={dirty} onDiscard={() => drafts.clear(DRAFT_KEY)} />
      {rows.length === 0 ? (
        <Notice tone="info">Nenhuma disciplina encontrada na agenda atual.</Notice>
      ) : (
        <form className="space-y-6" noValidate onSubmit={onSubmit}>
          {items.length > 0 ? <ErrorSummary items={items} /> : null}
          {saved ? <Notice tone="success">{saved}</Notice> : null}
          {rows.map((row, index) => {
            const current = watched?.[index]?.instructor?.trim() ?? ''
            const title = row.title ? `${row.code} — ${row.title}` : row.code
            return (
              <fieldset key={row.code} className="space-y-4 rounded-2xl border border-border bg-card p-5">
                <legend className="px-1 text-base font-semibold text-foreground">{title}</legend>
                {knownCodes.has(row.code) ? null : (
                  <p className="text-sm text-muted-foreground">Esta disciplina não aparece na agenda atual.</p>
                )}
                <TextField
                  id={`entries-${index}-instructor`}
                  label="Professor"
                  autoComplete="off"
                  {...register(`entries.${index}.instructor`)}
                  error={errorText(errors.entries?.[index]?.instructor, `entries.${index}.instructor`)}
                />
                {current === '' ? (
                  <p className="text-sm font-medium text-muted-foreground">Sem professor informado</p>
                ) : null}
              </fieldset>
            )
          })}
          <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
            Salvar alterações
          </Button>
        </form>
      )}
    </>
  )
}
