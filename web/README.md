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

## Notes

- Tests and fixtures use synthetic data only (`src/test/fixtures.ts`). No real calendar data is committed.
- Light theme only. Design tokens are CSS variables in `src/index.css`.
- `src/components/ui` holds shadcn/ui components with shadows removed; keep it shadow-free (a test checks this).
