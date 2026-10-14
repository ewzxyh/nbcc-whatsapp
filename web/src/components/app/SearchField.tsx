import { useRef } from 'react'
import { CloseIcon, MagnifierIcon } from '@solar-icons/react/linear'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface SearchFieldProps {
  value: string
  onChange: (value: string) => void
}

export function SearchField({ value, onChange }: SearchFieldProps) {
  const inputRef = useRef<HTMLInputElement>(null)

  return (
    <div className="space-y-2">
      <Label htmlFor="agenda-search" className="text-base font-medium text-foreground">
        Search subjects, professors, events or locations
      </Label>
      <div className="relative">
        <MagnifierIcon
          aria-hidden="true"
          className="pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          ref={inputRef}
          id="agenda-search"
          type="search"
          autoComplete="off"
          spellCheck={false}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Escape' && value !== '') {
              event.preventDefault()
              onChange('')
            }
          }}
          className="h-11 pr-12 pl-10 text-base"
        />
        {value !== '' && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Clear search"
            className="absolute top-1/2 right-1 -translate-y-1/2"
            onClick={() => {
              onChange('')
              inputRef.current?.focus()
            }}
          >
            <CloseIcon aria-hidden="true" className="size-5" />
          </Button>
        )}
      </div>
    </div>
  )
}
