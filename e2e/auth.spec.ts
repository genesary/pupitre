import { expect, test } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// Sign-in, guards and sign-out (docs/specs/001-keycloak-authentication.md, 002-roles-and-guards.md).

test('a visitor is sent to the sign-in page, then back where they were going', async ({ page }) => {
  await page.goto('/my-courses');
  await expect(page).toHaveURL('/login?redirect=%2Fmy-courses');

  await signIn(page, ACCOUNTS.alice, '/my-courses');
  await expect(page.getByRole('heading', { name: 'Mes cours' })).toBeVisible();
  await expect(page.locator('.topnav')).toContainText('alice');

  await page.getByRole('button', { name: 'Déconnexion' }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.goto('/my-courses');
  await expect(page).toHaveURL('/login?redirect=%2Fmy-courses');
});

test('a learner gets « Accès refusé » on an admin page', async ({ page }) => {
  await signIn(page, ACCOUNTS.alice, '/my-courses');
  await page.goto('/admin/users');
  await expect(page.getByRole('heading', { name: 'Accès refusé' })).toBeVisible();
});

test('a manager reaches the exports but not the users', async ({ page }) => {
  await signIn(page, ACCOUNTS.manager, '/admin/exports');
  await expect(page.getByRole('heading', { name: 'Exports CSV' })).toBeVisible();
  await page.goto('/admin/users');
  await expect(page.getByRole('heading', { name: 'Accès refusé' })).toBeVisible();
});
