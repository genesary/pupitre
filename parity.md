# Parity with genesary/pupitre

Feature parity of pupitre2 with [`genesary/pupitre`](https://github.com/genesary/pupitre) at
commit `3cdf7b2`, the reference of every spec. One row per page of its Astro frontend
(`frontend/src/pages`, 37 pages), then its backend and its deployment.
[`ebuildy/pupitre2`](https://github.com/ebuildy/pupitre2) keeps the same kind of table
(`parity.md`) for its full rewrite; the feature numbers of its checklist (`Fxxx`) are the ones of
the [specs index](docs/specs/README.md).

Status: ✅ done · 🟡 partial · ⬜ to do · ➖ not carried over on purpose (reason given).

Tests: "mock" is the Playwright suite against the mock mode (`e2e/`, `pnpm test:e2e`), "backend"
the one against pupitre-api, PostgreSQL and Keycloak (`e2e/backend/`, `pnpm test:e2e:backend`).

## Learner pages

Served by the learner app (`apps/app`).

| genesary page | pupitre2 | Status | Spec | Tests | Notes |
| --- | --- | --- | --- | --- | --- |
| `index.astro` | `/` | ✅ | [001](docs/specs/001-keycloak-authentication.md) | | Redirects to `/courses`, as before |
| `dashboard.astro` | `/dashboard` | ✅ | [001](docs/specs/001-keycloak-authentication.md) | | Redirects to `/courses`, as before |
| `login.astro` | `/login` | 🟡 | [001](docs/specs/001-keycloak-authentication.md) | mock `auth`, backend (every sign-in) | Keycloak only: the e-mail, LDAP and GitHub forms are gone (see "Deliberate differences") |
| `register.astro` | `/register` | ➖ | [001](docs/specs/001-keycloak-authentication.md) | | Redirects to `/login`: accounts come from Keycloak |
| `auth/callback.astro` | `/auth/callback` | ✅ | [001](docs/specs/001-keycloak-authentication.md) | backend (every sign-in) | Same route, registered in Keycloak; keeps the `redirect` for every role (090 A14) |
| `courses/index.astro` | `/courses` | ✅ | [010](docs/specs/010-catalogue.md) | mock `admin`, `learner`; backend `roles` | Icon instead of the thumbnail the API never returned (090 A2) |
| `courses/[slug]/index.astro` | `/courses/:slug` | ✅ | [011](docs/specs/011-course-page-and-enrollment.md) | mock `learner`, `sessions`; backend `learner-flow` | Enrollment errors shown (090 A8) |
| `courses/[slug]/lessons/[index].astro` | `/courses/:slug/lessons/:index` | ✅ | [012](docs/specs/012-lesson-player.md), [014](docs/specs/014-labs.md) | mock `learner`, `labs`, `patterns`, `paths`; backend `learner-flow`, `labs` | Patterns applied (090 A1), course title in the breadcrumb (A9); labs with GitLab step checks |
| `courses/[slug]/quiz/[index].astro` | `/courses/:slug/quiz/:index` | ✅ | [013](docs/specs/013-quizzes.md) | mock `learner` (inline quiz); backend `learner-flow` | The answers are no longer sent with the results (190 B17) |
| `skills/[slug].astro` | `/skills/:slug` | ✅ | [020](docs/specs/020-learner-space.md) | | |
| `my-courses.astro` | `/my-courses` | ✅ | [020](docs/specs/020-learner-space.md) | mock `learner`, `apps`; backend `roles` | |
| `my-paths.astro` | `/my-paths` | ✅ | [020](docs/specs/020-learner-space.md) | mock `paths`; backend `roles` | No level badge: the API has no `level` (090 A18, 190 B18) |
| `my-skills.astro` | `/my-skills` | ✅ | [020](docs/specs/020-learner-space.md) | backend `roles` | Level from the server (090 A10) |
| `my-badges.astro` | `/my-badges` | ✅ | [020](docs/specs/020-learner-space.md) | mock `theme`; backend `roles`, `learner-flow` | |
| `my-xp.astro` | `/my-xp` | ✅ | [020](docs/specs/020-learner-space.md) | backend `roles`, `learner-flow` | |
| `my-groups.astro` | `/my-groups` | ✅ | [020](docs/specs/020-learner-space.md) | mock `security`; backend `roles` | |
| `leaderboard.astro` | `/leaderboard` | ✅ | [020](docs/specs/020-learner-space.md) | mock `security`; backend `roles` | |
| `users/[id].astro` | `/users/:id` | ✅ | [020](docs/specs/020-learner-space.md) | mock `security` | |
| `profile.astro` | `/profile` | ✅ | [021](docs/specs/021-profile.md) | mock `apps`, `security`; backend `roles` | |
| `agenda.astro` | `/agenda` | ✅ | [022](docs/specs/022-agenda-and-sessions.md) | mock `sessions`; backend `roles` | Capacity 0 allowed (090 A19), local time when editing (A20) |

## Manager pages

Served by the admin app (`apps/admin`), under `/manager`.

| genesary page | pupitre2 | Status | Spec | Tests | Notes |
| --- | --- | --- | --- | --- | --- |
| `manager/index.astro` | `/manager` | ✅ | [030](docs/specs/030-manager-space.md) | mock `apps` | Opens « Mes membres » |
| `manager/users/index.astro` | `/manager/users` | ✅ | [030](docs/specs/030-manager-space.md) | mock `manager`; backend `roles` | Group filter works (090 A21) |
| `manager/groups/index.astro` | `/manager/groups` | ✅ | [030](docs/specs/030-manager-space.md) | backend `roles` | The creator of a subgroup is made a member of it (190 B1) |
| `manager/paths/index.astro` | `/manager/paths` | ✅ | [030](docs/specs/030-manager-space.md) | | |

## Administration pages

Served by the admin app (`apps/admin`), under `/admin`.

| genesary page | pupitre2 | Status | Spec | Tests | Notes |
| --- | --- | --- | --- | --- | --- |
| `admin/[...slug].astro` | `/admin` | ✅ | [040](docs/specs/040-admin-dashboard.md) | backend `roles` | The dashboard; an unknown admin URL is a 404 (090 A15), one sidebar (A4), no always-zero statistics (A3) |
| `admin/monitoring/index.astro` | `/admin/monitoring` | ✅ | [040](docs/specs/040-admin-dashboard.md) | backend `roles` | No « Recalculer la progression »: it always failed (090 A23) |
| `admin/leaderboard/index.astro` | `/admin/leaderboard` | ✅ | [040](docs/specs/040-admin-dashboard.md) | backend `roles` | Same |
| `admin/users/index.astro` | `/admin/users` | ✅ | [041](docs/specs/041-admin-users.md) | mock `auth`; backend `roles`, setup | |
| `admin/courses/index.astro` | `/admin/courses` | ✅ | [042](docs/specs/042-admin-courses.md) | mock `admin`, `labs`, `paths`; backend `roles`, `labs` | No « Quiz ref » field, which was lost (090 A5); stale-write check on the definition |
| `admin/courses/sessions/[slug]/[sessionId].astro` | `/admin/courses/sessions/:slug/:sessionId` | ✅ | [022](docs/specs/022-agenda-and-sessions.md) | | Attendance, for admins and managers |
| `admin/paths/index.astro` | `/admin/paths` | ✅ | [043](docs/specs/043-admin-paths.md) | mock `paths`; backend `roles` | |
| `admin/groups/index.astro` | `/admin/groups` | ✅ | [044](docs/specs/044-admin-groups.md) | backend `roles`, setup | A group can map to `manager` (190 B20) |
| `admin/patterns/index.astro` | `/admin/patterns` | ✅ | [045](docs/specs/045-admin-patterns.md) | mock `patterns`; backend `roles` | Live preview; the JS is labelled « non exécuté » (090 A13) |
| `admin/settings/index.astro` | `/admin/settings` | ✅ | [046](docs/specs/046-admin-settings.md) | backend `roles` | Saves the changed keys of one section, never the masked secrets (090 A24) |
| `admin/labs/index.astro` | `/admin/labs` | ✅ | [047](docs/specs/047-admin-exports-and-labs.md) | mock `labs`; backend `roles` | |
| `admin/exports/index.astro` | `/admin/exports` | ✅ | [047](docs/specs/047-admin-exports-and-labs.md) | mock `auth`; backend `roles` | Also open to managers, as before; formula-safe CSV (190 B8) |

## Backend

genesary's three Go services run as one application, `apps/api` (`pupitre-api`), with the same
routes, tables and rules ([100](docs/specs/100-backend-scope-and-architecture.md)). Parity is
checked by `internal/server/router_test.go`: every route of
[103](docs/specs/103-backend-http-api.md) is served, no other.

| genesary | pupitre2 | Status | Spec | Notes |
| --- | --- | --- | --- | --- |
| user-service: accounts, sign-in (local, OAuth, OIDC, LDAP), groups, enrollments, XP, badges, paths, sessions, settings, exports | `internal/users` | ✅ | [105](docs/specs/105-backend-authentication.md), [106](docs/specs/106-backend-courses-and-content.md) | Every route kept |
| course-service: courses, modules, git content, quizzes, progress, patterns, uploads, lab checks | `internal/courses` | ✅ | [106](docs/specs/106-backend-courses-and-content.md), [107](docs/specs/107-backend-labs-and-checker.md) | Every route kept |
| checker-service: GitLab state, OPA policies, step checks | `internal/checker` | ✅ | [107](docs/specs/107-backend-labs-and-checker.md) | Called in-process, with the same OPA limits |
| `/internal/*`, `/evaluate`, `/check-step`, internal secret | function calls | ➖ | [104](docs/specs/104-backend-in-process-calls.md) | Only needed between services; their public exposure was audit finding H6 |
| Configuration of the three services | one loader | ✅ | [101](docs/specs/101-backend-configuration.md) | Same keys, one port (8080); removed keys warned about |
| Database schema and migrations | same tables | ✅ | [102](docs/specs/102-backend-data-and-migrations.md) | One migration added: a group may map to `manager` (190 B20) |
| `/health`, `/metrics` | one of each | ✅ | [103](docs/specs/103-backend-http-api.md) | `/metrics` public and routed by the ingress, as before (190, "Kept as the reference") |
| Demo data (`SEED_DEV_COURSES`, `SEED_MOCK_DATA`) | same | ✅ | [102](docs/specs/102-backend-data-and-migrations.md) | Wrong module prerequisites of the demo courses fixed (190 B23) |
| Lab checks against GitLab | same | ✅ | [107](docs/specs/107-backend-labs-and-checker.md) | Tested against a fake GitLab (backend `labs`); limit per learner (190 B9) |

## Deployment

| genesary | pupitre2 | Status | Spec | Notes |
| --- | --- | --- | --- | --- |
| Images `pupitre-user-service`, `pupitre-course-service`, `pupitre-checker-service` | `pupitre-api` | ✅ | [108](docs/specs/108-backend-operations-and-testing.md) | One image, `FROM scratch` |
| Image `pupitre-frontend` (Astro, Node server) | `pupitre-front` (nginx, static files) | ✅ | [108](docs/specs/108-backend-operations-and-testing.md) | Same port (3000); the security headers of [003](docs/specs/003-security.md) S6 |
| Tag-triggered image publishing and GitHub Release | same | ✅ | [108](docs/specs/108-backend-operations-and-testing.md) | `containers.yaml`, `release.yaml`; released as `v0.1.0` |
| Chart `genesary/pupitre-helm` | [`jojo91550/pupitre2-helm`](https://github.com/jojo91550/pupitre2-helm) | ✅ | [108](docs/specs/108-backend-operations-and-testing.md) | One API; same values for admin, SSO, database, ingress; uploads volume (190 B22); chart `2.0.0` |
| Local kind cluster | `apps/api/infra/kind`, `make kind-*` | ✅ | [108](docs/specs/108-backend-operations-and-testing.md) | |

## Deliberate differences

| Topic | genesary | pupitre2 | Why | Spec |
| --- | --- | --- | --- | --- |
| Sign-in | E-mail and password, registration, GitHub, LDAP, OIDC | OIDC through Keycloak only | Decision of the project; the backend still serves the other routes, turned off by settings (audit H7) | [001](docs/specs/001-keycloak-authentication.md) |
| Frontend defects | 28 defects of the Astro pages | Fixed | Found while writing the specs, and in use (A26-A28) | [090](docs/specs/090-gaps-and-defects.md) A1-A28 |
| Backend defects | Audit findings and defects found during the move | Fixed: B1, B4-B14, B15 (security headers), B17 (`correctAnswer`), B19, B20, B22-B24 | Security and real bugs, each approved and tested | [190](docs/specs/190-backend-defects.md) |
| Kept as the reference | - | B2, B3, B15 (`/metrics`), B16, B17 (e-mail verification), B18, B21 | Documented or relied on by genesary | [190](docs/specs/190-backend-defects.md), "Kept as the reference" |
| Language | French | French only, strings grouped for a later translation | | [005](docs/specs/005-ui-theme-navigation.md) |
| Look | Pupitre's CSS | Pupitre's CSS, unchanged | The design is kept as is | [005](docs/specs/005-ui-theme-navigation.md) |
| Page background | `--color-page` and the page wrappers' gradient | Learner app, light mode: the drawing of `ebuildy/pupitre2` | Decision of the project | [005](docs/specs/005-ui-theme-navigation.md) |
| Catalogue layout | Grid of `.courses-grid-lg`, difficulty select | Up to five columns of larger cards, entrance animation on each filter change, difficulty range slider, white panel in light mode; white panels behind the course page's content, the lesson and the quiz too; earned and locked badges easier to tell apart | Decision of the project | [005](docs/specs/005-ui-theme-navigation.md), [010](docs/specs/010-catalogue.md) |
| Base font size | 16 px | 17 px, the whole interface about 6 % larger | Readability, decision of the project | [005](docs/specs/005-ui-theme-navigation.md) |
| Light mode panels | Content over the page background | A white panel on every learner page | Readability over the background drawing, decision of the project | [005](docs/specs/005-ui-theme-navigation.md) |
| Dark mode cards | Translucent, blurred cards over `--color-page` | Solid `--color-paper` cards everywhere, like the course page's module rows | Decision of the project | [005](docs/specs/005-ui-theme-navigation.md) |
| Guided tour | None | A tour of the learner app led by the bird of the logo and a first lesson tip, state kept in the browser | New feature of pupitre2 | [007](docs/specs/007-guided-tour.md) |

## To do

| Item | Status | Spec | Notes |
| --- | --- | --- | --- |
| Desktop app (`genesary/pupitre-desktop`) against pupitre2 | ⬜ | [014](docs/specs/014-labs.md), [099](docs/specs/099-migration-plan.md) | The local lab check is kept and compatible; checking it with the app is the next batch |
| Switch-over of a running genesary installation | ⬜ | [099](docs/specs/099-migration-plan.md), [199](docs/specs/199-backend-migration-plan.md) | Procedure ready and tried on a test cluster (pupitre2-helm `README.md`) |
| Dark mode of `.mod-row` and the glass cards | 🟡 | [099](docs/specs/099-migration-plan.md) | genesary's CSS has no dark variant for them: kept as is until decided |
| Course thumbnail, path level | ➖ | [190](docs/specs/190-backend-defects.md) B18 | Not in genesary's API: new features, out of the scope of the move |
