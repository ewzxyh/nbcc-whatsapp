import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

const apiProxyTarget = process.env.VITE_API_PROXY ?? 'http://127.0.0.1:8080'

// Dev only: client routes under /admin (HTML navigations without a file extension) are served by the
// admin entry, the same way the Go service serves dist/admin/index.html for /admin and /admin/*.
function adminSpaFallback(): Plugin {
  return {
    name: 'admin-spa-fallback',
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        if (req.method !== 'GET' && req.method !== 'HEAD') return next()
        const url = req.url ?? ''
        const queryAt = url.indexOf('?')
        const pathname = queryAt === -1 ? url : url.slice(0, queryAt)
        const query = queryAt === -1 ? '' : url.slice(queryAt)
        if (pathname !== '/admin' && !pathname.startsWith('/admin/')) return next()
        if (pathname.startsWith('/admin/assets/')) return next()
        if (/\.[A-Za-z0-9]+$/.test(pathname)) return next()
        if (!(req.headers.accept ?? '').includes('text/html')) return next()
        req.url = `/admin/index.html${query}`
        next()
      })
    },
  }
}

export default defineConfig({
  base: '/',
  plugins: [react(), tailwindcss(), adminSpaFallback()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': apiProxyTarget,
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL('./index.html', import.meta.url)),
        admin: fileURLToPath(new URL('./admin/index.html', import.meta.url)),
      },
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts', 'admin/**/*.test.ts'],
  },
})
