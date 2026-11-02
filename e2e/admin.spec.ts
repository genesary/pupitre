import { expect, test } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// An admin creates a course with a module; learners then see it (docs/specs/042-admin-courses.md).

test('an admin creates a public course with a module, shown in the catalogue', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/admin/courses');

  await page.getByRole('button', { name: '+ Nouveau cours' }).click();
  await page.getByLabel('Slug').fill('Cours E2E');
  await page.getByLabel(/^Titre/).fill('Cours de bout en bout');
  await page.getByRole('button', { name: 'Créer le cours' }).click();
  await expect(page.getByText('Lettres minuscules, chiffres et tirets uniquement', { exact: false })).toBeVisible();

  await page.getByLabel('Slug').fill('cours-e2e');
  await page.getByLabel(/Public/).check();
  await page.getByRole('button', { name: 'Créer le cours' }).click();
  await expect(page.getByText('Cours créé')).toBeVisible();

  const course = page.locator('.course-item', { hasText: 'cours-e2e' });
  await course.getByRole('button', { name: '+ Ajouter' }).click();
  await course.getByLabel('Nom *').fill('Bienvenue');
  await course.getByLabel('Contenu (markdown)').fill('## Bienvenue\n\nPremier module.');
  await course.getByRole('button', { name: 'Ajouter', exact: true }).click();
  await expect(page.getByText('Module ajouté')).toBeVisible();
  await expect(course.locator('.mod-name')).toHaveText(['Bienvenue']);

  await page.goto('/courses');
  await page.getByLabel('Rechercher un cours').fill('bout en bout');
  await expect(page.locator('.rail-card-title', { hasText: 'Cours de bout en bout' })).toBeVisible();
});
