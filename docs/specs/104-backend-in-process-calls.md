# 104 - Backend: in-process calls

In `genesary/pupitre`, the services called each other over HTTP: course-service read progress
from user-service and wrote it there, user-service read course definitions from course-service,
and course-service sent lab checks to checker-service. In the monolith each of those calls is a
**function call** behind a small interface. The semantics of each call (what it returns, what a
failure does, whether it is detached) are kept exactly.

## Rules

- **The caller declares the interface**, with its own types, in its own package:
  `UserProgress` and `LabChecker` in `courses/handlers`, `CourseSource` in `users/handlers`. The
  callee exposes plain methods (`users/handlers/progress_service.go`, `BatchCourses`…,
  `checker/handlers` `RunEvaluate` and `RunCheckStep`). Only `internal/server/adapters.go` knows
  both sides and wires them.
- A `State` built without one of its peers answers as if that peer were unreachable
  (`noUsers`, `noChecker`, `noCourseSource`): reads degrade, writes fail, as they did when a
  service was down. The monolith always wires them.
- An interface method does what the HTTP route did, with the same inputs and outputs: the route's
  handler and the new method share the same repository code, so there is one implementation.
- **Failure semantics are kept.** A call that used to degrade to an empty answer when user-service
  was unreachable still degrades to an empty answer on an error (a database error now); a call
  that used to fail the response still fails it with the same status and message.
- **Detached calls stay detached**: they run in a goroutine, on a context detached from the
  request (`context.WithoutCancel`), bounded by the same 10 s timeout, and a failure is only
  logged at `warn`. The response never waits for them.
- The `httpx` client, `internalauth`, the `X-Internal-Secret` header and the internal URLs
  disappear with the last call that used them.

## Calls from courses to users

From `course-service/internal/handlers/user_client.go`, `lessons.go` and `modules.go`.

| HTTP call at `3cdf7b2` | Method in the monolith | Used by | On failure | Mode |
| --- | --- | --- | --- | --- |
| `GET /internal/progress/overview?userId&courseSlug` | `Progress.CourseOverview(ctx, userID, courseSlug)` | course and lesson pages: completed modules, scores, enrollment | Empty progress, warning | Sync |
| `GET /internal/progress/course-summaries?userId&slugs` | `Progress.CourseSummaries(ctx, userID, slugs)` | prerequisites and path pages, in one call for every slug | Empty summaries, warning | Sync |
| `GET /internal/paths/check?userId&pathSlugs` | `Progress.EnrolledInAnyPath(ctx, userID, pathSlugs)` | access to a course reserved to a path | `false`, warning | Sync |
| `GET /internal/enrollments/check?userId&courseSlug` | `Progress.IsEnrolled(ctx, userID, courseSlug)` | access to a non-public course | `false`, warning | Sync |
| `POST /internal/progress/complete` | `Progress.MarkLessonComplete(ctx, userID, courseSlug, lessonSlug)` | "lesson completed" | `500 Failed to mark lesson as complete` | Sync |
| `POST /internal/progress/course-complete` | `Progress.MarkCourseComplete(ctx, …)` (badge, skills, XP) | last module completed | Logged only; the response is unchanged | Sync |
| `POST /internal/enrollments/auto` | `Progress.AutoEnroll(userID, courseSlug)` | first visit of a lesson | Logged at `warn` | **Detached** |
| `POST /internal/progress/module` | `Progress.RecordModule(userID, …, score, maxScore, passed)` | quiz and lab results | Logged at `warn` | **Detached** |

Three internal routes had no caller at `3cdf7b2`: `GET /internal/progress/course-summary`,
`GET /internal/progress/modules` and `GET /internal/progress/viewed`. They are removed without a
replacement; their repository code stays if another route uses it.

The read calls keep the guarantee of the performance invariants: **one** progress read per course
request, and one batched read for many courses, never one per item.

## Calls from users to courses

From `user-service/internal/handlers/catalog.go`. user-service needed course and path
definitions (titles, modules, skills) to build learner pages, exports and the leaderboard. It
fetched them in batches from course-service's public `/api/batch/*` routes and cached them.

| HTTP call at `3cdf7b2` | Method in the monolith |
| --- | --- |
| `GET /api/batch/courses?slugs=` | `Catalog.Courses(ctx, slugs)` |
| `GET /api/batch/paths?slugs=` | `Catalog.Paths(ctx, slugs)` |
| `GET /api/batch/skills?skills=` | `Catalog.SkillModules(ctx, skills)` |

The `Catalog` type and its cache are kept: entries are memoized for **30 s**, concurrent misses
share one fetch, and the size stays bounded. Only the fetch changes, from an HTTP request to a
call into the courses domain. A missing course is still left out of the result, as it was.

The users domain reads the catalog through its own types (`CourseInfo`, `PathInfo`,
`SkillModule`), which mirror the JSON of the batch routes. The adapter converts the courses
domain's batch results through that same JSON, so the two sides stay in step with the public
contract.

Three more calls were found while writing the monolith, all anonymous reads of public routes:

| HTTP call at `3cdf7b2` | Used by | Method in the monolith | On failure |
| --- | --- | --- | --- |
| `GET /api/courses/{slug}` | booking a session (`sessions.go`, `sessionCapacity`), for the session's capacity | `CourseSource.CourseSessions(ctx, slug)` | Capacity 0, that is unlimited, warning. Reads a private course too: the anonymous read refused it, so its capacity was ignored ([190](190-backend-defects.md) B24) |
| `GET /api/courses` | CSV exports (`exports.go`), to label rows with course titles | `CourseSource.PublicCourseTitles(ctx)` | Titles left empty, as before. Only public courses are named, as the anonymous list did |
| `GET /api/paths` | CSV exports, for path titles | `CourseSource.PathTitles(ctx)` | Titles left empty |

The `/api/batch/*` routes stay public routes ([103](103-backend-http-api.md)): they are part of
the API, not only of the internal traffic.

## Calls from courses to the checker

From `course-service/internal/handlers/check.go`: `POST /evaluate` and `POST /check-step` become
`Checker.Evaluate(ctx, req)` and `Checker.CheckStep(ctx, req)`, with the request and result types
of checker-service. Error mapping is kept as the learner saw it:

| Before | After |
| --- | --- |
| checker-service unreachable: `500 error when reaching for checker service` | Cannot happen: no network |
| checker-service answered non-200 with `{"error": msg}`: `502 msg` | The checker returns an error carrying `msg`: `502 msg` |
| checker-service rate limit: `429` relayed as `502` | Per learner, `429` with `Retry-After` ([101](101-backend-configuration.md), [190](190-backend-defects.md) B9) |

The limits and isolation of the checker are in [107](107-backend-labs-and-checker.md).

## Acceptance criteria

- No `http.Client` call targets the application itself, and no production code mounts an
  `/internal/` route.
- `internal/server/wiring_test.go` checks the failure semantics (degrade, fail, detach) with a
  failing fake, and a learner flow across the domains.
- The existing handler tests of the domains keep their assertions. They stood in for the other
  service with an `httptest` server and still do: each domain's `standalone_test.go` keeps, for
  the tests only, the old HTTP clients (as implementations of the interfaces) and the old router
  with its internal routes (as shims over the in-process methods).

## Sources

`course-service/internal/handlers/user_client.go`, `lessons.go`, `modules.go`, `check.go`;
`user-service/internal/handlers/internal.go`, `catalog.go`; `AGENTS.md` of `genesary/pupitre`
(batch endpoints, performance invariants).
