import { expect, test } from '@playwright/test';

import { ACCOUNTS, adminCall, signIn } from './helpers';

// A learner takes a whole course against the real API: the courses domain
// records progress, XP and the badge in the users domain, in-process
// (docs/specs/104-backend-in-process-calls.md). The database outlives a run:
// a course already taken by a previous run is simply taken again.

const COURSE = 'linux-intro';

type Spec = { modules: { type: string; questions?: { answers: { text: string; correct?: boolean }[] }[] }[] };

test('a learner completes a course: lessons, quiz, XP and badge', async ({ page, request }) => {
  // The right answers of the quiz, from the admin definition.
  const definition = await adminCall(request, 'GET', `/admin/courses/${COURSE}/definition`);
  const { spec } = (await definition.json()) as { spec: Spec };
  const rightAnswers = spec.modules
    .find((module) => module.type === 'quiz')!
    .questions!.map((question) => question.answers.find((answer) => answer.correct)!.text);

  await signIn(page, ACCOUNTS.learner, `/courses/${COURSE}`);

  const enroll = page.getByRole('button', { name: "S'inscrire au cours" });
  if (await enroll.isVisible()) {
    await enroll.click();
    await expect(page.getByText('Inscription confirmée')).toBeVisible();
  }

  // The lessons, one "Suivant" at a time: completing one unlocks the next. A
  // lesson already seen by a previous run offers a link instead of a button.
  await page.goto(`/courses/${COURSE}/lessons/0`);
  for (let lesson = 1; lesson <= 2; lesson++) {
    const next = page.getByRole('button', { name: 'Suivant ▸' }).or(page.getByRole('link', { name: 'Suivant ▸' }));
    await next.click();
    await expect(page).toHaveURL(new RegExp(`/lessons/${lesson}$`));
  }

  // The quiz shown inside the last lesson, answered right (unless a previous
  // run passed it already).
  const submit = page.getByRole('button', { name: 'Valider le quiz ▸' });
  const finish = page.getByRole('button', { name: 'Terminer le cours ▸' });
  await expect(submit.or(finish).first()).toBeVisible();
  if (await submit.isVisible()) {
    for (const text of rightAnswers) {
    await page
      .locator('[class*=quiz] button')
      // The rendered text: markdown drops what looks like a tag (`<service>`).
      .filter({ hasText: (text.split('<')[0] ?? text).trim().slice(0, 25) })
      .first()
      .click();
    }
    await submit.click();
    await expect(page.getByText(`${rightAnswers.length}/${rightAnswers.length}`).first()).toBeVisible();
  }

  await finish.click();
  await expect(page.getByText('✓ Badge obtenu')).toBeVisible();
  await expect(page.getByText('+200 XP ✓')).toBeVisible();

  await page.goto('/my-badges');
  await expect(page.getByText('Explorateur Linux').first()).toBeVisible();

  await page.goto('/my-xp');
  await expect(page.getByText(/\d+ points accumulés/)).toBeVisible();
});
