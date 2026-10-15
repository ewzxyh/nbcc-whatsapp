# NBCC Agenda (web)

Student-facing agenda UI for NBCC. It reads `GET /api/agenda` from the Go service and shows
course work and campus events in four views: List, Calendar, Subjects and Board. Times are shown in
Saint John, NB (America/Moncton).

Stack: Vite, React, TypeScript, Tailwind CSS v4, shadcn/ui (Radix), Figtree (self-hosted), Vitest.

## Scripts

```sh
npm ci            # install from package-lock.json
npm run dev       # Vite dev server on http://localhost:5173
npm test          # unit tests (pure logic, palette contrast, no shadows in ui/)
npm run typecheck # tsc -b
npm run build     # typecheck + production build into dist/
```

## Local development

`npm run dev` proxies `/api` to `http://127.0.0.1:8080`, where the Go service runs locally.
Start the backend first, or the app shows the "Agenda unavailable" state.

The production build (`dist/`) is served by the Go service at `/`. Assets are hashed under `/assets/`.

## Administração (`/admin`)

Área de administração em português (pt-BR), em `web/admin/`. É uma segunda entrada do Vite, separada do
site público, e não há nenhum link para ela no site público.

- Desenvolvimento: `npm run dev` e abra `http://localhost:5173/admin`. O Vite reescreve as rotas
  `/admin/*` (navegações HTML) para `admin/index.html`. O proxy de `/api` aponta para
  `VITE_API_PROXY`, ou para `http://127.0.0.1:8080` se a variável não estiver definida.
- Build: `npm run build` gera `dist/index.html` (público) e `dist/admin/index.html` (admin). O serviço Go
  entrega `dist/admin/index.html` para `/admin` e `/admin/*`.
- Testes: `npm test` inclui `web/admin/**/*.test.ts` (mensagens de erro, regras, datas, temporizador de sessão).
- O código não guarda senhas, segredos nem tokens em armazenamento do navegador. A sessão e o token CSRF
  ficam só na memória da página.

## Notes

- Activities whose title mentions LockDown Browser have red assessment cards, a separate browser-requirement badge, and the supplied chimp illustration. They show an **Assessment / Test** badge instead of progress controls.
- For these activities, the public UI, copied details and WhatsApp agendas deliberately label the feed's **Due** time as **Starts**, as requested. This does not infer an opening time or change the feed, API phases, or the instant used to schedule deadline reminders.
- Today's activity cards use a transparent monkey illustration, switching to the startled monkey at four hours before Due. The existing minute clock updates the image; campus events are excluded. Non-today LockDown assessments keep the chimp illustration.
- Tests and fixtures use synthetic data only (`src/test/fixtures.ts`). No real calendar data is committed.
- Light theme only. Design tokens are CSS variables in `src/index.css`.
- `src/components/ui` holds shadcn/ui components with shadows removed; keep it shadow-free (a test checks this).
