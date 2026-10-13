import { createContext, useContext, useState, type ReactNode } from 'react'

// Unsaved form values, kept in memory only so a re-login can continue the same edit.
// Nothing here is written to browser storage. Secret values are removed before a draft is stored.
export interface DraftStore {
  get<T>(key: string): T | undefined
  set(key: string, value: unknown | null): void
  clear(key: string): void
  clearAll(): void
  hasAny(): boolean
}

function createDraftStore(): DraftStore {
  const drafts = new Map<string, unknown>()
  return {
    get<T>(key: string) {
      return drafts.get(key) as T | undefined
    },
    set(key, value) {
      if (value === null || value === undefined) drafts.delete(key)
      else drafts.set(key, value)
    },
    clear(key) {
      drafts.delete(key)
    },
    clearAll() {
      drafts.clear()
    },
    hasAny() {
      return drafts.size > 0
    },
  }
}

const DraftContext = createContext<DraftStore | null>(null)

export function DraftProvider({ children }: { children: ReactNode }) {
  const [store] = useState(createDraftStore)
  return <DraftContext.Provider value={store}>{children}</DraftContext.Provider>
}

export function useDrafts(): DraftStore {
  const store = useContext(DraftContext)
  if (!store) throw new Error('useDrafts must be used inside DraftProvider')
  return store
}
