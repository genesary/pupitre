import { expect, test } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// A manager enrolls a member of their scope in a course (docs/specs/030-manager-space.md).

test('a manager enrolls a member in a course and sees the count change', async ({ page }) => {
  await signIn(page, ACCOUNTS.manager, '/manager/users');

  const row = page.locator('.mu-table tbody tr', { hasText: 'alice@pupitre.test' });
  await expect(row).toContainText('3 cours');
  await row.getByRole('button', { name: 'Gérer les inscriptions' }).click();

  const modal = page.getByRole('dialog');
  await expect(modal).toContainText('alice — Inscriptions');
  await modal.getByLabel('Cours', { exact: true }).selectOption('docker-fundamentals');
  await modal.getByRole('button', { name: 'Inscrire' }).first().click();
  await expect(page.getByText('Inscrit avec succès')).toBeVisible();
  await expect(modal).toContainText('Les fondamentaux de Docker');

  await page.keyboard.press('Escape');
  await expect(row).toContainText('4 cours');
});
