import { expect, type APIRequestContext, type Page } from '@playwright/test';

import { skipTour } from '../helpers';

// Shared steps of the tests against the real backend (playwright.backend.config.ts).

/** Accounts of the Keycloak test realm (apps/api/infra/keycloak/pupitre-realm.json). */
export const ACCOUNTS = {
  learner: { username: 'alice', password: 'alice-test-pw', email: 'alice@pupitre.local' },
  manager: { username: 'marc.manager', password: 'marc-test-pw', email: 'marc.manager@pupitre.local' },
  admin: { username: 'admin.kc', password: 'admin-test-pw', email: 'admin.kc@pupitre.local' },
} as const;

export type Account = (typeof ACCOUNTS)[keyof typeof ACCOUNTS];

/** The API's bootstrap admin, a local account (ADMIN_PASSWORD of apps/api/.env). */
export const LOCAL_ADMIN = {
  email: 'admin@pupitre.local',
  password: process.env.PUPITRE_ADMIN_PASSWORD ?? 'Local-Admin-Dev-1',
};

const API = process.env.PUPITRE_API_URL ?? 'http://localhost:8080';

const escape = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/** Signs in through the Keycloak page and waits to land on `path`. */
export async function signIn(page: Page, account: Account, path = '/my-courses') {
  await skipTour(page);
  await page.goto(`/login?redirect=${encodeURIComponent(path)}`);
  await page.getByRole('button', { name: 'Se connecter avec Keycloak' }).click();
  await page.locator('#username').fill(account.username);
  await page.locator('#password').fill(account.password);
  await page.locator('#kc-login').click();
  await expect(page).toHaveURL(new RegExp(`${escape(path)}$`));
}

let cachedAdminToken: string | undefined;

/**
 * A token of the local admin account, for setting data up through the API.
 * Signed in once per worker: the sign-in routes allow 10 calls a minute per
 * address (AUTH_RATE_LIMIT_REQUESTS), Keycloak sign-ins included.
 */
export async function adminToken(request: APIRequestContext): Promise<string> {
  if (cachedAdminToken) return cachedAdminToken;
  const response = await request.post(`${API}/api/auth/login`, {
    data: { email: LOCAL_ADMIN.email, password: LOCAL_ADMIN.password },
  });
  expect(response.ok(), `local admin login: ${response.status()}`).toBeTruthy();
  cachedAdminToken = ((await response.json()) as { token: string }).token;
  return cachedAdminToken;
}

/** Calls the API as the local admin. */
export async function adminCall(request: APIRequestContext, method: 'GET' | 'POST' | 'PUT', path: string, data?: unknown) {
  const token = await adminToken(request);
  return request.fetch(`${API}/api${path}`, { method, data, headers: { Authorization: `Bearer ${token}` } });
}

/** Fails if the page shows its load-error state. */
export async function expectLoaded(page: Page, heading: string | RegExp) {
  await expect(page.getByRole('heading', { name: heading }).first()).toBeVisible();
  await expect(page.getByText('Impossible de charger la page')).toHaveCount(0);
}
