import { expect, test } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// The frontend is two applications on one origin (docs/specs/000-scope-and-stack.md):
// the learner app, and the admin app under /admin and /manager.

test('a learner browsing the learner app downloads nothing from the admin app', async ({ page }) => {
  const adminRequests: string[] = [];
  page.on('request', (request) => {
    const { pathname } = new URL(request.url());
    if (pathname.startsWith('/admin/') || pathname.startsWith('/manager/')) adminRequests.push(pathname);
  });

  await signIn(page, ACCOUNTS.alice, '/my-courses');
  for (const path of ['/courses', '/courses/kubernetes-basics', '/my-paths', '/agenda', '/profile']) {
    await page.goto(path);
    await expect(page.locator('.topnav')).toBeVisible();
  }
  expect(adminRequests).toEqual([]);
});

test('an admin goes from one app to the other and keeps the session', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/courses');
  await page.getByRole('link', { name: '⚙ Admin' }).click();
  await expect(page).toHaveURL('/admin');
  await expect(page.getByRole('heading', { name: 'Tableau de bord' })).toBeVisible();
  await expect(page.locator('.topnav')).toContainText('admin');

  await page.getByRole('link', { name: 'Retour au catalogue' }).click();
  await expect(page).toHaveURL('/courses');
  await expect(page.getByRole('heading', { name: 'Catalogue' })).toBeVisible();
});

test('a manager reaches their space from the top bar, and an admin is refused there', async ({ page }) => {
  await signIn(page, ACCOUNTS.manager, '/courses');
  await page.getByRole('link', { name: 'Mon espace' }).click();
  await expect(page).toHaveURL('/manager/users');
  await expect(page.getByRole('heading', { name: 'Mes membres' })).toBeVisible();

  await page.getByRole('button', { name: 'Déconnexion' }).click();
  await expect(page).toHaveURL(/\/login/);
  await signIn(page, ACCOUNTS.admin, '/manager/users');
  await expect(page.getByRole('heading', { name: 'Accès refusé' })).toBeVisible();
});
