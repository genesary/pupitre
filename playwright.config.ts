import { defineConfig, devices } from '@playwright/test';

// End-to-end tests of the main flows (docs/specs/099-migration-plan.md, batch 5),
// run against the mock mode (docs/specs/006-mock-mode.md): no backend, no
// Keycloak. Each test gets a fresh browser context, so the mock state starts
// from its seed.

const PORT = 4173;

export default defineConfig({
  testDir: './e2e',
  // e2e/backend runs against the real backend: playwright.backend.config.ts.
  testIgnore: 'backend/**',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  // The dev server compiles pages on first request: generous timeouts, few workers.
  timeout: 60_000,
  expect: { timeout: 15_000 },
  workers: process.env.CI ? 1 : 2,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://localhost:${PORT}`,
    locale: 'fr-FR',
    timezoneId: 'Europe/Paris',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  // Both apps in mock mode; the learner app's server forwards /admin and /manager.
  webServer: {
    command: 'pnpm dev:mock',
    env: { PUPITRE_APP_PORT: String(PORT), PUPITRE_ADMIN_PORT: String(PORT + 1) },
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
