# 100 - Backend: scope and architecture

The Go backend of [`genesary/pupitre`](https://github.com/genesary/pupitre) (commit `3cdf7b2`) moves
into this repository, under `apps/api`, and becomes **one application** (a modular monolith)
instead of three services. Specs 100 to 199 describe that move. Specs 000 to 099 still describe
what the frontend does, and the backend must keep answering them exactly as today.

## Why one application

| Today | Problem | In the monolith |
| --- | --- | --- |
| Three binaries: user-service (8081), course-service (8082), checker-service (8083) | Three images, three deployments, three configs | One binary, one image, one config |
| They share **one** PostgreSQL database (same `DATABASE_URL` in the Helm chart) but talk over HTTP | Every course page costs HTTP round-trips between services | Function calls in the same process |
| A shared secret (`X-Internal-Secret`, default `change-me-internal-secret`) protects 11 internal routes and checker-service | A leaked or default secret opens them (audit C1) | No internal route left: nothing to protect |
| The frontend's proxy splits `/api` between two services by prefix ([004](004-api-and-routing.md)) | The table must stay identical in Vite, Helm and every front door | One `/api` target |
| The same JWT secret must be set on two services | Drift breaks sign-in | One setting |

What does **not** change: the database (already shared), the tables, every public route, every
request and response body, every status code and error message. The frontend is not touched.

## Scope

| From `genesary/pupitre` | Fate |
| --- | --- |
| `user-service/`, `course-service/`, `checker-service/` | Merged into `apps/api` ([this spec](#target-layout)) |
| `internal/` (JWT, internal auth, error envelope, HTTP client, metrics) | `apps/api/internal/platform/`; `internalauth` is deleted once no internal route is left |
| `e2e/` (Go end-to-end tests) | `apps/api/e2e/`, pointed at the single binary ([108](108-backend-operations-and-testing.md)) |
| `examples/` (sample courses, quizzes, Kubernetes manifests) | `apps/api/examples/` |
| `docs/` (architecture, course, module, skills, SSO, labs) | Rewritten as specs 100-107, in English; the French originals are not kept |
| `user-service/openapi.{json,yaml}` | Not kept: they cover about a quarter of the routes ([103](103-backend-http-api.md) is the inventory) |
| `Makefile`, `infra/` (kind cluster, manifests) | Adapted to one image ([108](108-backend-operations-and-testing.md)) |
| `.golangci.yml`, `renovate.json`, `cliff.toml`, `.github/` | `.golangci.yml` kept for `apps/api`; CI rewritten for the monorepo ([108](108-backend-operations-and-testing.md)) |
| `frontend/` (Astro) | **Not imported**: `apps/app` and `apps/admin` replace it |
| `helm` and `pupitre-desktop` (separate repositories) | Unchanged here; the chart is updated in its own repository ([108](108-backend-operations-and-testing.md)) |

The import is a plain copy of the files at `3cdf7b2`, without the git history. Its commit names
the source commit; the history stays in `genesary/pupitre`.

## Target layout

```
apps/api/
├── go.mod                       # module github.com/jojo91550/pupitre2/apps/api, Go 1.26
├── .golangci.yml
├── Dockerfile                   # one image, built from apps/api
├── cmd/pupitre-api/main.go      # the only entry point
├── internal/
│   ├── platform/                # shared plumbing, from the root internal/ of genesary/pupitre
│   │   ├── config/              # merged configuration (101)
│   │   ├── middleware/          # JWT and role guards
│   │   ├── httperr/  httpx/  metrics/  utils/
│   ├── server/                  # wiring: one pool, migrations (102), router (103), in-process
│   │                            # adapters (104), start-up and shutdown
│   ├── users/                   # was user-service/internal: auth, users, groups, enrollments,
│   │                            # progress, gamification, sessions, settings, patterns, exports
│   ├── courses/                 # was course-service/internal: catalogue, content, git, quizzes,
│   │                            # labs, paths, markdown import/export, uploads
│   └── checker/                 # was checker-service/internal: GitLab state + OPA engine, with
│                                # its handlers/ and config/ (107)
├── e2e/
└── examples/
```

Rules:

- **Domains keep their packages.** `users/`, `courses/` and `checker/` keep the inner layout they
  had in their service (`handlers/`, `repository/`, `models/`, …), so a file of `genesary/pupitre`
  can be found by its path. They are moved, not rewritten.
- **A domain never imports another domain.** Calls across domains go through the small interfaces
  of [104](104-backend-in-process-calls.md), declared by the caller; `internal/server` implements
  them with adapters that call the domain owning the data. It is the only package that imports two
  domains.
- **Each domain keeps its configuration package.** The merged loader of
  [101](101-backend-configuration.md) fills them from one file and one environment.
- **One process, one database pool, one router, one `/health`, one `/metrics`.**
- The module path changes from `github.com/genesary/pupitre` to
  `github.com/jojo91550/pupitre2/apps/api`; imports are rewritten accordingly.

## Invariants

These hold at every step of [199](199-backend-migration-plan.md) and are checked by tests
([108](108-backend-operations-and-testing.md)):

1. **API contract.** Every route of [103](103-backend-http-api.md) marked "Kept" answers with the
   same method, path, guard, body limit, status codes, JSON field names and error messages as at
   `3cdf7b2`. The frontend's `api-types.ts` and mock mode stay valid.
2. **Data.** The schema produced by the migrations is the union of the two services' schemas, with
   the same tables, columns and indexes ([102](102-backend-data-and-migrations.md)). An existing
   database is used as is: no data migration.
3. **Behaviour.** Every rule described by the backend docs of `genesary/pupitre` is kept: course
   and module definitions, git resolution, quiz scoring and cooldowns, completion and badges, XP,
   skills, paths, groups and roles, settings, sign-in methods, labs ([105](105-backend-authentication.md),
   [106](106-backend-courses-and-content.md), [107](107-backend-labs-and-checker.md)).
4. **Performance.** The performance invariants of `genesary/pupitre` stay enforced by tests: no
   per-item I/O, one progress read per request, outbound HTTP through `httpx`, streamed CSV
   exports, bounded caches. In-process calls only remove costs.
5. **Defects are fixed on purpose, not by accident.** A known defect ([190](190-backend-defects.md))
   keeps its current behaviour during the move; its fix is a separate, specified change.

## Risks

| Risk | Mitigation |
| --- | --- |
| A Rego policy now runs in the API process: a slow or hostile policy could hurt every request | The limits of checker-service are kept: restricted OPA capabilities, 10 s evaluation timeout, 5 MB input cap; policies come from course repositories, which only admins set ([107](107-backend-labs-and-checker.md)) |
| One process means one scaling unit | The services were already stateless above one database; the monolith scales horizontally the same way. The git cache and the catalog cache are per replica, as before |
| A crash takes down everything | Same `Recoverer` middleware on every route; graceful shutdown as today ([108](108-backend-operations-and-testing.md)) |
| Detached calls (fire-and-forget) change timing | They stay detached, with the same "never fails the response" rule ([104](104-backend-in-process-calls.md)) |

## Acceptance criteria

- `go build ./...`, `go vet ./...`, `golangci-lint run ./...` and `go test ./... -race` pass in
  `apps/api`.
- One binary serves every "Kept" route of [103](103-backend-http-api.md); the Go e2e suite and the
  frontend's Playwright suite pass against it.
- No `USER_SERVICE_URL`, `COURSE_SERVICE_URL`, `CHECKER_SERVICE_URL` or `INTERNAL_SERVICE_SECRET`
  remains in the code or the configuration.

## Sources

`genesary/pupitre` at `3cdf7b2`: `AGENTS.md`, `docs/ARCHITECTURE.md`, `go.mod`,
`user-service/main.go`, `course-service/main.go`, `checker-service/cmd/main.go`, the three
`internal/config` packages; `pupitre-helm` `values.yaml` and `templates/secret.yaml` (one
`DATABASE_URL` for every service).
