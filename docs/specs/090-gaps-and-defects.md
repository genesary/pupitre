# 090 - Differences from the reference and defects of the current frontend

## What these specs add compared with the `ebuildy/pupitre2` reference

The reference describes a full rewrite (TypeScript backend, two separate React applications,
server-side sessions). pupitre2 only rewrites the frontend. The specs therefore add:

| Topic | Reference | pupitre2 |
| --- | --- | --- |
| Backend | Rewritten in TypeScript (routes `/api/me`, `/api/sessions`…) | **Unchanged**: the specs call the existing Go routes ([004](004-api-and-routing.md)) |
| Applications | Two applications (learner and admin) | **Two** applications too, on one origin: the learner app, and the admin app (administration and manager space) under `/admin` and `/manager`, sharing `@pupitre/core` |
| Authentication | Local login, OAuth, OIDC, LDAP | **Keycloak OIDC only** ([001](001-keycloak-authentication.md)), plus the backend settings to turn off (H7) |
| Session | Server-side session cookie | The backend JWT, isolated in one module so it can move to a cookie later |
| Security | Fixes built into the new backend | Explicit frontend requirements from the audit ([003](003-security.md)): sanitization, avatar, redirects, CSP |
| Desktop app | Out of scope | Tauri local check kept and compatible ([014](014-labs.md)) |
| Languages | French and English | French only, with strings grouped for a future translation |
| API contract | Generated OpenAPI | Types written by hand from the Go structs, as the current OpenAPI is incomplete |
| Guided tour | None | A tour of the learner app led by the bird of the logo, state kept in the browser ([007](007-guided-tour.md)) |

The reference features that belong to its backend (admin account bootstrap, demo data,
server-side performance invariants, health and metrics, rate limiting) do not apply to a frontend
and are not carried over.

## Reusing the reference frontend

The reference frontend uses the same stack (React 19, Vite, React Router, Tailwind v4, shadcn/ui,
TanStack Query), and its author agreed that it be reused. Its code is taken over where it helps,
with these rules:

| Part of the reference | Reuse |
| --- | --- |
| `packages/ui` (shadcn components, theme, page blocks) | As is, adjusted to our tokens ([005](005-ui-theme-navigation.md)) |
| `apps/app`, `apps/admin` pages and components | The interface (JSX, layout, wording); the data layer is rewritten |
| `packages/i18n` French strings | The wording; French only, as specified |
| `packages/auth-ui` (better-auth, local login, registration) | Not reused: Keycloak only ([001](001-keycloak-authentication.md)) |
| `packages/shared` (types of its API) | Not reused: types mirror the Go structs ([004](004-api-and-routing.md)) |
| `apps/api` (TypeScript backend) | Not reused: the Go backend stays |

When a reference component is taken over:

1. **Data**: its Hono RPC calls (`api.….$get()`, `unwrap()`) are replaced by our API client and the
   Go routes of [004](004-api-and-routing.md). Field names follow the Go JSON tags.
2. **HTML**: the reference backend renders and sanitizes markdown on the server, so its components
   inject HTML directly (`dangerouslySetInnerHTML` in the lesson, quiz and lab components). The Go
   backend returns **raw** markdown: every such place goes through `renderMarkdown()` and
   `<SafeHtml>` instead ([003](003-security.md) S1, S2). Copying them unchanged would bring back
   findings C2 and C3.
3. **Two applications**: a reused learner page goes to `apps/app`, an admin page to `apps/admin`;
   the manager space is in the admin app here ([002](002-roles-and-guards.md)).
4. **Mock data**: every route a reused page calls gets its handler in `packages/core/src/mocks`
   ([006](006-mock-mode.md)).
5. **Attribution**: a reused file keeps a header comment naming its origin
   (`// Adapted from ebuildy/pupitre2: apps/app/src/routes/Catalogue.tsx`).
6. **Design**: the reference is used for structure and behaviour, **never for its look**. Its
   theme (`packages/ui/src/styles/globals.css`: Inter font, oklch colours, 12 px radius) is not
   imported. Every reused screen is restyled to match the current pupitre design
   ([005](005-ui-theme-navigation.md), "Visual design").

