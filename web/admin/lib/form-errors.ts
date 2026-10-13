import type { FieldValues, Path, UseFormSetError } from 'react-hook-form'
import { isApiError } from './api'
import { messageForCode, messageForError } from './errors'
import { fieldLabel } from './labels'

export interface FieldIssue {
  path: string
  message: string
}

export interface SummaryItem {
  id: string
  message: string
  label: string
}

// React Hook Form paths use dots (entries.0.instructor). The input ids use dashes.
export function fieldIdFor(path: string): string {
  return path.replace(/\./g, '-')
}

function leafOf(path: string): string {
  const parts = path.split('.')
  return parts[parts.length - 1]
}

// Walks react-hook-form's error tree. Each leaf holds an error CODE in `message`.
export function flattenFieldErrors(errors: unknown, prefix = ''): FieldIssue[] {
  const issues: FieldIssue[] = []
  if (!errors || typeof errors !== 'object') return issues
  for (const [key, value] of Object.entries(errors)) {
    if (!value || typeof value !== 'object') continue
    const path = prefix === '' ? key : `${prefix}.${key}`
    const message = (value as { message?: unknown }).message
    if (typeof message === 'string' && message !== '') {
      issues.push({ path, message: messageForCode(message, leafOf(path)) })
    } else {
      issues.push(...flattenFieldErrors(value, path))
    }
  }
  return issues
}

// idPrefix is for pages with two forms that share field names (the input ids must stay unique).
export function summaryFromErrors(
  errors: unknown,
  labelFor: (path: string) => string = (path) => fieldLabel(leafOf(path)),
  idPrefix = '',
): SummaryItem[] {
  return flattenFieldErrors(errors).map((issue) => ({
    id: `${idPrefix}${fieldIdFor(issue.path)}`,
    message: issue.message,
    label: labelFor(issue.path),
  }))
}

// Text for one field's error, or null. The field name selects field-specific copy.
export function errorText(error: { message?: string } | undefined, field: string): string | null {
  if (!error || typeof error.message !== 'string' || error.message === '') return null
  return messageForCode(error.message, field)
}

// Handles a failed save. Field codes go onto their inputs. Anything else becomes one form-level message,
// or null when every problem is already shown on a field.
export function formMessageForSaveError<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  knownFields: readonly string[],
): string | null {
  const hasFieldErrors = isApiError(error) && Object.keys(error.fields).length > 0
  const unmapped = applyServerFieldErrors(error, setError, knownFields)
  if (unmapped.length > 0) return unmapped.join(' ')
  if (hasFieldErrors) return null
  return messageForError(error)
}

// Maps a 422 `fields` object onto react-hook-form errors. Returns messages for fields the form does not show.
export function applyServerFieldErrors<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  knownFields: readonly string[],
): string[] {
  const unmapped: string[] = []
  if (!isApiError(error)) return unmapped
  for (const [field, code] of Object.entries(error.fields)) {
    if (knownFields.includes(field)) {
      setError(field as Path<T>, { type: 'server', message: code })
    } else {
      unmapped.push(messageForCode(code, field))
    }
  }
  return unmapped
}
