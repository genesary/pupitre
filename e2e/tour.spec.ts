import { expect, test, type Page } from '@playwright/test';

import { ACCOUNTS, forbidDialogs, signIn } from './helpers';

// The guided tour and the first lesson tip (docs/specs/007-guided-tour.md).

const coach = (page: Page) => page.locator('.tour-coach');

/** Clicks through the running tour; returns the step titles and the pages visited. */
async function runTour(page: Page) {
  const titles: string[] = [];
  const pages: string[] = [];
  for (let step = 0; step < 30; step++) {
    const next = coach(page).getByRole('button', { name: /^(Suivant|Terminer)$/ });
    await expect(next).toBeVisible();
    titles.push(await coach(page).locator('.tour-title').innerText());
    const path = new URL(page.url()).pathname;
    if (pages.at(-1) !== path) pages.push(path);
    const label = await next.innerText();
    await next.click();
    if (label === 'Terminer') break;
  }
  await expect(coach(page)).toHaveCount(0);
  return { titles, pages };
}

test('a new learner is offered the tour and goes through every page, without changing anything', async ({ page }) => {
  const writes: string[] = [];
  page.on('request', (request) => {
    if (request.method() !== 'GET' && request.url().includes('/api/')) writes.push(request.url());
  });
  await signIn(page, ACCOUNTS.alice, '/my-courses', { tour: true });

  await expect(coach(page).getByText('Bonjour alice !')).toBeVisible();
  writes.length = 0;
  await coach(page).getByRole('button', { name: "C'est parti !" }).click();

  const { titles, pages } = await runTour(page);
  expect(titles[0]).toBe('Le catalogue');
  expect(titles).toContain('Mes groupes');
  expect(titles).toContain("S'inscrire");
  expect(titles).not.toContain('Mon espace');
  expect(titles).not.toContain('Administration');
  expect(titles.at(-1)).toBe("C'est tout !");
  expect(pages[0]).toBe('/courses');
  expect(pages[1]).toMatch(/^\/courses\/[^/]+$/);
  expect(pages.slice(2)).toEqual([
    '/my-courses',
    '/my-paths',
    '/my-groups',
    '/my-badges',
    '/my-xp',
    '/leaderboard',
    '/my-skills',
    '/agenda',
  ]);
  expect(writes).toEqual([]);

  // Done: not offered again.
  await page.reload();
  await expect(page.locator('.cal-topbar-title')).toBeVisible();
  await expect(coach(page)).toHaveCount(0);
});

test('« Plus tard » is remembered, and the side menu restarts the tour', async ({ page }) => {
  await signIn(page, ACCOUNTS.alice, '/my-courses', { tour: true });
  await coach(page).getByRole('button', { name: 'Plus tard' }).click();
  await expect(page.getByText('Vous pourrez relancer la visite depuis le menu.')).toBeVisible();

  await page.reload();
  await expect(page.getByRole('heading', { name: 'Mes cours' })).toBeVisible();
  await expect(coach(page)).toHaveCount(0);

  await page.getByRole('button', { name: 'Menu' }).click();
  await page.getByRole('button', { name: 'Visite guidée' }).click();
  await expect(coach(page).locator('.tour-title', { hasText: 'Le catalogue' })).toBeVisible();
  await expect(page).toHaveURL(/\/courses$/);

  // Escape ends it, gives focus back and says how to come back.
  await page.keyboard.press('Escape');
  await expect(coach(page)).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Menu' })).toBeFocused();
  await expect(page.getByText('Visite interrompue. Relancez-la depuis le menu quand vous voulez.')).toBeVisible();
});

test('the manager and the admin see the step of their own space', async ({ page }) => {
  await signIn(page, ACCOUNTS.manager, '/my-courses', { tour: true });
  await coach(page).getByRole('button', { name: "C'est parti !" }).click();
  const manager = await runTour(page);
  expect(manager.titles).toContain('Mon espace');
  expect(manager.titles).not.toContain('Administration');

  await page.goto('/my-courses');
  await page.getByRole('button', { name: 'Déconnexion' }).click();
  await signIn(page, ACCOUNTS.admin, '/my-courses', { tour: true });
  await coach(page).getByRole('button', { name: "C'est parti !" }).click();
  const admin = await runTour(page);
  expect(admin.titles).toContain('Administration');
  expect(admin.titles).not.toContain('Mon espace');
});

test('the welcome bubble shows a malicious username as text', async ({ page }) => {
  forbidDialogs(page);
  await signIn(page, ACCOUNTS.trap, '/my-courses', { tour: true });
  await expect(coach(page).getByText(`Bonjour ${ACCOUNTS.trap} !`)).toBeVisible();
  await expect(coach(page).locator('img[src="x"]')).toHaveCount(0);
});

test('the first lesson opened shows the lesson tip, once', async ({ page }) => {
  await signIn(page, ACCOUNTS.alice, '/my-courses', { tour: true });
  await coach(page).getByRole('button', { name: 'Plus tard' }).click();

  await page.goto('/courses/kubernetes-basics');
  await page.getByRole('link', { name: /Continuer le cours/ }).click();
  await expect(page).toHaveURL(/\/lessons\/\d+$/);
  await expect(coach(page).locator('.tour-title', { hasText: 'Les modules' })).toBeVisible();
  await expect(coach(page).getByText('1 / 3')).toBeVisible();
  await coach(page).getByRole('button', { name: 'Suivant' }).click();
  await expect(coach(page).locator('.tour-title', { hasText: 'La leçon' })).toBeVisible();
  await coach(page).getByRole('button', { name: 'Passer' }).click();
  await expect(coach(page)).toHaveCount(0);

  // The next lesson does not show it again.
  const url = page.url();
  const nav = page.locator('.lesson-nav');
  await nav.getByRole('button', { name: /Suivant/ }).or(nav.getByRole('link', { name: /Suivant/ })).first().click();
  await page.waitForURL((target) => target.href !== url);
  await expect(page.locator('.lesson-title')).toBeVisible();
  await expect(coach(page)).toHaveCount(0);
});

test('the bubble follows the dark theme', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await signIn(page, ACCOUNTS.alice, '/my-courses', { tour: true });
  await coach(page).getByRole('button', { name: "C'est parti !" }).click();
  await expect(coach(page).getByRole('button', { name: 'Suivant' })).toBeVisible();
  // --color-paper in dark mode (#1e2128), never the light card on a dark page.
  await expect(coach(page).locator('.tour-card')).toHaveCSS('background-color', 'rgb(30, 33, 40)');
});

test('the tour fits a 360 px screen', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 740 });
  await signIn(page, ACCOUNTS.alice, '/my-courses', { tour: true });
  await coach(page).getByRole('button', { name: "C'est parti !" }).click();
  await expect(coach(page).getByRole('button', { name: 'Suivant' })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
  const box = await coach(page).boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(360);
});
