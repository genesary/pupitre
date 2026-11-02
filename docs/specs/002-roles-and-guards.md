# 002 - Roles, page guards and role-dependent UI

Feature: F001.

## Roles

| Role | Value | What it sees in addition |
| --- | --- | --- |
| Learner | `student` | Learner space |
| Manager | `manager` | "Mon espace" (`/manager/*`), session creation in the agenda, attendance, CSV exports |
| Admin | `admin` | Administration (`/admin/*`), session creation, attendance, CSV exports |

The role comes from `GET /api/auth/me` (see [001](001-keycloak-authentication.md)). **The backend
stays the only authority**: frontend guards guide the user, they do not protect data. A hidden
page must also be refused by the API.

## Guards

Guards are route components (`RequireAuth`, `RequireRole`) applied in `router.tsx`, not code
copied into each page as it is today.

| Guard | No session | Session, wrong role |
| --- | --- | --- |
| `RequireAuth` | Redirect to `/login?redirect=<path>` | - |
| `RequireRole(['admin'])` | Same | "Access denied" page with a « Retour au catalogue » button |
| `RequireRole(['manager'])` | Same | Same |
| `RequireRole(['admin','manager'])` | Same | Same |

## Route table

| Route | Access | Spec |
| --- | --- | --- |
| `/login`, `/auth/callback` | Public | [001](001-keycloak-authentication.md) |
| `/courses` | Public (enriched when signed in) | [010](010-catalogue.md) |
| `/courses/:slug` | Public (modules and enrollment when signed in) | [011](011-course-page-and-enrollment.md) |
| `/courses/:slug/lessons/:index` | Signed in | [012](012-lesson-player.md) |
| `/courses/:slug/quiz/:index` | Signed in | [013](013-quizzes.md) |
| `/leaderboard`, `/users/:id` | Public | [020](020-learner-space.md) |
| `/skills/:slug` | Signed in | [020](020-learner-space.md) |
| `/my-courses`, `/my-paths`, `/my-skills`, `/my-badges`, `/my-xp`, `/my-groups` | Signed in | [020](020-learner-space.md) |
| `/profile` | Signed in | [021](021-profile.md) |
| `/agenda` | Signed in | [022](022-agenda-and-sessions.md) |
| `/manager/users`, `/manager/groups`, `/manager/paths` | `manager` | [030](030-manager-space.md) |
| `/manager` | `manager` → redirects to `/manager/users` | |
| `/admin` | `admin` | [040](040-admin-dashboard.md) |
| `/admin/monitoring`, `/admin/leaderboard` | `admin` | [040](040-admin-dashboard.md) |
| `/admin/users` | `admin` | [041](041-admin-users.md) |
| `/admin/courses` | `admin` | [042](042-admin-courses.md) |
| `/admin/courses/sessions/:slug/:sessionId` | `admin`, `manager` | [022](022-agenda-and-sessions.md) |
| `/admin/paths` | `admin` | [043](043-admin-paths.md) |
| `/admin/groups` | `admin` | [044](044-admin-groups.md) |
| `/admin/patterns` | `admin` | [045](045-admin-patterns.md) |
| `/admin/settings` | `admin` | [046](046-admin-settings.md) |
| `/admin/exports` | `admin`, `manager` | [047](047-admin-exports-and-labs.md) |
| `/admin/labs` | `admin` | [047](047-admin-exports-and-labs.md) |
| Any other route | - | 404 page « Page introuvable » with a link to the catalogue |

The current frontend has a catch-all route `/admin/[...slug]` that shows an old dashboard: it is
replaced by the 404 page.

## Role-dependent UI

- Top bar: the "Mon espace" link appears for a `manager`, the "Admin" button for an `admin`.
- Agenda: the session create, edit and delete buttons appear for `admin` and `manager`
  ([022](022-agenda-and-sessions.md)).
- The attendance page of a session calls `/api/manager/session-bookings/…` for a manager and
  `/api/admin/session-bookings/…` for an admin.
- An admin sees the hidden modules of a course and the correct quiz answers: the backend returns
  them, the frontend only displays what it receives.

## Acceptance criteria

- A learner who opens `/admin/users` sees "Access denied", and no `/api/admin/*` call is sent.
- A manager can open `/admin/exports` but not `/admin/users`.
- An admin has no access to `/manager/*` (current behaviour, kept: the manager space depends on
  groups the admin is not necessarily a member of).
- A visitor who is not signed in and opens `/my-courses` lands on
  `/login?redirect=%2Fmy-courses`, then comes back to `/my-courses` after signing in.

## Sources

`frontend/src/layouts/Admin.astro`, `layouts/Manager.astro`, `layouts/App.astro`, the guards at
the top of each `pages/my-*.astro` page; `internal/middleware/auth.go` (server-side roles).
