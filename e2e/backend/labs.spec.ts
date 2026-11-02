import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

import { expect, test, type Page } from '@playwright/test';

import { ACCOUNTS, adminCall, signIn } from './helpers';

// GitLab labs against the real API (docs/specs/014-labs.md, 107): the
// checker of pupitre-api reads the learner's project from the fake GitLab
// of e2e/backend/fake-gitlab.mjs (GITLAB_BASE_URL), as ebuildy/pupitre2's
// e2e/labs.spec.ts does. The lab's text comes from a local git repository,
// since step checks need a lab with git content. The learner is alice, whose
// GitLab username is her Keycloak login (190 B11).

const API = process.env.PUPITRE_API_URL ?? 'http://localhost:8080';
const GITLAB = `http://localhost:${process.env.FAKE_GITLAB_PORT ?? 3103}`;
const COURSE = 'e2e-gitlab-lab';
const PROJECT = `e-learning/${ACCOUNTS.learner.username}`;
const params = { project: 'e-learning/{{ .Username }}' };

/** Describes the learner's project in the fake GitLab, replacing what it was. */
async function fakeProject(project: Record<string, unknown>) {
  const response = await fetch(`${GITLAB}/__fake/projects`, { method: 'POST', body: JSON.stringify({ path: PROJECT, ...project }) });
  if (!response.ok) throw new Error(`fake gitlab: ${response.status}`);
}

/** A git repository holding the lab's text, as a course repository does. */
function labRepository(): string {
  const dir = mkdtempSync(join(tmpdir(), 'pupitre-e2e-lab-repo-'));
  mkdirSync(join(dir, 'lab'), { recursive: true });
  writeFileSync(
    join(dir, 'lab', 'content.md'),
    '# Le lab\n\nCréez une branche.\n\n<!-- check:0 -->\n\nPuis ouvrez une merge request.\n\n<!-- check:1 -->\n\nFin du lab.\n',
  );
  const git = (...args: string[]) =>
    execFileSync('git', ['-c', 'user.name=e2e', '-c', 'user.email=e2e@pupitre.local', ...args], { cwd: dir, stdio: 'pipe' });
  git('init', '--initial-branch=main');
  git('add', '.');
  git('commit', '-m', 'docs: lab');
  return pathToFileURL(dir).href;
}

/** The course: one GitLab lab, then the module the lab hands over to. */
function courseSpec(src: string) {
  return {
    title: 'Lab GitLab de bout en bout',
    description: 'Un lab vérifié contre un GitLab de test.',
    public: true,
    modules: [
      {
        name: 'Lab GitLab',
        type: 'lab',
        src,
        ref: 'main',
        path: 'lab/content.md',
        labUrl: 'https://gitlab.test/e-learning',
        checkProvider: 'gitlab',
        steps: [
          { title: 'Créer une branche', checkType: 'gitlab_branch', checkParams: { ...params, pattern: 'feature/' } },
          { title: 'Ouvrir une merge request', checkType: 'gitlab_mr_open', checkParams: params },
        ],
      },
      {
        name: 'Toutes les vérifications',
        type: 'lab',
        src,
        ref: 'main',
        path: 'lab/content.md',
        checkProvider: 'gitlab',
        steps: [
          { title: 'Commit conventionnel', checkType: 'gitlab_conventional_commit', checkParams: params },
          { title: 'Pipeline', checkType: 'gitlab_pipeline_passed', checkParams: params },
          { title: 'Commit sur la branche', checkType: 'gitlab_commit_on_branch', checkParams: { ...params, pattern: 'feature/' } },
          { title: 'Fichier modifié', checkType: 'gitlab_file_on_branch', checkParams: { ...params, pattern: 'feature/', file: 'lab.py' } },
          { title: 'Commits conventionnels', checkType: 'gitlab_conventional_commit_on_branch', checkParams: { ...params, pattern: 'feature/' } },
          { title: 'Inconnu', checkType: 'gitlab_nope', checkParams: params },
          { title: 'Projet d’un autre', checkType: 'gitlab_branch', checkParams: { project: 'e-learning/bob', pattern: 'feature/' } },
        ],
      },
    ],
  };
}

// One sign-in for both tests: the sign-in routes allow 10 calls a minute.
test.describe.configure({ mode: 'serial' });
let page: Page;

