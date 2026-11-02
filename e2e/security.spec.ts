import { expect, test } from '@playwright/test';

import { ACCOUNTS, forbidDialogs, signIn, TRAP_ID } from './helpers';

// The trap account carries the XSS payloads of audit findings C2 and C4
// (docs/specs/003-security.md): they must show as text and never run.

test('the payloads of the trap account never run', async ({ page }) => {
  forbidDialogs(page);
  await signIn(page, ACCOUNTS.trap, '/profile');

  await expect(page.getByText('<script>alert("xss-bio")</script>')).toBeVisible();
  await expect(page.locator('.topnav')).toContainText(ACCOUNTS.trap);

  await page.goto(`/users/${TRAP_ID}`);
  await expect(page.locator('.up-username')).toHaveText(ACCOUNTS.trap);

  await page.goto('/my-groups');
  await expect(page.getByRole('heading', { name: 'Mes groupes' })).toBeVisible();

  await page.goto('/leaderboard');
  await expect(page.getByText(ACCOUNTS.trap).first()).toBeVisible();
  await expect(page.locator('img[src="x"]')).toHaveCount(0);
  await expect(page.locator('[onerror]')).toHaveCount(0);
});
