# pupitre2 specs

This folder describes the **new React frontend** of pupitre. It replaces the Astro frontend of
[`genesary/pupitre`](https://github.com/genesary/pupitre) (`frontend/`) **without changing the Go
backend**: the specs describe what the interface must do and which routes of the existing API it
calls.

Specs 100 to 199 describe the **backend**: the Go services of `genesary/pupitre` move into
`apps/api` as one application (a modular monolith), keeping every route, table and rule.

## Sources

- **Current behaviour**: the Astro frontend code (`frontend/src/pages`, `layouts`, `lib`) at
  commit `3cdf7b2` of `genesary/pupitre`. Every spec cites its source files.
- **API contract**: the Go routers (`user-service/internal/handlers/router.go`,
  `course-service/internal/handlers/router.go`) and the response structs of the handlers. The
  `openapi.yaml` files are not used as a reference: they cover only about a quarter of the
  routes.
- **Reference rewrite**: [`ebuildy/pupitre2`](https://github.com/ebuildy/pupitre2), a full
  rewrite with its own TypeScript backend. Its specs were used as a checklist so that no feature
  is forgotten, and its **frontend code may be reused** with its author's agreement (see
  "Reusing the reference frontend" in [090](090-gaps-and-defects.md)). Its backend is not: pupitre2
  keeps the existing Go backend.
- **Security audit** of 2026-10-20: its frontend requirements are in [003](003-security.md).

Coverage of every page and service of `genesary/pupitre` is tracked in [`parity.md`](../../parity.md).

## Conventions

- Specs are written in English. **UI strings stay in French**, because the interface is in
  French: they are quoted as they appear on screen (« Catalogue », "Mes cours").
- `Fxxx` identifiers refer to the platform feature list (same numbering as the reference
  checklist). They are used to track coverage.
- "Must" is a requirement, "may" is optional.
- API routes are written without the host, always under `/api` (see [004](004-api-and-routing.md)).
- Dates and numbers are formatted with the `fr-FR` locale.

## Index

| Spec | Topic | Features |
| --- | --- | --- |
| [000](000-scope-and-stack.md) | Scope, stack and repository conventions | - |
| [001](001-keycloak-authentication.md) | OIDC authentication through Keycloak, session, logout, landing page | F002, F003, F023-F028, F034 |
| [002](002-roles-and-guards.md) | Roles, page guards, role-dependent UI | F001 |
| [003](003-security.md) | Frontend security requirements | - |
| [004](004-api-and-routing.md) | API client, routing to the services, errors, route inventory | F004, F005 |
| [006](006-mock-mode.md) | Mock mode: running and testing the frontend without the backend | - |
| [005](005-ui-theme-navigation.md) | Theme, dark mode, navigation, toasts, page states, accessibility | F010-F012, F018 |
| [007](007-guided-tour.md) | Guided tour of the learner app, led by the bird of the logo (new) | F186-F189 |
| [010](010-catalogue.md) | Catalogue, filters, course card, "Reprendre" | F040-F044 |
| [011](011-course-page-and-enrollment.md) | Course page, enrollment, prerequisites, badge, sessions | F045-F050 |
| [012](012-lesson-player.md) | Lesson player, markdown rendering, media, progress | F053-F063, F100 |
| [013](013-quizzes.md) | Quizzes (quiz page and inline quiz) | F075-F085 |
| [014](014-labs.md) | Labs: GitLab checks, local check (desktop app) | F090-F096 |
| [020](020-learner-space.md) | My courses, paths, skills, badges, XP, groups, leaderboard, public profile | F037, F052, F103-F114, F123, F136 |
| [021](021-profile.md) | Profile, avatar, bio | F035, F036 |
| [022](022-agenda-and-sessions.md) | Agenda, in-person sessions, booking, attendance | F150-F155 |
| [030](030-manager-space.md) | Manager space | F140-F143 |
| [040](040-admin-dashboard.md) | Admin: dashboard, monitoring, leaderboard | F115, F160, F161, F184 |
| [041](041-admin-users.md) | Admin: users | F162, F163 |
| [042](042-admin-courses.md) | Admin: courses, definition, markdown import / export, enrollments, cache | F145, F165-F177, F185 |
| [043](043-admin-paths.md) | Admin: learning paths | F126, F127 |
| [044](044-admin-groups.md) | Admin: groups, subgroups, role mappings | F130-F135 |
| [045](045-admin-patterns.md) | Admin: markdown patterns | F180, F181 |
| [046](046-admin-settings.md) | Admin: platform settings | F179 |
| [047](047-admin-exports-and-labs.md) | Admin: CSV exports, lab tracking | F095, F182, F183 |
| [090](090-gaps-and-defects.md) | Differences from the reference, defects of the current frontend | - |
| [099](099-migration-plan.md) | Migration plan in batches | - |

### Backend

| Spec | Topic |
| --- | --- |
| [100](100-backend-scope-and-architecture.md) | Scope, why one application, target layout, invariants |
| [101](101-backend-configuration.md) | Merged configuration: keys, defaults, removed keys |
| [102](102-backend-data-and-migrations.md) | Tables, migrations, start-up seeding, connection pool |
| [103](103-backend-http-api.md) | Router, middleware, body limits, inventory of every route |
| [104](104-backend-in-process-calls.md) | Calls between services that become function calls |
| [105](105-backend-authentication.md) | Tokens, guards, OIDC, OAuth, LDAP, local accounts |
| [106](106-backend-courses-and-content.md) | Courses, modules, git, markdown import, quizzes, progress, XP, skills, paths |
| [107](107-backend-labs-and-checker.md) | Labs, the checker, OPA limits |
| [108](108-backend-operations-and-testing.md) | Tooling, image, deployment, tests, CI |
| [190](190-backend-defects.md) | Known backend defects, kept during the move and fixed after |
| [199](199-backend-migration-plan.md) | Backend migration plan in batches |

## Out of scope

- For the frontend specs (000-099), the backend API is the one of `3cdf7b2`: the move of specs
  100-199 keeps it unchanged. Server-side fixes are listed in [190](190-backend-defects.md).
- Local registration, local login, GitHub and LDAP: the platform signs in only with OIDC through
  Keycloak (see [001](001-keycloak-authentication.md)).
