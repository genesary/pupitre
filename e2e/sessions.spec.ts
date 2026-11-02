import { expect, test } from '@playwright/test';

import { ACCOUNTS, inDays, signIn, signOut } from './helpers';

// In-person sessions (docs/specs/022-agenda-and-sessions.md).

test('an admin schedules a session; a learner books it; once full it refuses more', async ({ page }) => {
  await signIn(page, ACCOUNTS.admin, '/agenda');
  await page.getByRole('button', { name: '+ Nouvelle session' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Cours').selectOption('atelier-kubernetes');
  await dialog.getByLabel('Titre').fill('Session de Nantes');
  await dialog.getByLabel('Date et heure').fill(inDays(3, 10));
  await dialog.getByRole('button', { name: 'Créer' }).click();
  // No location: refused below the field, nothing sent.
  await expect(dialog.getByText('Le lieu est requis.')).toBeVisible();
  await dialog.getByLabel('Lieu').fill('Nantes — salle Hopper');
  await dialog.getByLabel('Capacité').fill('1');
  await dialog.getByRole('button', { name: 'Créer' }).click();
  await expect(page.getByText('Session créée')).toBeVisible();
  await signOut(page);

  // The learner books it from the course page; the agenda shows it booked.
  await signIn(page, ACCOUNTS.alice, '/courses/atelier-kubernetes');
  const card = page.locator('.session-card', { hasText: 'Session de Nantes' });
  await card.getByRole('button', { name: 'Réserver' }).click();
  await expect(card.getByRole('button', { name: '✓ Réservé — Annuler' })).toBeVisible();
  await page.goto('/agenda');
  await expect(page.locator('.ev-card', { hasText: 'Session de Nantes' })).toContainText('✓ Inscrit(e)');
  await signOut(page);

  // One place only: the next booking is refused.
  await signIn(page, ACCOUNTS.manager, '/courses/atelier-kubernetes');
  await page.locator('.session-card', { hasText: 'Session de Nantes' }).getByRole('button', { name: 'Réserver' }).click();
  await expect(page.getByText('Cette session est complète.')).toBeVisible();
});

test('a learner sees no session management in the agenda', async ({ page }) => {
  await signIn(page, ACCOUNTS.alice, '/agenda');
  await expect(page.locator('.ev-card').first()).toBeVisible();
  await expect(page.getByRole('button', { name: '+ Nouvelle session' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Modifier' })).toHaveCount(0);
});
