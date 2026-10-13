# e-learning platform

## MANDATORY — NEVER COMMIT

You are strictly forbidden from running `git commit`, `git add`, `git push`, or any other git mutation commands. Only the user may commit. If you are asked to commit or you think a commit is needed, refuse and tell the user to do it themselves.

## Domain model (source of truth: `docs/`)

- **Course** — has a title, description, hidden (boolean publication state), category, difficulty, and a list of modules. Stored in PostgreSQL and managed through the course-service admin API. Modules carry their markdown inline (`content`), or reference external content by type (`video`, `text`, `image`) and a git repo (`src`, `ref`, `path`).
- **Module** — a course element. Video/image: server-hosted URL. Text: inline markdown stored on the module, or a markdown file from a git repo. Inline is the default authoring path; git is for content that already lives in a repo.
- **User management** — database-backed (no further detail yet).

## API shape

- `api/courses/kubernetes-basics/<courseSlug>` — returns module list with slugs, titles, order, and per-user `viewed` status.
- `api/courses/kubernetes-basics/<courseSlug>/<moduleSlug>` — returns module content with `viewed` boolean.

## Architecture

Two micro-services (see `docs/ARCHITECTURE.md`):

- **Course Service** (PG, Port 8082) — course catalogue and content, media serving, calls User Service for enrollment/progress
- **User Service** (PG, Port 8081) — auth, users, enrollments, progress, settings, OAuth, exposes internal API for Course Service

## Codebase

### One module, shared packages

The repository is a **single Go module** rooted at `github.com/genesary/pupitre`.
There is one `go.mod`, one `.golangci.yml`, and one test/lint/build invocation
for the whole tree:

```sh
go build ./...
go test ./... -race
golangci-lint run ./...
```

Each service still owns its own `internal/` tree, and Go's internal-package
rule still enforces that boundary: `course-service/internal/...` is
importable only from under `course-service/`, so services cannot reach into
each other. Cross-cutting code lives in the **root** `internal/`, which every
service may import:

