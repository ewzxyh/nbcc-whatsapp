import { createBrowserRouter } from 'react-router'
import { AdminLayout } from './layout/admin-layout'
import { GuestOnly, RequireAuth, SetupOnly } from './guards'
import { ActivityPage } from './pages/activity-page'
import { AgendaPage } from './pages/agenda-page'
import { CoursesPage } from './pages/courses-page'
import { LoginPage } from './pages/login-page'
import { NotFoundPage } from './pages/not-found-page'
import { OverviewPage } from './pages/overview-page'
import { SecurityPage } from './pages/security-page'
import { SetupPage } from './pages/setup-page'
import { WhatsAppPage } from './pages/whatsapp-page'
import { AdminRoot } from './root'

// basename /admin: the Go service serves admin/index.html for /admin and /admin/*.
export const router = createBrowserRouter(
  [
    {
      path: '/',
      element: <AdminRoot />,
      children: [
        {
          path: 'entrar',
          element: (
            <GuestOnly>
              <LoginPage />
            </GuestOnly>
          ),
        },
        {
          path: 'configurar',
          element: (
            <SetupOnly>
              <SetupPage />
            </SetupOnly>
          ),
        },
        {
          element: (
            <RequireAuth>
              <AdminLayout />
            </RequireAuth>
          ),
          children: [
            { index: true, element: <OverviewPage /> },
            { path: 'agenda', element: <AgendaPage /> },
            { path: 'whatsapp', element: <WhatsAppPage /> },
            { path: 'professores', element: <CoursesPage /> },
            { path: 'seguranca', element: <SecurityPage /> },
            { path: 'atividade', element: <ActivityPage /> },
          ],
        },
        { path: '*', element: <NotFoundPage /> },
      ],
    },
  ],
  { basename: '/admin' },
)
