/// <reference types="vitest/config" />
import { fileURLToPath } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The admin app (docs/specs/000-scope-and-stack.md): the administration and the
// manager space. It is served under /admin/ on the learner app's origin, which
// routes /admin/* and /manager/* to it (apps/app/vite.config.ts in development,
// the reverse proxy in production, docs/specs/004-api-and-routing.md). Open it
// through the learner app's dev server, not on its own port.

export default defineConfig(() => ({
  base: '/admin/',
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: Number(process.env.PUPITRE_ADMIN_PORT || 3001),
    strictPort: true,
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
}));