## Defects of the current frontend

Found while reading `genesary/pupitre/frontend` (commit `3cdf7b2`). Each one is fixed by the spec
indicated.

| # | Defect | Where | Fix |
| --- | --- | --- | --- |
| A1 | Patterns never apply in lessons: `GET /api/patterns` is called **without a token**, while the route requires a signed-in user (401) | `lessons/[index].astro`, `quiz/[index].astro` | Call through the authenticated client ([012](012-lesson-player.md)) |
| A2 | The course `thumbnail` field is read everywhere, but the API does not return it | `courses/index.astro`, `courses/[slug]/index.astro`, `my-courses.astro` | Icon thumbnail ([010](010-catalogue.md)); a real thumbnail needs a backend field |
| A3 | The `total_labs`, `total_submissions` and `success_rate` statistics are always 0 (hard-coded in `AdminStats`) | `admin/index.astro` | Not shown ([040](040-admin-dashboard.md)) |
| A4 | Two different admin sidebars (`Admin.astro` without Patterns, Settings or Monitoring; the old dashboard without Paths, Labs or Exports) | `layouts/Admin.astro`, `admin/[...slug].astro` | A single list ([005](005-ui-theme-navigation.md)) |
| A5 | The « Quiz ref » field of the module editor does not exist in the definition: what is entered is lost | `admin/courses/index.astro` | Removed ([042](042-admin-courses.md)) |
| A6 | The client `role` type does not know `manager`; some lab types and one route no longer exist in the API | `lib/api.ts` | Types rebuilt from the Go structs ([004](004-api-and-routing.md)) |
| A7 | 401 is handled in three ways: redirect to `/`, redirect to `/login`, and `localStorage.clear()`, which also wipes the theme | every page | A single handling ([001](001-keycloak-authentication.md)) |
| A8 | An enrollment failure is ignored, then the page reloads | `courses/[slug]/index.astro` | Error toast ([011](011-course-page-and-enrollment.md)) |
| A9 | A lesson's breadcrumb shows the course slug instead of its title | `lessons/[index].astro` | Title ([012](012-lesson-player.md)) |
| A10 | A skill's level is recomputed in the frontend, while the server returns it | `my-skills.astro` | Server value ([020](020-learner-space.md)) |
| A11 | The 200 XP per course and the level thresholds are hard-coded in the frontend | `courses/index.astro`, `courses/[slug]/index.astro`, `my-xp.astro` | Kept, but centralized in one constant until the API exposes them |
| A12 | `courseReturnTo` stays in `localStorage` if the learner leaves the course: it can hijack the end of another course | `my-paths.astro`, `lessons/[index].astro` | `sessionStorage`, cleared once used ([012](012-lesson-player.md)) |
| A13 | The pattern form says the JS is "executed after rendering", but it is executed nowhere | `admin/patterns/index.astro` | Label « non exécuté » ([045](045-admin-patterns.md)) |
| A14 | After an SSO callback, the `redirect` parameter is lost; after a local login, it is ignored for an admin | `auth/callback.astro`, `login.astro` | Destination stored and validated for every role ([001](001-keycloak-authentication.md)) |
| A15 | A `/admin/[...slug]` route catches every unknown admin URL and shows an old dashboard | `admin/[...slug].astro` | 404 page ([002](002-roles-and-guards.md)) |
| A16 | The code that marked a lab as done when it was opened (`a[data-lab-slug]`) never runs: no link carries this attribute | `courses/[slug]/index.astro` | Not carried over |
| A17 | The names `labCount` and `completedLabs` actually mean the number of modules and of completed lessons | API, several pages | Names kept in the types, with a comment |
| A18 | The path level badge reads `level`, which `GET /api/my/paths` does not return: it never shows | `my-paths.astro` | Not shown; a backend field is needed ([020](020-learner-space.md)) |
| A19 | The session form requires a capacity of at least 1, while 0 means unlimited | `agenda.astro` | Capacity ≥ 0 ([022](022-agenda-and-sessions.md)) |
| A20 | Editing a session fills the date field with the UTC time cut from the ISO string: saving it unchanged moves the session by the time zone offset | `agenda.astro` | Local time in the field ([022](022-agenda-and-sessions.md)) |
| A21 | The group filter of « Mes membres » never works: the buttons have the class `mu-group-filter-btn` but the script listens on `.group-filter-btn` | `manager/users/index.astro` | Working filter ([030](030-manager-space.md)) |
| A22 | `GET /api/manager/users/{id}/path-enrollments` returns `Slug` and `EnrolledAt`: `PathEnrollmentRow` has no json tags; the same goes for `GroupRow` in `GET /api/admin/groups/{id}/descendants` | API (`repository/path_enrollment.go`, `repository/group.go`) | Field names kept as sent; descendants computed from the group list ([044](044-admin-groups.md)); json tags are a backend fix |
| A23 | « Recalculer la progression » posts to `/api/admin/sync-progress` without a body, while the route marks one lesson for one user and requires `userId`, `courseSlug`, `lessonSlug`: it always answers 400 | `admin/monitoring`, `admin/leaderboard` | Button removed ([040](040-admin-dashboard.md)) |
| A24 | Saving the settings sends every key back, the masked secrets (`********`) included, which overwrites the real secrets, and keys the server refuses | `admin/settings/index.astro` | Changed keys of one section only, secrets only when retyped ([046](046-admin-settings.md)) |
| A25 | The pattern label is optional in the form, but the server refuses a creation without it (400) | `admin/patterns/index.astro` | The name stands in for an empty label ([045](045-admin-patterns.md)) |
| A26 | In dark mode, some elements keep their light background while their text switches to the light dark-mode ink, and the text disappears: the agenda (calendar, top bar, « Sessions à venir »), the module rows and in-person session cards of the course page, the « Tout » chip of the catalogue, the members table of « Mes membres »; the violet links of the admin lists and the settings info box are hard to read too | `styles/learning.css`, `styles/admin.css` (no dark override for `.cal-area`, `.mod-row`, `.session-card-*`, `.chip-reset`, `.mu-table-wrap`…) | Dark overrides in `packages/ui/src/styles/contrast-fixes.css`, the pupitre copies unchanged; AA contrast checked by `e2e/contrast.spec.ts` ([005](005-ui-theme-navigation.md)) |
| A27 | A locked module is dimmed to 50 % (55 % in the lesson sidebar): on the course page, after the first module, the names and the « Verrouillé » / type badges fall to about 2:1, in both modes; the current module of a sidebar is blue on light blue (3.2:1) | `styles/learning.css` (`.mod-row.locked`), `styles/components.css` (`.sidebar-link.locked`, `.sidebar-link.active`) | Locked rows at full opacity with a dashed border, only the lock icon faded; a darker blue for the current page; in `contrast-fixes.css`, checked by `e2e/contrast.spec.ts` |
| A28 | On the course page, the « XP de ce cours » card is a link with no `display`: it stays inline, so its yellow background and border break into a strip at the left and its bar runs into the « Continuer le cours » button | `styles/learning.css` (`.course-xp-card`) | `display: block` in `apps/app/src/index.css` |

## Backend defects found while writing the frontend

Not worked around in the frontend ([`AGENTS.md`](../../AGENTS.md): the Go backend's API is not
changed). They are tracked, with the audit's server-side findings, in
[190](190-backend-defects.md).

| # | Defect | Where | Effect |
| --- | --- | --- | --- |
| B1 | A manager who creates a subgroup is not made a member of it, while the scope is the groups the manager belongs to | `user-service` `ManagerCreateSubgroup` | The new subgroup disappears from « Mes groupes » and can no longer be managed by its creator |

## Security

Audit findings C2, C3, C4 and H5 (issue genesary/pupitre#194) come from the current frontend.
They are covered by [003](003-security.md) and [001](001-keycloak-authentication.md).
