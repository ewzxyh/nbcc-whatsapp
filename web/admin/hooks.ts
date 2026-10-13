import { useCallback, useEffect, useRef, useState } from 'react'
import type { FieldValues, UseFormReturn } from 'react-hook-form'
import { useDrafts } from './drafts'

export interface AsyncState<T> {
  data: T | undefined
  error: unknown
  loading: boolean
  reload: () => void
  setData: (value: T) => void
}

// Runs `load` on mount and on reload. A stale response never overwrites a newer one.
export function useAsync<T>(load: () => Promise<T>): AsyncState<T> {
  const loadRef = useRef(load)
  loadRef.current = load
  const [state, setState] = useState<{ data?: T; error?: unknown; loading: boolean }>({ loading: true })
  const sequence = useRef(0)

  const run = useCallback(() => {
    sequence.current += 1
    const id = sequence.current
    setState((prev) => ({ data: prev.data, loading: true }))
    loadRef.current().then(
      (data) => {
        if (id === sequence.current) setState({ data, loading: false })
      },
      (error: unknown) => {
        if (id === sequence.current) setState((prev) => ({ data: prev.data, error, loading: false }))
      },
    )
  }, [])

  useEffect(() => {
    run()
  }, [run])

  const setData = useCallback((value: T) => setState({ data: value, loading: false }), [])

  return { data: state.data, error: state.error, loading: state.loading, reload: run, setData }
}

// Keeps unsaved edits in memory across re-login and reports whether the form differs from the saved values.
// `baseline` is the saved state. Passing a new object (after a save or reload) resets the form to it.
export function useFormDraft<T extends FieldValues>(
  key: string,
  form: UseFormReturn<T>,
  baseline: T | null,
  sanitize: (values: T) => T,
): boolean {
  const drafts = useDrafts()
  const enabled = useRef(false)
  const baselineRef = useRef<T | null>(baseline)
  baselineRef.current = baseline
  const sanitizeRef = useRef(sanitize)
  sanitizeRef.current = sanitize

  useEffect(() => {
    if (!baseline) return
    enabled.current = false
    form.reset(drafts.get<T>(key) ?? baseline)
    enabled.current = true
  }, [baseline, form, drafts, key])

  useEffect(() => {
    const subscription = form.watch((values) => {
      const base = baselineRef.current
      if (!enabled.current || !base) return
      const current = sanitizeRef.current(values as T)
      const dirty = JSON.stringify(current) !== JSON.stringify(sanitizeRef.current(base))
      drafts.set(key, dirty ? current : null)
    })
    return () => subscription.unsubscribe()
  }, [form, drafts, key])

  if (!baseline) return false
  // watch() in render subscribes this component to every field change, so `dirty` stays current.
  const current = sanitize(form.watch() as T)
  return JSON.stringify(current) !== JSON.stringify(sanitize(baseline))
}

export function useDocumentTitle(title: string) {
  useEffect(() => {
    const previous = document.title
    document.title = `${title} · Administração · Agenda NBCC`
    return () => {
      document.title = previous
    }
  }, [title])
}
