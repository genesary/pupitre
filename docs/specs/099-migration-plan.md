# 099 - Migration plan in batches

Each batch is a `feature/…` branch and a pull request that can ship on its own. The Astro
frontend stays in service until the end of batch 5; the reverse proxy then switches to pupitre2.

## Status

| Batch | State |
| --- | --- |
| 0 - Foundation | Done, merged into `main` (squash). |
| 1 - Main learner flow | Done, merged into `main` (squash): catalogue, course page and enrollment, lesson player (markdown, admonitions, patterns, mermaid), inline and full quizzes with cooldowns, GitLab and local labs, sessions booking, mock handlers. The repository was also restructured as a monorepo (`apps/app`, `packages/ui`) like `ebuildy/pupitre2`. Verified in mock mode, light and dark. |
| 2 - Learner space | Done, merged into `main` (squash): my courses, paths (SVG progress map), skills and skill page, badges, XP, groups, leaderboard, public profile, profile (bio, emoji avatar), agenda with booking and session management, mock handlers. Verified in mock mode, light and dark. The attendance page stays in batch 3. |
| 3 - Manager space | Done, merged into `main` (squash): members (group filter, search, course and path enrollments), groups (tree, create, subgroup, delete, members), paths, session attendance (optimistic, admin or manager routes), CSV exports (both sources, manager notice), mock handlers. Verified in mock mode. |
| 4 - Administration | Done, merged into `main` (squash): dashboard, monitoring, score leaderboard, users, courses (definition editor with a stale-write check, modules and reordering, markdown import and export, enrollments, git cache), paths, groups and role mappings, patterns with live preview, settings (per-section save, secret rule, Keycloak warnings), lab tracking, mock handlers. Verified in mock mode, light and dark. |
| 5 - Switch-over | In progress. Done on `feature/e2e-tests`: Playwright tests of the main flows against the mock mode (`pnpm test:e2e`), and the split of the frontend into a learner app and an admin app (`apps/admin`, with the manager space) sharing `packages/core`. Next for the frontend: the headers and CSP of the served build. **Postponed until after the backend work**: the desktop app check and the switch-over itself (they change `genesary/pupitre`). |

Open points:

- The original pupitre CSS has no dark-mode variant for `.mod-row` and the glass cards: they stay
  light in dark mode. Kept as is (design unchanged) until the user decides whether to fix it.
- Context: about 1,000 users; the repository is public; pull requests are squash merged.

## Batch 0 - Foundation

Branch `feature/react-frontend` (this branch).

- Specs (this folder).
- Vite + React + TypeScript project, pnpm, Tailwind v4, shadcn/ui, ESLint, Vitest.
- Theme, local fonts, dark mode ([005](005-ui-theme-navigation.md)).
- API client, types, dev proxy ([004](004-api-and-routing.md)).
- Tested security utilities: `safeRedirect`, `safeUrl`, `sanitizeHtml`, `<Avatar>`
  ([003](003-security.md)).
- Session, Keycloak sign-in, callback, logout, guards ([001](001-keycloak-authentication.md),
  [002](002-roles-and-guards.md)).
- Top bar, side menu, 404 and "Access denied" pages.

**Done when** a user can sign in with Keycloak, see their avatar in the top bar and sign out, and
the security tests of [003](003-security.md) pass.

## Batch 1 - Main learner flow

Catalogue ([010](010-catalogue.md)), course page and enrollment
([011](011-course-page-and-enrollment.md)), lesson player and markdown pipeline
([012](012-lesson-player.md)), quizzes ([013](013-quizzes.md)), labs ([014](014-labs.md)).

**Done when** a learner can follow a course end to end (lesson, inline quiz, quiz, lab), and the
progress matches the current frontend on the same data.

## Batch 2 - Learner space

My courses, paths, skills, badges, XP, groups, leaderboard, public profile
([020](020-learner-space.md)); profile ([021](021-profile.md)); agenda and sessions
([022](022-agenda-and-sessions.md)).

## Batch 3 - Manager space

[030](030-manager-space.md), attendance page ([022](022-agenda-and-sessions.md)), exports for
managers ([047](047-admin-exports-and-labs.md)).

## Batch 4 - Administration

Dashboard, monitoring, leaderboard ([040](040-admin-dashboard.md)), users
([041](041-admin-users.md)), courses and markdown import ([042](042-admin-courses.md)), learning
paths ([043](043-admin-paths.md)), groups ([044](044-admin-groups.md)), patterns
([045](045-admin-patterns.md)), settings ([046](046-admin-settings.md)), exports and labs
([047](047-admin-exports-and-labs.md)).

## Batch 5 - Switch-over

- Playwright tests of the main flows (sign-in, following a course, quiz, a manager enrolling a
  member, an admin creating a course), in `e2e/`, run with `pnpm test:e2e` against the mock mode,
  plus guards, XSS, theme and phone drawer, in-person sessions, patterns, learning paths and GitLab
  labs (scenarios taken from the `ebuildy/pupitre2` end-to-end suite where they apply to the
  frontend).
- Reverse proxy: the routing table of [004](004-api-and-routing.md), the headers and CSP of
  [003](003-security.md).
- Desktop app: check that it embeds the new build and that the local check works.
- Switch-over, then removal of the Astro frontend from `genesary/pupitre`.

## In parallel, on the backend (`genesary/pupitre`)

Not blocking for the batches, but to be done before opening the platform:

1. Turn off registration, local login and username changes (H7): settings only, no code.
2. Refuse default secrets at start-up (C1).
3. Validate `avatarUrl` server side (C4).
4. Manager scope (H2), token revocation (H3).
5. Enable private vulnerability reporting and answer issue genesary/pupitre#193.
