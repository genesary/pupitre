# 103 - Backend: HTTP API

The monolith serves every public route of user-service and course-service from **one** chi
router, on one port. Nothing in a route changes: method, path, guard, body limit, request and
response bodies, status codes and error messages are those of `3cdf7b2`. The frontend contract
of [004](004-api-and-routing.md) and `packages/core/src/lib/api-types.ts` stays valid.

## Middleware

Applied to every route, in this order, as both services did:

1. `RequestID`
2. `RealIP` (the API runs behind a trusted reverse proxy; the IP is used for logs and rate limits)
3. Request logger (off in tests)
4. `Recoverer`
5. Security headers ([190](190-backend-defects.md) B15): `X-Content-Type-Options: nosniff`,
   `Referrer-Policy: no-referrer`, and `Content-Security-Policy: default-src 'none';
   frame-ancestors 'none'` with `X-Frame-Options: DENY`; on `/uploads/*`,
   `Content-Security-Policy: script-src 'none'; frame-ancestors 'self'` with
   `X-Frame-Options: SAMEORIGIN`, so an uploaded SVG or HTML file runs no script.
6. CORS: origins from `CORS_ORIGINS`, methods `GET POST PUT PATCH DELETE OPTIONS`, headers
   `Accept Authorization Content-Type`, credentials allowed, preflight cached 300 s.

The **body limit** is set per route group, not globally: **1 MB** for every route, except the
course admin routes (`registerAdminRoutes` of course-service) which accept **10 MB** for markdown
imports of whole courses. user-service applied its 1 MB limit globally; in the monolith it must
not wrap the course admin routes, or course imports would break.

Guards are the shared middlewares of `internal/platform/auth`: `Auth` (signed in), `Admin`,
`Manager`, `ManagerOrAdmin`. The guard of a route is the one of its group in the table below.
Five sign-in routes are rate-limited per client IP ([105](105-backend-authentication.md)).

## Server

| Setting | Value | Was |
| --- | --- | --- |
| Read timeout | 30 s | 30 s (user, course, checker) |
| Read header timeout | 10 s | 10 s (checker only; now everywhere) |
| Write timeout | 60 s | 60 s (user, course); 30 s (checker, no public route) |
| Idle timeout | 120 s | 120 s (user, course) |
| Graceful shutdown | 15 s | 15 s |

## Overlapping prefixes

user-service and course-service both mounted routes under `/api/admin`, `/api/users`,
`/api/paths` and `/api/manager`. No method and path pair is served by both (the table is built
from both routers and shows none), so one router holds them all. A route added later must not
reuse a pair of the other domain; a test walks the router and fails on a duplicate.

The frontend's split of `/api` by prefix ([004](004-api-and-routing.md)) is no longer needed: the
dev proxy and the front door send all of `/api` to one target
([108](108-backend-operations-and-testing.md)).

## Removed and merged routes

- The 11 `/internal/*` routes of user-service and the `/evaluate` and `/check-step` routes of
  checker-service are removed: their callers now call functions
  ([104](104-backend-in-process-calls.md), [107](107-backend-labs-and-checker.md)). They were
  never reachable from a browser (guarded by `X-Internal-Secret`), so no client loses anything.
- `/health` and `/metrics` existed once per service: the monolith serves each once. `/health`
  answers `{"status":"ok","service":"pupitre-api","version":"1.0.0"}` (the services named
  themselves in `service`; probes only read the status). `/metrics` exposes the metrics of every
  domain, with the same names: they were already one registry in `internal/metrics`.

## Route inventory

Generated from the routers at `3cdf7b2` (`user-service/internal/handlers/router.go`,
`course-service/internal/handlers/router.go`, `checker-service/internal/handlers/handler.go`).
"From" is the service that served the route. 135 routes are kept as they are, 2 are merged and
13 are removed; one was added after the move by a fix of [190](190-backend-defects.md).

