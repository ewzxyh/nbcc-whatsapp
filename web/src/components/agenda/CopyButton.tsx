import { useEffect, useRef, useState } from 'react'
import { CheckIcon, CopyIcon } from '@solar-icons/react/linear'
import { Button } from '@/components/ui/button'
import { displayTitle, type AgendaEntry } from '@/lib/agenda'
import { copyText } from '@/lib/clipboard'
import { formatCopyText } from '@/lib/share'

type CopyState = 'idle' | 'copied' | 'failed'

const RESET_MS = 2500

export function CopyButton({ entry }: { entry: AgendaEntry }) {
  const [state, setState] = useState<CopyState>('idle')
  const timer = useRef<number | undefined>(undefined)
  const mounted = useRef(true)

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      window.clearTimeout(timer.current)
    }
  }, [])

  const onCopy = async () => {
    const ok = await copyText(formatCopyText(entry))
    // The copy can finish after the sheet or card has closed; a late update must not start a timer.
    if (!mounted.current) return
    setState(ok ? 'copied' : 'failed')
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setState('idle'), RESET_MS)
  }

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-none">
      <Button
        variant="outline"
        onClick={onCopy}
        aria-label={`Copy details for ${displayTitle(entry)}`}
        className="min-w-0 w-full sm:w-auto"
      >
        {state === 'copied' ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}
        {state === 'copied' ? 'Copied' : 'Copy details'}
      </Button>
      <p role="status" className="text-sm text-muted-foreground empty:hidden">
        {state === 'copied' ? 'Details copied to the clipboard.' : state === 'failed' ? 'Copy failed. Your browser blocked clipboard access; try again from the details panel.' : ''}
      </p>
    </div>
  )
}
