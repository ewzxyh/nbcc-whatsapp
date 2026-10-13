import { NavLink, Outlet } from 'react-router'
import { cn } from '@/lib/utils'
import { LogoutButton } from '../components/logout-button'
import { useSession } from '../session'

const NAV_ITEMS = [
  { to: '/', label: 'Visão geral', end: true },
  { to: '/agenda', label: 'Agenda pública', end: false },
  { to: '/whatsapp', label: 'WhatsApp e Grapnel', end: false },
  { to: '/professores', label: 'Professores', end: false },
  { to: '/seguranca', label: 'Segurança', end: false },
  { to: '/atividade', label: 'Atividade', end: false },
]

// Sidebar from lg up (not sticky). Below lg the links wrap into rows. Nothing scrolls horizontally.
export function AdminLayout() {
  const { email } = useSession()
  return (
    <div className="min-h-dvh bg-background lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-card px-4 py-3 lg:col-span-2 lg:px-8">
        <div className="min-w-0">
          <p className="text-base font-semibold text-foreground">Agenda NBCC · Administração</p>
          <p className="text-sm break-words text-muted-foreground">Entrou como {email}</p>
        </div>
        <LogoutButton />
      </header>
      <nav aria-label="Principal" className="border-b border-border bg-surface px-4 py-3 lg:border-r lg:border-b-0 lg:py-6">
        <ul className="flex flex-wrap gap-2 lg:flex-col lg:gap-1">
          {NAV_ITEMS.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  cn(
                    'inline-flex min-h-11 items-center rounded-xl px-3 text-base text-foreground hover:bg-muted',
                    isActive && 'bg-accent font-semibold text-accent-foreground hover:bg-accent',
                  )
                }
              >
                {item.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      <main
        id="conteudo"
        tabIndex={-1}
        className="mx-auto w-full min-w-0 max-w-3xl space-y-8 px-4 py-8 lg:px-8"
      >
        <Outlet />
      </main>
    </div>
  )
}
