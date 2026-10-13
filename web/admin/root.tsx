import { Outlet } from 'react-router'
import { DraftProvider } from './drafts'
import { SessionProvider } from './session'

// Top-level element for every /admin route. The skip link is the first focusable element on each page.
export function AdminRoot() {
  return (
    <DraftProvider>
      <SessionProvider>
        <a
          href="#conteudo"
          className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:inline-flex focus:min-h-11 focus:items-center focus:rounded-xl focus:bg-primary focus:px-4 focus:text-base focus:font-medium focus:text-primary-foreground"
        >
          Pular para o conteúdo
        </a>
        <Outlet />
      </SessionProvider>
    </DraftProvider>
  )
}
