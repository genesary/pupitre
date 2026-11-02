# 004 - API client, routing to the services and route inventory

Features: F004 (API routing, single hostname), F005 (HTTP conventions).

## Single origin

The browser calls only `/api/*` on the application origin. The reverse proxy (production) or the
Vite proxy (development) sends all of `/api` to the backend, which is one application
(`pupitre-api`, [100](100-backend-scope-and-architecture.md)). `/uploads/{filename}` (course media)
is served by the same application **outside** `/api`: the proxy also sends `/uploads/*` to it.

Until the backend became one application, `/api` was split between user-service and
course-service by prefix (`courses`, `paths`, `skills`, `batch`, `uploads`, `admin/courses`,
`admin/cache`, `admin/lab-checks`, `admin/exports/lab-checks` to course-service). The split is
gone: the route inventory below still names the service each route came from.

The pages are routed the same way: `/admin` and `/admin/*` (the admin app's base is `/admin/`:
`/admin` alone must get the trailing slash) and `/manager/*` go to the admin app's `index.html` and
assets, every other page to the learner app's ([000](000-scope-and-stack.md)).

The current Astro proxy (`pages/api/[...path].ts`) forwards **every** client header, which is
finding H4. In pupitre2, the dev proxy forwards only `Authorization`, `Content-Type`, `Accept`
and `Cookie` (the API's only cookie is the OAuth state, [105](105-backend-authentication.md)),
and the production reverse proxy must do the same.

## HTTP client

A single client (`packages/core/src/lib/api-client.ts`):

- `/api` prefix, JSON in and out;
- adds `Authorization: Bearer <token>` when a session exists ([001](001-keycloak-authentication.md));
- on error, reads the `{ "error": "message" }` envelope and throws an `ApiError` (`status`,
  `message`, `body`);
- 401 on an authenticated call → session ends and redirect (see [001](001-keycloak-authentication.md));
- 403 → an `ApiError` handled by the page (prerequisites not met, access denied…);
- non-JSON responses (CSV, exported markdown) go through `requestBlob()`.

Each route is exposed by a typed function in `packages/core/src/lib/api.ts`, used through TanStack Query
hooks (`useQuery` for reads, `useMutation` with invalidation for writes). No page calls `fetch`
directly.

Types mirror the Go structs (JSON field names are the ones in the `json:"…"` tags). As the
OpenAPI documents are incomplete, the types are written by hand in `packages/core/src/lib/api-types.ts`, and each
type cites the Go handler it comes from.

## Routes used by the frontend

### user-service

| Method | Route | Use | Spec |
| --- | --- | --- | --- |
| GET | `/api/settings/public` | Public settings (`oidc_enabled`…) | 001 |
| GET | `/api/auth/oidc/authorize` | Keycloak authorization URL | 001 |
| POST | `/api/auth/oidc/callback` | Code exchange | 001 |
| GET | `/api/auth/me` | Current user | 001 |
| PUT | `/api/auth/profile` | Bio, avatar | 021 |
| POST, DELETE | `/api/enrollments/{slug}` | Enroll, unenroll | 011 |
| GET | `/api/my/courses` | My courses | 010, 011, 020 |
| GET | `/api/my/paths` | My learning paths | 020 |
| GET | `/api/my/badges` | My badges | 011, 020, 021 |
| GET | `/api/my/xp` | My XP (`total`, `level`, `levelName`, `history`) | 020 |
| GET | `/api/my/skills`, `/api/my/skills/{slug}` | My skills | 010, 020 |
| GET | `/api/my/groups` | My groups | 020 |
| GET | `/api/my/session-bookings` | My bookings | 011, 022 |
| POST, DELETE | `/api/session-bookings/{slug}/{sessionId}` | Book, cancel | 011, 022 |
| GET | `/api/session-bookings/{slug}/{sessionId}/count` | Booking count | 022 |
| GET | `/api/users/{id}`, `/api/users/{id}/badges` | Public profile | 020 |
| GET | `/api/leaderboard` | Public leaderboard | 020 |
| GET | `/api/badges/{slug}` | Badge statistics | 020 |
| GET | `/api/patterns` | Markdown patterns (signed in) | 012 |
| * | `/api/manager/*` | Manager space | 030 |
| * | `/api/admin/{settings,stats,leaderboard,users,enrollments,paths,groups,patterns,sync-progress,session-bookings}` | Administration | 04x |
| * | `/api/admin/exports/{categories,preview,download}` | CSV exports (manager or admin) | 047 |

### course-service

| Method | Route | Use | Spec |
| --- | --- | --- | --- |
| GET | `/api/courses` | Catalogue | 010 |
| GET | `/api/courses/{slug}` | Course details | 011 |
| GET | `/api/paths`, `/api/paths/{slug}` | Public learning paths | 020, 030 |
| GET | `/api/skills/{slug}/modules` | Modules of a skill | 020 |
| GET | `/api/courses/{slug}/modules` | Module list and state | 011, 012 |
| GET | `/api/courses/{slug}/modules/{index}` | Module content | 012, 013 |
| POST | `/api/courses/{slug}/modules/{index}/submit` | Submit a quiz | 013 |
| POST | `/api/courses/{slug}/modules/{index}/check` | Check a lab | 014 |
| POST | `/api/courses/{slug}/modules/{index}/steps/{stepIndex}/check` | Check a lab step | 014 |
| POST | `/api/courses/{slug}/modules/{index}/record` | Record a local check | 014 |
| POST | `/api/courses/{slug}/lessons/{lessonSlug}/complete` | Mark a lesson as completed | 012 |
| POST, PUT, DELETE | `/api/admin/courses/{slug}/sessions[/{sessionId}]` | Sessions (manager or admin) | 022 |
| * | `/api/admin/courses…`, `/api/admin/cache/clear`, `/api/admin/lab-checks`, `/api/admin/exports/lab-checks/*` | Administration | 042, 047 |
| GET | `/uploads/{filename}` | Media | 012 |

### Routes of the old client not to carry over

`lib/api.ts` declares routes that no longer exist in the Go routers:
`/courses/{id}/labs/{labId}/submit` and the `Lab`, `LabProgress`, `SubmissionResult` types. They
are not carried over. The `thumbnail` field of a course is read by the frontend but is not
returned by the API (see [090](090-gaps-and-defects.md)).

## Acceptance criteria

- `grep -r "fetch(" src` finds only the HTTP client.
- With `pnpm dev`, every `/api/*` and `/uploads/*` request reaches `VITE_API_URL`.
- An error `{ "error": "Course not found" }` with a 404 status shows this message in the page's
  error state.

## Sources

`frontend/src/lib/api.ts`, `pages/api/[...path].ts`, `astro.config.mjs` (dev proxy);
`user-service/internal/handlers/router.go`, `course-service/internal/handlers/router.go`.
