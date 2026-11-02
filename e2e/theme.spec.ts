import { expect, test } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// Light and dark mode, and the phone navigation (docs/specs/005-ui-theme-navigation.md).

test('the theme toggle switches to dark mode, which survives a reload', async ({ page }) => {
  await page.goto('/courses');
  const html = page.locator('html');
  await expect(html).not.toHaveClass(/dark/);

  await page.getByRole('button', { name: 'Basculer le thème' }).click();
  await expect(html).toHaveClass(/dark/);
  await page.reload();
  await expect(html).toHaveClass(/dark/);

  await page.getByRole('button', { name: 'Basculer le thème' }).click();
  await expect(html).not.toHaveClass(/dark/);
});

test.describe('on a phone', () => {
  test.use({ viewport: { width: 375, height: 812 } });

  test('the navigation opens in a drawer', async ({ page }) => {
    await signIn(page, ACCOUNTS.alice, '/courses');
    await page.getByRole('button', { name: 'Menu' }).click();
    const drawer = page.getByRole('dialog');
    await expect(drawer).toBeVisible();
    await drawer.getByRole('link', { name: /Mes badges/ }).click();
    await expect(page).toHaveURL('/my-badges');
    await expect(page.getByRole('heading', { name: 'Mes badges' })).toBeVisible();
  });
});
