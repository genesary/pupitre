import { expect, test } from '@playwright/test';

import { ACCOUNTS, expectLoaded, signIn } from './helpers';

// Each role signs in through Keycloak and reaches its pages, served by the
// real API (docs/specs/001, 002, 030, 040-047).

test('a learner signs in and reaches the learner space', async ({ page }) => {
  await signIn(page, ACCOUNTS.learner, '/courses');
  await expectLoaded(page, 'Catalogue');
  // The demo catalogue of SEED_DEV_COURSES.
  await expect(page.getByText('Introduction à Linux').first()).toBeVisible();

  for (const [path, heading] of [
    ['/my-courses', 'Mes cours'],
    ['/my-paths', 'Mes parcours'],
    ['/my-skills', 'Mes compétences'],
    ['/my-badges', 'Mes badges'],
    ['/my-xp', 'Mon XP'],
    ['/my-groups', 'Mes groupes'],
    ['/leaderboard', 'Classement'],
    ['/profile', 'Mon compte'],
  ] as const) {
    await page.goto(path);
    await expectLoaded(page, heading);
  }
});

test('a learner cannot open the admin app', async ({ page }) => {
  await signIn(page, ACCOUNTS.learner, '/courses');
  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: 'Tableau de bord' })).toHaveCount(0);
});

test('an admin signs in through the admin group and reaches every admin page', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/admin');
  await expectLoaded(page, 'Tableau de bord');

  for (const [path, heading] of [
    ['/admin/users', 'Utilisateurs'],
    ['/admin/courses', 'Cours'],
    ['/admin/paths', 'Parcours'],
    ['/admin/groups', 'Groupes'],
    ['/admin/leaderboard', 'Classement'],
    ['/admin/labs', 'Suivi des Labs'],
    ['/admin/exports', 'Exports CSV'],
    ['/admin/patterns', 'Markdown Patterns'],
    ['/admin/settings', 'Paramètres'],
    ['/admin/monitoring', 'Monitoring'],
  ] as const) {
    await page.goto(path);
    await expectLoaded(page, heading);
  }

  await page.goto('/admin/users');
  await expect(page.getByText(ACCOUNTS.learner.email)).toBeVisible();
});

test('a manager reaches the manager space and the agenda', async ({ page }) => {
  await signIn(page, ACCOUNTS.manager, '/manager/users');
  await expectLoaded(page, 'Mes membres');

  await page.goto('/manager/groups');
  await expectLoaded(page, 'Mes groupes');

  await page.goto('/agenda');
  await expect(page.getByRole('button', { name: '+ Nouvelle session' })).toBeVisible();
});
