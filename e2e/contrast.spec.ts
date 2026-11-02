import { expect, test, type Locator, type Page } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// Contrast of the texts the pupitre CSS left unreadable (docs/specs/090-gaps-and-defects.md, A26:
// dark mode; A27: locked modules): each must reach the WCAG AA ratio (4.5:1) against the
// background it really sits on, its ancestors' opacity included.

/**
 * Contrast ratio of an element's text against its composited background. Only background colours
 * are composited, not background images: text on a gradient card (`.continue-hero`, `.xp-hero`)
 * would be misread, so such elements are not checked here.
 */
async function contrast(locator: Locator): Promise<number> {
  return locator.evaluate((element) => {
    type Rgba = { r: number; g: number; b: number; a: number };
    const parse = (css: string): Rgba => {
      const [r = 0, g = 0, b = 0, a = 1] = (css.match(/[\d.]+/g) ?? []).map(Number);
      return { r, g, b, a };
    };
    const over = (top: Rgba, bottom: Rgba): Rgba => ({
      r: top.r * top.a + bottom.r * (1 - top.a),
      g: top.g * top.a + bottom.g * (1 - top.a),
      b: top.b * top.a + bottom.b * (1 - top.a),
      a: 1,
    });
    const luminance = ({ r, g, b }: Rgba) => {
      const channel = (v: number) => ((v /= 255) <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
      return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
    };

    const layers: Rgba[] = [];
    for (let e: Element | null = element; e; e = e.parentElement) {
      const color = parse(getComputedStyle(e).backgroundColor);
      if (color.a > 0) layers.push(color);
      if (color.a >= 1) break;
    }
    let background = parse(getComputedStyle(document.body).backgroundColor);
    for (const layer of layers.reverse()) background = over(layer, background);
    let opacity = 1;
    for (let e: Element | null = element; e; e = e.parentElement) opacity *= Number(getComputedStyle(e).opacity);
    const ink = parse(getComputedStyle(element).color);
    const text = over({ ...ink, a: ink.a * opacity }, background);

    const [light, dark] = [luminance(text), luminance(background)].sort((x, y) => y - x);
    return (light! + 0.05) / (dark! + 0.05);
  });
}

async function expectReadable(page: Page, locator: Locator, name: string) {
  await expect(locator, name).toBeVisible();
  expect(await contrast(locator), `${name} on ${page.url()}`).toBeGreaterThanOrEqual(4.5);
}

for (const colorScheme of ['light', 'dark'] as const) {
  test(`locked modules and the current module stay readable (${colorScheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme });
    await signIn(page, ACCOUNTS.alice, '/courses/pupitre-demo');
    await expectReadable(page, page.locator('.mod-row.locked .mod-name').first(), 'locked module name');
    await expectReadable(page, page.locator('.mod-row.locked .badge').getByText('Verrouillé').first(), '« Verrouillé »');

    await page.goto('/courses/kubernetes-basics');
    await page.getByRole('link', { name: /Continuer le cours/ }).click();
    await expectReadable(page, page.locator('.sidebar-link.locked .sidebar-link-text').first(), 'locked sidebar module');
    await expectReadable(page, page.locator('.sidebar-link.active .sidebar-link-text').first(), 'current sidebar module');
  });
}

test.describe('dark mode', () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark' });
  });

  test('the learner pages stay readable in dark mode', async ({ page }) => {
    await signIn(page, ACCOUNTS.alice, '/courses');
    await expectReadable(page, page.locator('.chip-reset'), '« Tout » chip');

    await page.goto('/courses/kubernetes-basics');
    await expectReadable(page, page.locator('.mod-name').first(), 'module name');

    await page.goto('/courses/atelier-kubernetes');
    await expectReadable(page, page.locator('.session-title').first(), 'session title');

    await page.goto('/agenda');
    await expectReadable(page, page.locator('.cal-today-btn'), '« Aujourd’hui »');
    await expectReadable(page, page.locator('.cal-month-btn'), 'month');
    await expectReadable(page, page.locator('.cal-cell:not(.other-month):not(.today) .cal-day-num').first(), 'day number');
    await expectReadable(page, page.locator('.ev-course').first(), 'upcoming session');
  });

  test('the admin and manager pages stay readable in dark mode', async ({ page }) => {
    await signIn(page, ACCOUNTS.admin, '/admin/courses');
    await expectReadable(page, page.locator('.link-btn.violet').first(), '« Définition » link');

    await page.goto('/admin/settings');
    await expectReadable(page, page.locator('.info-box').first(), 'info box');

    await page.getByRole('button', { name: 'Déconnexion' }).click();
    await signIn(page, ACCOUNTS.manager, '/manager/users');
    await expectReadable(page, page.locator('.mu-table tbody td').getByText('alice', { exact: true }), 'member name');
  });
});
