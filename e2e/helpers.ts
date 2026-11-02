import { expect, type Page } from '@playwright/test';

// Shared steps of the end-to-end tests. Sign-in goes through the mock Keycloak
// page (docs/specs/006-mock-mode.md), so the real login and callback code runs,
// state check included. The mock state lives in the tab's sessionStorage: it
// is kept across a sign-out and a sign-in as someone else in the same test.

export const ACCOUNTS = {
  alice: 'alice',
  manager: 'marc.manager',
  admin: 'admin',
  trap: '<img src=x onerror=alert("xss-username")>',
} as const;

export const TRAP_ID = '9c8b7a65-4d3e-4f21-b0a9-8e7d6c5b4a39';

const escape = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/**
 * Marks the guided tour and the first lesson tip as seen for every account
 * (docs/specs/007-guided-tour.md), so they do not cover the pages under test.
 */
export async function skipTour(page: Page) {
  await page.addInitScript(() => {
    const getItem = Storage.prototype.getItem;
    Storage.prototype.getItem = function (key: string) {
      if (key.startsWith('pupitre.tour.')) return JSON.stringify({ version: 999, status: 'completed', lessonTip: 'seen' });
      return getItem.call(this, key);
    };
  });
}

/**
 * Signs in as `username` from the sign-in page, and waits to land on `path`.
 * The guided tour stays out of the way unless `tour` is set.
 */
export async function signIn(page: Page, username: string, path = '/my-courses', { tour = false } = {}) {
  if (!tour) await skipTour(page);
  await page.goto(`/login?redirect=${encodeURIComponent(path)}`);
  await page.getByRole('button', { name: 'Se connecter avec Keycloak' }).click();
  await expect(page.getByRole('heading', { name: /Keycloak simulé/ })).toBeVisible();
  // Each account button is named « <username> <description> ».
  await page.getByRole('button', { name: new RegExp(`^${escape(username)} `) }).click();
  await expect(page).toHaveURL(new RegExp(`${escape(path)}$`));
}

export async function signOut(page: Page) {
  await page.getByRole('button', { name: 'Déconnexion' }).click();
  await expect(page).toHaveURL(/\/login/);
}

/** Fails the test if any alert, confirm or prompt opens: an XSS payload ran. */
export function forbidDialogs(page: Page) {
  page.on('dialog', async (dialog) => {
    await dialog.dismiss();
    throw new Error(`Unexpected dialog: ${dialog.message()}`);
  });
}

type NewModule = { name: string; type?: string; content: string; advanced?: object };

/** As an admin: creates a public course with one module, from /admin/courses. */
export async function createCourse(page: Page, slug: string, title: string, module: NewModule) {
  await page.goto('/admin/courses');
  await page.getByRole('button', { name: '+ Nouveau cours' }).click();
  await page.getByLabel('Slug').fill(slug);
  await page.getByLabel(/^Titre/).fill(title);
  await page.getByLabel(/Public/).check();
  await page.getByRole('button', { name: 'Créer le cours' }).click();
  await expect(page.getByText('Cours créé')).toBeVisible();

  const course = page.locator('.course-item', { hasText: slug });
  await course.getByRole('button', { name: '+ Ajouter' }).click();
  await course.getByLabel('Nom *').fill(module.name);
  if (module.type) await course.getByLabel('Type').selectOption(module.type);
  await course.getByLabel('Contenu (markdown)').fill(module.content);
  if (module.advanced) {
    await course.getByText('Champs avancés (JSON)').click();
    await course.getByRole('textbox', { name: 'Champs avancés (JSON)' }).fill(JSON.stringify(module.advanced));
  }
  await course.getByRole('button', { name: 'Ajouter', exact: true }).click();
  await expect(page.getByText('Module ajouté')).toBeVisible();
}

/** A datetime-local value `days` from now at `hour`:00, in the browser's time zone. */
export function inDays(days: number, hour: number): string {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(hour)}:00`;
}
