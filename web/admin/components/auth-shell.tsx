import type { ReactNode } from 'react'

// Centered layout for sign-in, setup, loading and the 404 page.
export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <main
      id="conteudo"
      tabIndex={-1}
      className="mx-auto flex min-h-dvh w-full max-w-md min-w-0 flex-col justify-center px-4 py-10"
    >
      <p className="mb-6 text-base font-semibold text-muted-foreground">Agenda NBCC · Administração</p>
      <div className="min-w-0 rounded-2xl border border-border bg-card p-6 sm:p-8">{children}</div>
    </main>
  )
}
