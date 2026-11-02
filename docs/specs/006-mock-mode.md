# 006 - Mock mode

The frontend can run and be tested without the Go backend or Keycloak. The mock mode answers
`/api/*` in the browser with [MSW](https://mswjs.io/) (Mock Service Worker), using response
shapes copied from the Go structs.

## Commands

| Command | Backend |
| --- | --- |
| `pnpm dev:mock` | None: MSW answers every `/api/*` call in the browser |
| `pnpm dev` | The real services, through the Vite proxy ([004](004-api-and-routing.md)) |
| `pnpm test` | None: tests mock the API modules or use the MSW handlers |

## What is simulated

- `GET /api/settings/public`: `oidc_enabled` is `"true"`, local login and registration are
  `"false"`.
- **Keycloak**: `GET /api/auth/oidc/authorize` returns a URL to `/mock-keycloak`, a page that
  lists test accounts. Choosing one goes back to `/auth/callback` with a `code` and the same
  `state`, as Keycloak does, so the real callback code runs (state check included). « Annuler la
  connexion » returns `error=access_denied`.
- `POST /api/auth/oidc/callback`: checks that the state was issued and not used yet, then
  returns an unsigned JWT (valid 8 hours) and the user.
- `GET /api/auth/me`: reads the JWT and returns the user, or 401 once it has expired.
- **Learner flow (batch 1)**, in `packages/core/src/mocks/learner-handlers.ts` and `course-logic.ts`, which mirror
  the Go handlers:
  - catalogue and course: `GET /api/courses`, `GET /api/courses/{slug}` (private courses only for
    enrolled users and admins);
  - modules: list with the sequential and prerequisite locking of `ListModules`, detail with the
    inline quiz of the next module, automatic enrolment in public courses, 403 when course
    prerequisites are not met;
  - quizzes: scoring of the four question types (no partial scoring), exponential cooldowns,
    lock after the maximum attempts, 425 while a cooldown runs;
  - lessons: `…/lessons/{slug}/complete`, badge awarded once every module is done;
  - labs: step and module checks fail on the first try and pass on the second, so both outcomes
    can be seen; `…/record` echoes the result;
  - user-service: `GET /api/my/courses`, `/my/skills`, `/my/badges`, `/my/session-bookings`,
    enrolment (with the XP gate on the XP earned), session booking (409 once a limited session is
    full, then 204), `GET /api/patterns`.
- **Learner space (batch 2)**, in `packages/core/src/mocks/space-handlers.ts`: `GET /api/my/paths`
  (course and skill paths, with the `completed` / `available` / `locked` statuses of `paths.go`),
  `/my/skills/{slug}`, `/my/xp` (10 XP per lesson, 25 per passed quiz, 200 per completed course,
  awarded once), `/my/groups`, the public `GET /api/leaderboard`, `/users/{id}`,
  `/users/{id}/badges`, `/badges/{slug}`, `PUT /api/auth/profile` (bio and avatar), and the session
  routes of admins and managers (`POST`, `PUT`, `DELETE /api/admin/courses/{slug}/sessions…`).
- **Manager space (batch 3)**, in `packages/core/src/mocks/manager-handlers.ts`: `GET /api/paths`, every
  `/api/manager/*` route of [030](030-manager-space.md) with the scope of `manager.go` (members of
  the manager's groups, « everyone » excepted), attendance under `/api/admin/…` and
  `/api/manager/…`, and the CSV exports: « Utilisateurs » and « Inscriptions aux cours » for admins
  and managers, « Lab Checks » for admins only (a UTF-8 CSV with BOM, `;` separator and the
  server's file name).
- **Administration (batch 4)**, in `packages/core/src/mocks/admin-handlers.ts`: every `/api/admin/*`
  route of [040](040-admin-dashboard.md) to [047](047-admin-exports-and-labs.md). Courses, paths,
  patterns, settings, groups, mappings and user changes are written to the mock state, so they
  show up on the learner pages too; `GET /api/settings/public` follows the saved settings. The
  markdown import splits on the chosen heading level; « Suivi des labs » lists fixtures plus the checks learners run in mock mode.
- **Fixtures**: the 17 demo courses and 2 paths of the Go seed (`fixtures/seed-courses.ts`,
  generated from `course-service/internal/db/seed`), plus `fixtures/demo-courses.ts`, which covers
  what the seed does not (admonitions, mermaid, patterns, trap HTML, media, every question type,
  GitLab lab, in-person sessions, XP gate, private course, a skill path), and two patterns.
- **State**: enrolments, viewed lessons, quiz results, cooldowns, bookings, badges, XP, path
  enrolments, profile changes and edited sessions are kept in `sessionStorage` (`pupitre.mock.db`):
  they survive reloads and reset with the tab. `alice` starts enrolled in three courses (one
  finished, with its badge and XP) and two paths, and belongs to two groups; `admin` and the trap
  account hold badges too, so the leaderboard shows the trap account's payloads. `marc.manager`
  manages « Promo DevOps 2026 » (alice and the trap account) and its subgroup « Groupe A »; the
  trap account and `admin` have booked the next Paris session, so its attendance page has
  participants.
- Any other `/api/*` route answers **501** « Route non simulée en mode mock », until the batch
  that needs it adds its handler.

## Test accounts

| Account | Role | Purpose |
| --- | --- | --- |
| `alice` | `student` | Learner flows |
| `marc.manager` | `manager` | Manager space, exports, attendance |
| `admin` | `admin` | Administration |
| `<img src=x onerror=alert("xss-username")>` | `student` | Security check: its username, avatar and bio carry the C2 and C4 payloads of the audit. **Any alert box is a regression.** |

## Rules

- Mock code lives in `packages/core/src/mocks/` and is loaded only when `import.meta.env.MODE === 'mock'`: it
  is not in a production build, and `apps/app/public/mockServiceWorker.js` is removed from `dist/`.
  The admin app registers the same worker script (same origin) with the same handlers, and shares
  the mock state (sessionStorage).
- Handlers follow the Go routes and the `{ "error": "…" }` envelope. A page built against the
  mock must work unchanged against the real backend.
- Each batch of [099](099-migration-plan.md) adds the handlers and fixtures of its routes.
- The mock is not a second backend: it carries no business rule beyond what a page needs to be
  shown and tested.
