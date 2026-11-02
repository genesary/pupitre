import { expect, test } from '@playwright/test';

import { ACCOUNTS, createCourse, signIn, signOut } from './helpers';

// GitLab labs: a learner checks the steps, the admin tracks the checks (docs/specs/014, 047).
// The mock check of a step fails the first time and passes the second.

test('a learner validates the steps of a lab; the admin sees the checks', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/admin/courses');
  await createCourse(page, 'lab-e2e', 'Lab de bout en bout', {
    name: 'Lab GitLab',
    type: 'lab',
    content: '## Objectif\n\nCréez votre projet.\n\n<!-- check:0 -->',
    advanced: { checkProvider: 'gitlab', steps: [{ title: 'Projet créé', checkType: 'project_exists' }] },
  });
  await signOut(page);

  await signIn(page, ACCOUNTS.alice, '/courses/lab-e2e/lessons/0');
  const step = page.locator('.step-block', { hasText: 'Projet créé' });
  await step.getByRole('button', { name: 'Vérifier cette étape' }).click();
  await expect(step.getByText('✗ Pas encore')).toBeVisible();
  await step.getByRole('button', { name: 'Vérifier cette étape' }).click();
  await expect(step.getByText('✓ Étape validée !')).toBeVisible();
  await signOut(page);

  await signIn(page, ACCOUNTS.admin, '/admin/labs');
  await page.getByLabel('Cours').selectOption('lab-e2e');
  const rows = page.locator('tbody tr');
  await expect(rows).toHaveCount(2);
  await expect(rows.filter({ hasText: '✓ Validé' })).toContainText('alice');
  await expect(rows.filter({ hasText: '✗ Échec' })).toContainText('Projet créé');
  await expect(rows.first()).toContainText('Serveur');
});