| Package                  | Role                                                                                                                                                                                        |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/middleware/`   | The JWT contract — `Claims`, `CreateToken`, `VerifyToken`, and the `Auth`/`Manager`/`Admin`/`ManagerOrAdmin` guards. user-service mints tokens; every service verifies them with this code. |
| `internal/internalauth/` | Shared-secret guard for service-to-service routes (`X-Internal-Secret`). Kept separate from `internal/middleware` so checker-service does not link the JWT machinery.                       |
| `internal/httperr/`      | The JSON error envelope (`httperr.Write`) every service returns.                                                                                                                            |
| `internal/httpx/`        | Outbound HTTP client with the shared timeouts and connection pool.                                                                                                                          |
| `internal/metrics/`      | Prometheus collectors and the `/metrics` handler.                                                                                                                                           |

**Do not copy anything out of the root `internal/` into a service.** These
packages exist because the per-service copies drifted: the JWT middleware was
duplicated in two services and only one of them rejected a token with an empty
`sub`, while the shared-secret guard existed in three places. Authentication
has exactly one implementation now — change it there.

Container images build from the **repository root** with an explicit
Dockerfile path, because the build needs the root `go.mod` and `internal/`:

```sh
docker build -f user-service/Dockerfile -t pupitre-user-service .
```

### Course Service (`course-service/`)

```sh
go build -o /dev/null ./course-service
```

| Package                                     | Role                                                                                 |
| ------------------------------------------- | ------------------------------------------------------------------------------------ |
| `internal/content/`                         | Domain types, quiz scoring, git module resolution                                    |
| `internal/models/` + `internal/repository/` | GORM models and data access (courses, modules, paths, quiz attempts, lab checks)     |
| `internal/db/`                              | PG connection + AutoMigrate + dev course seed                                        |
| `internal/definition/`                      | Wire shape of a course/path definition, shared by the admin API and the seeder       |
| `internal/markdown/`                        | Course ⇄ single markdown document: frontmatter, heading splitting, module directives |
| `internal/handlers/`                        | HTTP handlers (chi router) — courses, modules, lessons                               |
| `internal/config/`                          | Env-based config (port, JWT, DATABASE_URL, UserServiceURL)                           |

**Routes:** Public: `/health`, `/metrics`, `/api/courses`, `/api/courses/{slug}`, `/uploads/{filename}` — Authenticated: `/api/courses/{slug}/modules`, `/api/courses/{slug}/lessons`, `/api/courses/{slug}/lessons/{lesson}/complete`

**Batch endpoints.** Anything that resolves a *set* of slugs uses these, never
one request per slug:

```
GET /api/batch/courses?slugs=a,b,c   # course metadata, one query
GET /api/batch/paths?slugs=a,b,c     # path metadata, members included
GET /api/batch/skills?slugs=a,b      # modules per skill, keyed by skill
```

Each caps out at 500 slugs and answers in a fixed number of queries. When you
add a consumer that needs many courses/paths/skills, extend a batch endpoint
rather than looping over the single-slug one.

They live under `/api/batch/` and **not** as a `batch` segment of the
collection they read: chi matches a static segment before a wildcard, so
`/api/courses/batch` made a course legitimately slugged `batch` unreachable
and answered with a course list in its place. Keep new collection-wide
endpoints off the `/api/{collection}/{slug}` namespace for the same reason.

### User Service (`user-service/`)

```sh
go build -o /dev/null ./user-service
```

| Package              | Role                                                                |
| -------------------- | ------------------------------------------------------------------- |
| `internal/db/`       | PG connection + migration runner                                    |
| `internal/handlers/` | HTTP handlers — auth, oauth, settings, admin, enrollments, progress |
| `internal/config/`   | Env-based config (DB, JWT, OAuth)                                   |
| `migrations/`        | SQL migrations (embed)                                              |

**Routes:** Public auth + OAuth, Authenticated (enroll, progress, my courses), Admin (users, settings, enrollments), Internal (for Course Service): `/internal/enrollments/check`, `/internal/progress/viewed`, `/internal/progress/complete`

**Routing invariant — no route may sit under a course-service prefix**
(`/api/courses`, `/api/admin/courses`). The two services share one hostname,
so whichever segment decides the owner has to come *before* any variable
segment. Enrolling was once `POST /api/courses/{slug}/enroll`: the owner was
only known after the slug, which no prefix match can express, so every front
door needed its own workaround (a Traefik CRD, an nginx regex annotation, an
implementation-specific Gateway API match). These are keyed by their own
resource instead:

```
POST|DELETE /api/enrollments/{slug}
POST|DELETE /api/session-bookings/{slug}/{sessionId}
GET         /api/session-bookings/{slug}/{sessionId}/count
GET|POST    /api/admin/enrollments/{slug}          + /groups, /users/{userId}
GET|PATCH   /api/admin/session-bookings/{slug}/{sessionId}[/{userId}/presence]
            …and the /api/manager/… equivalents
```

`TestRouter_NoCourseServicePrefixOverlap` fails if a route is added back under
a course-service prefix.

**Internal progress API.** Course-service reads a learner's state through two
consolidated endpoints rather than one call per fact:

```
GET /internal/progress/overview?userId=&courseSlug=
      # enrolled + viewed lessons + per-module scores + aggregates, 3 queries
GET /internal/progress/course-summaries?userId=&courseSlugs=a,b,c
      # prerequisite aggregates for many courses, 2 queries
```

`/internal/progress/viewed`, `/internal/progress/modules` and
`/internal/progress/course-summary` remain for single-fact callers, but a
handler that needs more than one of them should use `overview`.

## Course source of truth

Courses live in **PostgreSQL** and are managed through the admin API. There is no in-memory store, no filesystem loading, and no Kubernetes controller: every request reads what it needs from the database.

```
POST   /api/admin/courses                      # create  {slug, spec}
GET    /api/admin/courses/{slug}/definition    # read
PUT    /api/admin/courses/{slug}/definition    # replace {spec}
DELETE /api/admin/courses/{slug}/definition    # delete

