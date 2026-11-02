/// <reference types="vitest/config" />
import { rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import type { ClientRequest } from 'node:http';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv, type Plugin, type ProxyOptions } from 'vite';

// The project root. Vite rewrites import.meta.url when it bundles this config,
// but not import.meta.dirname, and the dev server may run from another folder:
// every path is built from here, never from process.cwd().
const ROOT = fileURLToPath(new URL('.', import.meta.url));

// Only these request headers reach the backend. The Astro proxy forwarded every
// client header, X-Forwarded-For and X-Internal-Secret included (audit H4).
const FORWARDED_HEADERS = new Set([
  'accept',
  'authorization',
  'content-type',
  'content-length',
  // The OAuth state cookie, the API's only cookie (docs/specs/190-backend-defects.md B13).
  'cookie',
  'host',
]);

function stripHeaders(proxyReq: ClientRequest) {
  for (const name of proxyReq.getHeaderNames()) {
    if (!FORWARDED_HEADERS.has(name.toLowerCase())) proxyReq.removeHeader(name);
  }
}

// public/mockServiceWorker.js is only for `pnpm dev:mock`: keep it out of a
// production build.
function dropMockWorker(mode: string): Plugin {
  return {
    name: 'pupitre:drop-mock-worker',
    apply: 'build',
    closeBundle() {
      if (mode !== 'mock') rmSync(new URL('./dist/mockServiceWorker.js', import.meta.url), { force: true });
    },
  };
}

// The admin app (apps/admin) is served on the same origin under /admin and
// /manager, so the session in sessionStorage is shared: in development this
// server forwards those paths to the admin dev server. /manager/* gets the
// admin app's page, whose router reads the real URL.
function adminApp(port: number): Record<string, ProxyOptions> {
  const admin = { target: `http://localhost:${port}`, changeOrigin: true, ws: true };
  return {
    // The admin app's base is /admin/: /admin alone gets the trailing slash.
    '^/admin(/|\\?|$)': { ...admin, rewrite: (path) => path.replace(/^\/admin(?=$|\?)/, '/admin/') },
    '^/manager(/|$)': { ...admin, rewrite: () => '/admin/' },
  };
}

function target(url: string): ProxyOptions {
  return {
    target: url,
    changeOrigin: true,
    configure: (proxy) => proxy.on('proxyReq', stripHeaders),
  };
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ROOT, 'VITE_');
  // The backend is one application (docs/specs/004-api-and-routing.md): /api
  // and /uploads go to the same target.
  const api = env.VITE_API_URL || 'http://localhost:8080';

  // Ports can be moved, e.g. by the end-to-end tests (playwright.config.ts).
  const port = Number(process.env.PUPITRE_APP_PORT || 3000);
  const adminPort = Number(process.env.PUPITRE_ADMIN_PORT || 3001);

  return {
    plugins: [react(), tailwindcss(), dropMockWorker(mode)],
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
    server: {
      port,
      strictPort: true,
      // In mock mode MSW answers /api in the browser, so only the admin app is proxied.
      proxy:
        mode === 'mock'
          ? adminApp(adminPort)
          : {
              ...adminApp(adminPort),
              '/uploads': target(api),
              '/api': target(api),
            },
    },
    test: {
      environment: 'jsdom',
      globals: true,
      setupFiles: ['./src/test/setup.ts'],
      css: false,
    },
  };
});
