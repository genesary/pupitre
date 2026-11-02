import { expect, test } from '@playwright/test';

import { ACCOUNTS, createCourse, signIn } from './helpers';

// Markdown patterns: live preview, then rendering in a lesson (docs/specs/045, 012).

test('an admin creates a pattern with a live preview; a lesson renders it with its CSS', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/admin/patterns');
  await page.getByRole('button', { name: '+ Nouveau pattern' }).click();
  await page.getByLabel(/^Nom/).fill('rouge');
  await page.getByLabel('Label').fill('Texte rouge');
  await page.getByLabel('CSS').fill('.md-pat-rouge { color: rgb(200, 0, 0); }');
  await page.getByLabel('Markdown personnalisé').fill('Un |||rouge mot important||| ici.');

  const preview = page.locator('.preview-area .md-pat-rouge');
  await expect(preview).toHaveText('mot important');
  await expect(preview).toHaveCSS('color', 'rgb(200, 0, 0)');

  await page.getByRole('button', { name: 'Créer le pattern' }).click();
  await expect(page.getByText('Pattern « rouge » créé')).toBeVisible();
  await expect(page.locator('.pat-name', { hasText: '|||rouge' })).toBeVisible();

  await createCourse(page, 'cours-patterns', 'Cours avec pattern', {
    name: 'Leçon',
    content: '## Leçon\n\nCeci est |||rouge très important|||.',
  });
  await page.goto('/courses/cours-patterns/lessons/0');
  const word = page.locator('.md-pat-rouge');
  await expect(word).toHaveText('très important');
  await expect(word).toHaveCSS('color', 'rgb(200, 0, 0)');
});