| Method | Route | Access | Body limit | From | Handler | In the monolith |
| --- | --- | --- | --- | --- | --- | --- |
| POST | `/api/admin/cache/clear` | admin | 10 MB | course | `ClearCache` | Kept |
| GET | `/api/admin/courses` | admin | 10 MB | course | `ListAdminCourses` | Kept |
| POST | `/api/admin/courses` | admin | 10 MB | course | `CreateCourse` | Kept |
| POST | `/api/admin/courses/import` | admin | 10 MB | course | `ImportCourseMarkdown` | Kept |
| POST | `/api/admin/courses/import/preview` | admin | 10 MB | course | `PreviewCourseMarkdownImport` | Kept |
| POST | `/api/admin/courses/paths` | admin | 10 MB | course | `CreatePath` | Kept |
| GET | `/api/admin/courses/paths/{slug}/definition` | admin | 10 MB | course | `GetPathDefinition` | Kept |
| PUT | `/api/admin/courses/paths/{slug}/definition` | admin | 10 MB | course | `UpdatePath` | Kept |
| DELETE | `/api/admin/courses/paths/{slug}/definition` | admin | 10 MB | course | `DeletePath` | Kept |
| POST | `/api/admin/courses/{slug}/cache/clear` | admin | 10 MB | course | `ClearCourseCache` | Kept |
| GET | `/api/admin/courses/{slug}/definition` | admin | 10 MB | course | `GetCourseDefinition` | Kept |
| PUT | `/api/admin/courses/{slug}/definition` | admin | 10 MB | course | `UpdateCourse` | Kept |
| DELETE | `/api/admin/courses/{slug}/definition` | admin | 10 MB | course | `DeleteCourse` | Kept |
| GET | `/api/admin/courses/{slug}/export/markdown` | admin | 10 MB | course | `ExportCourseMarkdown` | Kept |
| POST | `/api/admin/courses/{slug}/modules/{index}/cache/clear` | admin | 10 MB | course | `ClearModuleCache` | Kept |
| POST | `/api/admin/courses/{slug}/sessions` | manager or admin | 1 MB | course | `CreateSession` | Kept |
| PUT | `/api/admin/courses/{slug}/sessions/{sessionId}` | manager or admin | 1 MB | course | `UpdateSession` | Kept |
| DELETE | `/api/admin/courses/{slug}/sessions/{sessionId}` | manager or admin | 1 MB | course | `DeleteSession` | Kept |
| GET | `/api/admin/enrollments/{slug}` | admin | 1 MB | user | `ListCourseEnrollments` | Kept |
| POST | `/api/admin/enrollments/{slug}` | admin | 1 MB | user | `AdminEnrollUser` | Kept |
| GET | `/api/admin/enrollments/{slug}/groups` | admin | 1 MB | user | `AdminListGroupEnrollments` | Kept |
| POST | `/api/admin/enrollments/{slug}/groups` | admin | 1 MB | user | `AdminEnrollGroup` | Kept |
| DELETE | `/api/admin/enrollments/{slug}/groups/{groupId}` | admin | 1 MB | user | `AdminUnenrollGroup` | Kept |
| DELETE | `/api/admin/enrollments/{slug}/users/{userId}` | admin | 1 MB | user | `AdminUnenrollUser` | Kept |
| GET | `/api/admin/exports/categories` | manager or admin | 1 MB | user | `ExportCategories` | Kept |
| POST | `/api/admin/exports/download` | manager or admin | 1 MB | user | `ExportDownload` | Kept |
| GET | `/api/admin/exports/lab-checks/categories` | admin | 10 MB | course | `ExportLabCategories` | Kept |
| POST | `/api/admin/exports/lab-checks/download` | admin | 10 MB | course | `ExportLabDownload` | Kept |
| POST | `/api/admin/exports/lab-checks/preview` | admin | 10 MB | course | `ExportLabPreview` | Kept |
| POST | `/api/admin/exports/preview` | manager or admin | 1 MB | user | `ExportPreview` | Kept |
| GET | `/api/admin/groups` | admin | 1 MB | user | `ListGroups` | Kept |
| POST | `/api/admin/groups` | admin | 1 MB | user | `CreateGroup` | Kept |
| GET | `/api/admin/groups/mappings` | admin | 1 MB | user | `ListGroupMappings` | Kept |
| POST | `/api/admin/groups/mappings` | admin | 1 MB | user | `UpsertGroupMapping` | Kept |
| DELETE | `/api/admin/groups/mappings/{groupName}` | admin | 1 MB | user | `DeleteGroupMapping` | Kept |
| PUT | `/api/admin/groups/{groupId}` | admin | 1 MB | user | `UpdateGroup` | Kept |
| DELETE | `/api/admin/groups/{groupId}` | admin | 1 MB | user | `DeleteGroup` | Kept |
| GET | `/api/admin/groups/{groupId}/ancestors` | admin | 1 MB | user | `ListAncestors` | Kept |
| GET | `/api/admin/groups/{groupId}/descendants` | admin | 1 MB | user | `ListDescendants` | Kept |
| GET | `/api/admin/groups/{groupId}/members` | admin | 1 MB | user | `ListGroupMembers` | Kept |
| POST | `/api/admin/groups/{groupId}/members` | admin | 1 MB | user | `AddGroupMember` | Kept |
| DELETE | `/api/admin/groups/{groupId}/members/{userId}` | admin | 1 MB | user | `RemoveGroupMember` | Kept |
| PATCH | `/api/admin/groups/{groupId}/parent` | admin | 1 MB | user | `MoveGroup` | Kept |
| GET | `/api/admin/groups/{groupId}/subgroups` | admin | 1 MB | user | `ListSubgroups` | Kept |
| POST | `/api/admin/groups/{groupId}/subgroups` | admin | 1 MB | user | `CreateSubgroup` | Kept |
| GET | `/api/admin/lab-checks` | admin | 10 MB | course | `GetLabResults` | Kept |
| GET | `/api/admin/leaderboard` | admin | 1 MB | user | `AdminLeaderboard` | Kept |
| GET | `/api/admin/paths/{slug}/enrollments` | admin | 1 MB | user | `AdminListPathEnrollments` | Kept |
| POST | `/api/admin/paths/{slug}/enrollments` | admin | 1 MB | user | `AdminEnrollUserInPath` | Kept |
| DELETE | `/api/admin/paths/{slug}/enrollments/{userId}` | admin | 1 MB | user | `AdminUnenrollUserFromPath` | Kept |
| POST | `/api/admin/patterns` | admin | 1 MB | user | `CreatePattern` | Kept |
| PUT | `/api/admin/patterns/{name}` | admin | 1 MB | user | `UpdatePattern` | Kept |
| DELETE | `/api/admin/patterns/{name}` | admin | 1 MB | user | `DeletePattern` | Kept |
| GET | `/api/admin/session-bookings/{slug}/{sessionId}` | admin | 1 MB | user | `ListSessionBookings` | Kept |
| PATCH | `/api/admin/session-bookings/{slug}/{sessionId}/{userId}/presence` | admin | 1 MB | user | `MarkSessionPresence` | Kept |
| GET | `/api/admin/settings` | admin | 1 MB | user | `GetSettings` | Kept |
| PUT | `/api/admin/settings` | admin | 1 MB | user | `UpdateSettings` | Kept |
| GET | `/api/admin/stats` | admin | 1 MB | user | `AdminStats` | Kept |
| POST | `/api/admin/sync-progress` | admin | 1 MB | user | `SyncProgress` | Kept |
| GET | `/api/admin/users` | admin | 1 MB | user | `ListUsers` | Kept |
| GET | `/api/admin/users/providers` | admin | 1 MB | user | `ListAuthProviders` | Kept |
| GET | `/api/admin/users/search` | admin | 1 MB | user | `SearchUsers` | Kept |
| GET | `/api/admin/users/{userId}` | admin | 1 MB | user | `GetUser` | Kept |
| PUT | `/api/admin/users/{userId}` | admin | 1 MB | user | `UpdateUser` | Kept |
| DELETE | `/api/admin/users/{userId}` | admin | 1 MB | user | `DeleteUser` | Kept |
| POST | `/api/auth/ldap/login` | public, rate-limited | 1 MB | user | `LDAPLogin` | Kept |
| POST | `/api/auth/login` | public, rate-limited | 1 MB | user | `Login` | Kept |
| GET | `/api/auth/me` | signed in | 1 MB | user | `Me` | Kept |
| POST | `/api/auth/logout` | signed in | 1 MB | user | `Logout` | **Added**: ends every session of the user ([190](190-backend-defects.md) B7) |
| POST | `/api/auth/oauth/callback` | public, rate-limited | 1 MB | user | `OAuthCallback` | Kept |
| GET | `/api/auth/oauth/providers` | public | 1 MB | user | `ListProviders` | Kept |
| GET | `/api/auth/oauth/{provider}/authorize` | public | 1 MB | user | `OAuthAuthorize` | Kept |
| GET | `/api/auth/oidc/authorize` | public | 1 MB | user | `OIDCAuthorize` | Kept |
| POST | `/api/auth/oidc/callback` | public, rate-limited | 1 MB | user | `OIDCCallback` | Kept |
| PUT | `/api/auth/password` | signed in | 1 MB | user | `ChangePassword` | Kept |
| PUT | `/api/auth/profile` | signed in | 1 MB | user | `UpdateProfile` | Kept |
| POST | `/api/auth/register` | public, rate-limited | 1 MB | user | `Register` | Kept |
| GET | `/api/badges/{slug}` | public | 1 MB | user | `BadgeStats` | Kept |
| GET | `/api/batch/courses` | public | 1 MB | course | `ListCoursesBatch` | Kept |
| GET | `/api/batch/paths` | public | 1 MB | course | `ListPathsBatch` | Kept |
| GET | `/api/batch/skills` | public | 1 MB | course | `ListSkillModulesBatch` | Kept |
| GET | `/api/courses` | public | 1 MB | course | `ListCourses` | Kept |
| GET | `/api/courses/{slug}` | public | 1 MB | course | `GetCourse` | Kept |
| GET | `/api/courses/{slug}/labs` | signed in | 1 MB | course | `ListLabs` | Kept |
| GET | `/api/courses/{slug}/labs/{lab_id}` | signed in | 1 MB | course | `GetLab` | Kept |
| GET | `/api/courses/{slug}/lessons` | signed in | 1 MB | course | `ListLessons` | Kept |
| GET | `/api/courses/{slug}/lessons/{lessonSlug}` | signed in | 1 MB | course | `GetLesson` | Kept |
| POST | `/api/courses/{slug}/lessons/{lessonSlug}/complete` | signed in | 1 MB | course | `MarkLessonComplete` | Kept |
| GET | `/api/courses/{slug}/modules` | signed in | 1 MB | course | `ListModules` | Kept |
| GET | `/api/courses/{slug}/modules/{index}` | signed in | 1 MB | course | `GetModule` | Kept |
| POST | `/api/courses/{slug}/modules/{index}/check` | signed in | 1 MB | course | `CheckModule` | Kept |
| POST | `/api/courses/{slug}/modules/{index}/record` | signed in | 1 MB | course | `RecordLocalCheck` | Kept |
| POST | `/api/courses/{slug}/modules/{index}/steps/{stepIndex}/check` | signed in | 1 MB | course | `CheckModuleStep` | Kept |
| POST | `/api/courses/{slug}/modules/{index}/submit` | signed in | 1 MB | course | `SubmitModule` | Kept |
| GET | `/api/courses/{slug}/progress` | signed in | 1 MB | course | `GetCourseProgress` | Kept |
| POST | `/api/enrollments/{slug}` | signed in | 1 MB | user | `Enroll` | Kept |
| DELETE | `/api/enrollments/{slug}` | signed in | 1 MB | user | `Unenroll` | Kept |
| GET | `/api/leaderboard` | public | 1 MB | user | `BadgeLeaderboard` | Kept |
| POST | `/api/manager/enrollments/{slug}` | manager | 1 MB | user | `ManagerEnrollUser` | Kept |
| DELETE | `/api/manager/enrollments/{slug}/users/{userId}` | manager | 1 MB | user | `ManagerUnenrollUser` | Kept |
| GET | `/api/manager/groups` | manager | 1 MB | user | `ManagerListGroups` | Kept |
| POST | `/api/manager/groups` | manager | 1 MB | user | `ManagerCreateGroup` | Kept |
| DELETE | `/api/manager/groups/{groupId}` | manager | 1 MB | user | `ManagerDeleteGroup` | Kept |
| GET | `/api/manager/groups/{groupId}/members` | manager | 1 MB | user | `ManagerListGroupMembers` | Kept |
| POST | `/api/manager/groups/{groupId}/members` | manager | 1 MB | user | `ManagerAddGroupMember` | Kept |
| DELETE | `/api/manager/groups/{groupId}/members/{userId}` | manager | 1 MB | user | `ManagerRemoveGroupMember` | Kept |
| POST | `/api/manager/groups/{groupId}/subgroups` | manager | 1 MB | user | `ManagerCreateSubgroup` | Kept |
| GET | `/api/manager/paths/{slug}/enrollments` | manager | 1 MB | user | `ManagerListPathEnrollments` | Kept |
| POST | `/api/manager/paths/{slug}/enrollments` | manager | 1 MB | user | `ManagerEnrollUserPath` | Kept |
| DELETE | `/api/manager/paths/{slug}/enrollments/{userId}` | manager | 1 MB | user | `ManagerUnenrollUserPath` | Kept |
| GET | `/api/manager/session-bookings/{slug}/{sessionId}` | manager | 1 MB | user | `ListSessionBookings` | Kept |
| PATCH | `/api/manager/session-bookings/{slug}/{sessionId}/{userId}/presence` | manager | 1 MB | user | `MarkSessionPresence` | Kept |
| GET | `/api/manager/users` | manager | 1 MB | user | `ManagerListUsers` | Kept |
| GET | `/api/manager/users/{userId}/enrollments` | manager | 1 MB | user | `ManagerGetUserEnrollments` | Kept |
| GET | `/api/manager/users/{userId}/path-enrollments` | manager | 1 MB | user | `ManagerGetUserPathEnrollments` | Kept |
| GET | `/api/my/badges` | signed in | 1 MB | user | `MyBadges` | Kept |
| GET | `/api/my/courses` | signed in | 1 MB | user | `MyCourses` | Kept |
| GET | `/api/my/groups` | signed in | 1 MB | user | `MyGroups` | Kept |
| GET | `/api/my/groups/{groupId}/members` | signed in | 1 MB | user | `MyGroupMembers` | Kept |
| GET | `/api/my/paths` | signed in | 1 MB | user | `MyPaths` | Kept |
| GET | `/api/my/session-bookings` | signed in | 1 MB | user | `MySessionBookings` | Kept |
| GET | `/api/my/skills` | signed in | 1 MB | user | `MySkills` | Kept |
| GET | `/api/my/skills/{slug}` | signed in | 1 MB | user | `MySkillModules` | Kept |
| GET | `/api/my/xp` | signed in | 1 MB | user | `MyXP` | Kept |
| GET | `/api/paths` | public | 1 MB | course | `ListPaths` | Kept |
| GET | `/api/paths/{slug}` | public | 1 MB | course | `GetPath` | Kept |
| GET | `/api/patterns` | signed in | 1 MB | user | `ListPatterns` | Kept |
| GET | `/api/patterns/{id}` | signed in | 1 MB | user | `GetPattern` | Kept |
| POST | `/api/session-bookings/{slug}/{sessionId}` | signed in | 1 MB | user | `BookSession` | Kept |
| DELETE | `/api/session-bookings/{slug}/{sessionId}` | signed in | 1 MB | user | `UnbookSession` | Kept |
| GET | `/api/session-bookings/{slug}/{sessionId}/count` | signed in | 1 MB | user | `SessionBookingCount` | Kept |
| GET | `/api/settings/public` | public | 1 MB | user | `PublicSettings` | Kept |
| GET | `/api/skills/{slug}/modules` | public | 1 MB | course | `ListSkillModules` | Kept |
| GET | `/api/users/{id}` | public | 1 MB | user | `PublicUser` | Kept |
| GET | `/api/users/{id}/badges` | public | 1 MB | user | `UserBadges` | Kept |
| POST | `/check-step` | internal secret | 1 MB | checker | `CheckStep` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| POST | `/evaluate` | internal secret | 1 MB | checker | `Evaluate` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/health` | public | 1 MB | checker+course+user | `Health` | One route for the whole process |
| POST | `/internal/enrollments/auto` | internal secret | 1 MB | user | `InternalAutoEnroll` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/enrollments/check` | internal secret | 1 MB | user | `InternalCheckEnrollment` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/paths/check` | internal secret | 1 MB | user | `InternalCheckPathEnrollment` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| POST | `/internal/progress/complete` | internal secret | 1 MB | user | `InternalMarkComplete` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| POST | `/internal/progress/course-complete` | internal secret | 1 MB | user | `InternalMarkCourseComplete` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/progress/course-summaries` | internal secret | 1 MB | user | `InternalCourseSummaries` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/progress/course-summary` | internal secret | 1 MB | user | `InternalCourseSummary` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| POST | `/internal/progress/module` | internal secret | 1 MB | user | `InternalRecordModuleProgress` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/progress/modules` | internal secret | 1 MB | user | `InternalGetModuleProgress` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/progress/overview` | internal secret | 1 MB | user | `InternalProgressOverview` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/internal/progress/viewed` | internal secret | 1 MB | user | `InternalViewedLessons` | Removed: in-process call ([104](104-backend-in-process-calls.md)) |
| GET | `/metrics` | public | 1 MB | course+user | `metrics.Handler` | One route for the whole process |
| GET | `/uploads/{filename}` | public | 1 MB | course | `ServeUpload` | Kept |

150 method and route pairs.

## Acceptance criteria

- `internal/server/router_test.go` walks the monolith router and compares its method and path
  pairs with this table (`testdata/routes.tsv`, generated from it): every "Kept" and "One route"
  row is served, no other route is. It also checks every guard: a guarded route answers an
  anonymous request with 401, a role-guarded one answers a student with 403.
- The Go e2e suite and the frontend Playwright suite pass against the monolith.
- A 5 MB markdown import to `/api/admin/courses/import` is accepted; a 2 MB body to a learner route
  is refused with the same status as today.

## Sources

The three routers named above; `AGENTS.md` of `genesary/pupitre` ("Routing", body limits).