test.beforeAll(async ({ browser, request }) => {
  const spec = courseSpec(labRepository());
  const created = await adminCall(request, 'POST', '/admin/courses', { slug: COURSE, spec });
  if (created.status() === 409) {
    const updated = await adminCall(request, 'PUT', `/admin/courses/${COURSE}/definition`, { spec });
    expect(updated.ok(), `course update: ${updated.status()} ${await updated.text()}`).toBeTruthy();
  } else {
    expect(created.ok(), `course creation: ${created.status()} ${await created.text()}`).toBeTruthy();
  }

  // alice signs in on the course and enrolls, unless a previous run did.
  page = await browser.newPage();
  await signIn(page, ACCOUNTS.learner, `/courses/${COURSE}`);
  const enroll = page.getByRole('button', { name: "S'inscrire au cours" });
  if (await enroll.isVisible()) {
    await enroll.click();
    await expect(page.getByText('Inscription confirmée')).toBeVisible();
  }
});

test.afterAll(async () => {
  await page?.close();
});

test('a learner validates the GitLab steps of a lab', async () => {
  await fakeProject({});
  await page.goto(`/courses/${COURSE}/lessons/0`);

  // The steps sit where their markers are in the lab's text from git.
  await expect(page.getByText('Créez une branche.')).toBeVisible();
  const first = page.locator('.step-block', { hasText: 'Créer une branche' });
  const second = page.locator('.step-block', { hasText: 'Ouvrir une merge request' });

  await first.getByRole('button', { name: 'Vérifier cette étape' }).click();
  await expect(first.getByText('✗ Pas encore')).toBeVisible();
  await expect(first).toContainText('Aucune branche commençant par "feature/" trouvée.');

  await fakeProject({ branches: { 'feature/login': { commits: ['feat: login'] } } });
  await first.getByRole('button', { name: 'Vérifier cette étape' }).click();
  await expect(first.getByText('✓ Étape validée !')).toBeVisible();

  // Only the learner's own merge requests count.
  await fakeProject({
    branches: { 'feature/login': { commits: ['feat: login'] } },
    mergeRequests: [{ title: 'Login', source_branch: 'feature/login', author: 'bob' }],
  });
  await second.getByRole('button', { name: 'Vérifier cette étape' }).click();
  await expect(second).toContainText('Aucune Merge Request ouverte trouvée.');

  await fakeProject({
    branches: { 'feature/login': { commits: ['feat: login'] } },
    mergeRequests: [{ title: 'Login', source_branch: 'feature/login', author: ACCOUNTS.learner.username }],
  });
  await second.getByRole('button', { name: 'Vérifier cette étape' }).click();
  await expect(second.getByText('✓ Étape validée !')).toBeVisible();

  // Every step validated: the lab is done. The learner moves on to the next
  // module, or back to the course while it was still locked when the lab
  // opened (on the first run).
  await expect(page).toHaveURL(new RegExp(`/courses/${COURSE}(/lessons/1)?$`));
});

test('every step check type, through the API', async () => {
  const token = await page.evaluate(() => (JSON.parse(sessionStorage.getItem('pupitre.session') ?? '{}') as { token?: string }).token);
  expect(token, 'session token').toBeTruthy();
  const check = async (step: number) => {
    const response = await page.request.post(`${API}/api/courses/${COURSE}/modules/1/steps/${step}/check`, {
      headers: { Authorization: `Bearer ${token}` },
      data: {},
    });
    expect(response.ok(), `step ${step}: ${response.status()} ${await response.text()}`).toBeTruthy();
    return (await response.json()) as { allow: boolean; violations?: string[] };
  };

  await fakeProject({
    branches: { 'feature/login': { commits: ['wip'], files: ['README.md'] } },
    mergeRequests: [
      { title: 'Login', source_branch: 'feature/login', author: ACCOUNTS.learner.username, commits: ['wip'], pipeline: 'failed' },
    ],
  });
  expect(await check(0)).toMatchObject({ allow: false, violations: [expect.stringContaining('Aucun commit conventionnel')] });
  expect(await check(1)).toMatchObject({ allow: false, violations: ['Pipeline non validée (statut : failed). Vérifiez les logs CI.'] });
  expect(await check(2)).toMatchObject({ allow: true });
  expect(await check(3)).toMatchObject({ allow: false, violations: [expect.stringContaining('"lab.py" n\'a pas été modifié')] });
  expect(await check(4)).toMatchObject({ allow: false, violations: [expect.stringContaining('Commit non conventionnel : "wip"')] });
  expect(await check(5)).toMatchObject({ allow: false, violations: ['Type de vérification inconnu : gitlab_nope'] });
  expect(await check(6)).toMatchObject({ allow: false, violations: ['Le projet ne correspond pas à votre compte GitLab.'] });

  await fakeProject({
    branches: { 'feature/login': { commits: ['feat: login', 'fix: tests'], files: ['lab.py'] } },
    mergeRequests: [
      { title: 'Login', source_branch: 'feature/login', author: ACCOUNTS.learner.username, commits: ['wip', 'fix: tests'], pipeline: 'success' },
    ],
  });
  for (const step of [0, 1, 3, 4]) expect(await check(step), `step ${step}`).toMatchObject({ allow: true });
});
