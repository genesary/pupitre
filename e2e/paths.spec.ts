import { expect, test } from '@playwright/test';

import { ACCOUNTS, createCourse, signIn, signOut } from './helpers';

// Learning paths: created by an admin, followed by a learner (docs/specs/043, 020, 012).

test('an admin builds a path and enrolls a learner; the end of a course opened from it leads back to it', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/admin/courses');
  await createCourse(page, 'cours-du-parcours', 'Cours du parcours', { name: 'Unique leçon', content: '## Unique leçon\n\nC’est tout.' });

  await page.goto('/admin/paths');
  await page.getByRole('button', { name: '+ Nouveau parcours' }).click();
  const panel = page.locator('.draft-panel');
  await panel.getByLabel('Slug').fill('parcours-e2e');
  await panel.getByLabel(/^Titre/).fill('Parcours e2e');
  for (const slug of ['cours-du-parcours', 'linux-intro']) {
    await panel.getByLabel('Cours à ajouter').selectOption(slug);
    await panel.getByRole('button', { name: 'Ajouter' }).click();
  }
  await expect(panel.locator('.mod-info .font-mono')).toHaveText(['cours-du-parcours', 'linux-intro']);
  await panel.getByRole('button', { name: 'Créer le parcours' }).click();
  await expect(page.getByText('Parcours créé')).toBeVisible();

  await page.getByRole('tab', { name: 'Inscriptions' }).click();
  await page.getByLabel('Parcours :').selectOption('parcours-e2e');
  await page.getByLabel('Utilisateur').selectOption({ label: 'alice (alice@pupitre.test)' });
  await page.getByRole('button', { name: 'Inscrire' }).click();
  await expect(page.getByText('Utilisateur inscrit au parcours')).toBeVisible();
  await signOut(page);

  await signIn(page, ACCOUNTS.alice, '/my-paths');
  await page.getByRole('button', { name: /Parcours e2e/ }).click();
  await expect(page).toHaveURL('/my-paths?path=parcours-e2e');
  // An available step pulses (SVG animation): it is never « stable », so no stability wait.
  await page.getByRole('link', { name: 'Cours Du Parcours — Disponible' }).click({ force: true });
  await expect(page).toHaveURL('/courses/cours-du-parcours');

  await page.getByRole('link', { name: /Commencer le cours/ }).click();
  await page.getByRole('button', { name: /Terminer le cours/ }).click();
  await expect(page).toHaveURL('/my-paths?path=parcours-e2e');
  await expect(page.getByRole('link', { name: 'Cours Du Parcours — Terminé' })).toBeVisible();
});
