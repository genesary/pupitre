import { defineConfig } from 'astro/config';
import node from '@astrojs/node';
import tailwindcss from '@tailwindcss/vite';

// Dev-only proxy targets (npm run dev). In production the Astro SSR server
// runs standalone and services are reached via process.env.* in server routes.
// Override with env vars for non-Kind setups (e.g. USER_SERVICE_URL=http://user-service:8081).
const USER_SERVICE_URL   = process.env.USER_SERVICE_URL   ?? 'http://localhost:8081';
const COURSE_SERVICE_URL = process.env.COURSE_SERVICE_URL ?? 'http://localhost:8082';

export default defineConfig({
  output: 'server',
  adapter: node({
    mode: 'standalone',
  }),
  devToolbar: { enabled: false },
  vite: {
    plugins: [tailwindcss()],
    server: {
      proxy: {
        '/api/admin/exports/lab-checks': {
          target: COURSE_SERVICE_URL,
          changeOrigin: true,
        },
        '/api': {
          target: USER_SERVICE_URL,
          changeOrigin: true,
        },
        '/uploads': {
          target: USER_SERVICE_URL,
          changeOrigin: true,
        },
      },
    },
  },
});