POST   /api/admin/courses/import               # create/replace/append from markdown
POST   /api/admin/courses/import/preview       # same, but stores nothing
GET    /api/admin/courses/{slug}/export/markdown
```

Learning paths are managed the same way, under `courses/paths` (the admin UI
for them is the "Parcours" tab of `/admin/paths`):

```
POST   /api/admin/courses/paths                       # create  {slug, spec}
GET    /api/admin/courses/paths/{slug}/definition     # read
PUT    /api/admin/courses/paths/{slug}/definition     # replace {spec}
DELETE /api/admin/courses/paths/{slug}/definition     # delete
```

Read a path through the admin endpoint, not the public `GET /api/paths/{slug}`:
that one replaces a course-kind path's stored `skills` with the aggregate of
its members' skills, so an editor round-tripping through it would write the
aggregate back as if an author had typed it.

The `spec` body (shown as YAML for readability; the API takes JSON):

```yaml
slug: kubernetes-basics
spec:
  title: "Kubernetes Basics"
  description: "..."
  hidden: false
  category: "kubernetes"
  difficulty: "beginner"
  modules:
    - name: "What is K8s"
      type: "text"
      content: |
        Kubernetes is a container orchestrator.
    - name: "From a repo"
      type: "text"
      src: "https://github.com/user/repo"
      ref: "main"
      path: "lessons/intro.md"
```

A module's body is `content` (markdown stored on the module) **or** the
`src`/`ref`/`path` triple (a file in a git repo). Git wins when both are set,
so writers clear one when they set the other.

## Markdown import/export

A whole course also round-trips through one markdown document — the path the
admin UI leads with, since it needs no repository:

```markdown
---
slug: kubernetes-basics
title: Kubernetes Basics
category: kubernetes
split: h2
---

## What is Kubernetes

A container orchestrator.

## Knowledge check

<!--pupitre
type: quiz
passingScore: 80
questions: [...]
-->
```

- Frontmatter is the course definition, keyed exactly like the admin API's `spec`.
- Modules are cut at the heading level named by `split` (`none`, `h1`…`h6`); a
  request may override it.
- A `<!--pupitre …-->` comment under a heading carries whatever markdown cannot
  express (type, git source, quiz questions, skills, prerequisites). No renderer
  shows it, so an exported course stays readable as plain markdown.
- Export picks a heading level no module body already uses and records it in
  `split`, so `export → import` is lossless.
- Admin routes accept 10 MB bodies (`maxAdminRequestBodyBytes`); everything else
  stays at 1 MB.

## Performance invariants

These are enforced by tests (`internal/handlers/scaling_test.go` in both
services), which assert call counts stay fixed as the data grows. Breaking
one of them fails a test, not just a benchmark.

- **No per-item I/O.** A handler rendering N things issues a number of
  queries and HTTP calls that does not depend on N. When you need data for a
  set, add a batched repository method or batch endpoint; never loop.
- **One progress read per request.** Course-service loads the learner's
  course progress once, via `learnerView`, and passes it down. Do not fetch
  viewed lessons, module progress or the course summary separately.
- **Outbound HTTP goes through the shared `internal/httpx`.** It carries the timeouts
  and the shared connection pool. `http.DefaultClient` has neither: no
  timeout at all, and two idle connections per host.
- **Large results stream.** CSV exports write rows as they are read
  (`ExportRepository.StreamRows`); nothing accumulates the whole result set
  in memory first.
- **Caches are bounded.** The user-service catalog cache and the
  course-service git cache both evict; an unbounded map keyed by user input
  is a leak.

## Build notes

- Go 1.26
- Course Service: chi, JWT, Prometheus, GORM/pgx, go-git
- User Service: chi, pgx, JWT, Prometheus, godotenv, crypto
- `DATABASE_URL` is required by both services; course-service refuses to start without it
- `SEED_DEV_COURSES=true` seeds the embedded demo catalogue (`course-service/internal/db/seed/courses/`); `overwrite` replaces it. Unset in production.
