import { expect, test } from '@playwright/test';

import { ACCOUNTS, signIn } from './helpers';

// Following a course: enrollment, lessons, quiz (docs/specs/011, 012, 013).

test('a learner enrolls in a course, then finds it in « Mes cours »', async ({ page }) => {
  await signIn(page, ACCOUNTS.alice, '/my-courses');
  await expect(page.getByText('3 cours', { exact: true })).toBeVisible();

  await page.goto('/courses/docker-fundamentals');
  await page.getByRole('button', { name: "S'inscrire au cours" }).click();
  await expect(page.getByText('Inscription confirmée')).toBeVisible();

  await page.goto('/my-courses');
  await expect(page.getByText('4 cours', { exact: true })).toBeVisible();
  await expect(page.getByText('Les fondamentaux de Docker')).toBeVisible();
});

test('a learner follows the lessons of a course and submits its quiz', async ({ page }) => {
  await signIn(page, ACCOUNTS.alice, '/my-courses');

  await page.goto('/courses/kubernetes-basics');
  await page.getByRole('link', { name: /Continuer le cours/ }).click();
  await expect(page).toHaveURL(/\/courses\/kubernetes-basics\/lessons\/\d+$/);

  // The quiz of this course is inline: it shows at the bottom of the last lesson.
  const nav = page.locator('.lesson-nav');
  const validate = page.getByRole('button', { name: 'Valider le quiz ▸' });
  const next = nav.getByRole('button', { name: /Suivant/ }).or(nav.getByRole('link', { name: /Suivant/ })).first();
  for (let step = 0; step < 4; step++) {
    // Wait for the lesson to render: either its quiz or the way to the next lesson.
    await expect(validate.or(next).first()).toBeVisible();
    if (await validate.isVisible()) break;
    const url = page.url();
    await next.click();
    await page.waitForURL((target) => target.href !== url);
  }
  await expect(validate).toBeVisible();

  // One answer per question: the first option.
  const questions = page.locator('.mini-q-text');
  const count = await questions.count();
  for (let i = 0; i < count; i++) {
    await questions.nth(i).locator('xpath=following-sibling::*[last()]').getByRole('button').first().click();
  }
  await validate.click();
  await expect(page.getByText(/Quiz réussi|Quiz échoué/)).toBeVisible();
  await expect(page.getByText('Correction', { exact: true })).toBeVisible();
});
