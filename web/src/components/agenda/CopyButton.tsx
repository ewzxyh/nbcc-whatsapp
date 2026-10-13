import { useEffect, useRef, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { AgendaEntry } from '@/lib/agenda'
import { copyText } from '@/lib/clipboard'
import { formatCopyText } from '@/lib/share'

type CopyState = 'idle' | 'copied' | 'failed'

const RESET_MS = 2500

export function CopyButton({ entry }: { entry: AgendaEntry }) {
  const [state, setState] = useState<CopyState>('idle')
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const onCopy = async () => {
    const ok = await copyText(formatCopyText(entry))
    setState(ok ? 'copied' : 'failed')
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setState('idle'), RESET_MS)
  }

  return (
    <>
      <Button variant="outline" onClick={onCopy} className="min-w-0 flex-1 sm:flex-none">
        {state === 'copied' ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
        {state === 'copied' ? 'Copied' : 'Copy details'}
      </Button>
      <span role="status" className="sr-only">
        {state === 'copied' ? 'Details copied to the clipboard.' : state === 'failed' ? 'Copy failed. Select the text and copy it manually.' : ''}
      </span>
    </>
  )
}
