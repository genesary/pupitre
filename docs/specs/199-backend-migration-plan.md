# 199 - Backend: migration plan

The order in which the Go backend of `genesary/pupitre` becomes the monolith of `apps/api`. Each
batch is one feature branch and one pull request, squash merged; each leaves every test green.

## B0 - Import as is

Branch `feature/api-import`.

- Copy, without history, at `3cdf7b2`: `user-service/`, `course-service/`, `checker-service/`,
  `internal/`, `e2e/`, `examples/`, `infra/`, `go.mod`, `go.sum`, `.golangci.yml`, `Makefile`
  into `apps/api/`, keeping their relative layout. Not copied: `frontend/`, `docs/`, `helm`,
  `pupitre-desktop`, `.github/`, `.claude/`.
- Rewrite the module path to `github.com/jojo91550/pupitre2/apps/api`; nothing else.
- Commit message names the source: `feat(api): import the Go backend of genesary/pupitre at 3cdf7b2`.
- Done when `go build ./...`, `go vet ./...`, `golangci-lint run ./...` and `go test ./... -race`
  pass in `apps/api`, with the three services still separate.

## B1 - One binary

Branch `feature/api-monolith`.

- `internal/platform`: the root `internal/` packages, then the merged `config`
  ([101](101-backend-configuration.md)).
- Move `user-service/internal` to `internal/users`, `course-service/internal` to
  `internal/courses`, `checker-service/internal/checker` to `internal/checker`.
- The interfaces of [104](104-backend-in-process-calls.md), their implementations, and the fakes
  replacing the `httptest` stand-ins in tests.
- `internal/server`: migrations ([102](102-backend-data-and-migrations.md)), one router
  ([103](103-backend-http-api.md)), the adapters, start-up; `cmd/pupitre-api/main.go`.
- The three old `main` packages still build, with HTTP implementations of the interfaces, until B2
  removes them.
- Done when the acceptance criteria of 100 to 107 pass: route inventory test, schema test, Go
  e2e against one binary, and a manual run of the frontend against the monolith with Keycloak.

## B2 - Clean-up

Branch `chore/api-cleanup`.

- Delete the old `main.go` files, the internal handlers, `internalauth`, the `httpx` user-service
  client, the `/evaluate` and `/check-step` handlers, the three Dockerfiles and the per-service
  config packages.
- One `Dockerfile`, `compose.yaml` (PostgreSQL, Keycloak and its test realm), the `Makefile` for
  one image; root scripts `pnpm api:*`; the single-target dev proxy and
  [004](004-api-and-routing.md) ([108](108-backend-operations-and-testing.md)). The kind and Helm
  targets wait for the chart of B4.
- The Go e2e suite targets `API_URL`; the tests of the internal routes become tests of the routes
  that replaced them.
- `AGENTS.md` and `README.md` describe the API; the CI workflows.
- A Playwright suite runs against the monolith (`e2e/backend`), with the `compose.yaml`
  PostgreSQL and Keycloak and the demo seed, besides the mock suite: each role signs in through
  the real Keycloak and opens its pages, and a learner takes a course to the badge. The mock
  suite keeps the cases that need its fixtures (XSS accounts, prepared sessions).
- Done when no `USER_SERVICE_URL`, `COURSE_SERVICE_URL`, `CHECKER_SERVICE_URL`,
  `INTERNAL_SERVICE_SECRET` or `/internal/` route remains, and Playwright passes against the
  monolith.

## B3 - Fixes

One branch per defect of [190](190-backend-defects.md) (`bugfix/api-<name>`), security first: B4,
B5, B6, B7, B12, B11, B10, then the others. Each one updates 190.

## B4 - Deployment

- First, the images: the frontend image and the release workflows that publish both images to
  GHCR on a tag ([108](108-backend-operations-and-testing.md), "Releases"). A first tag gives the
  chart versions to pin.
- The Helm chart, [`jojo91550/pupitre2-helm`](https://github.com/jojo91550/pupitre2-helm),
  deploys one API and the frontend image ([108](108-backend-operations-and-testing.md),
  "Deployment"); it mounts the uploads volume ([190](190-backend-defects.md) B22).
  `apps/api/infra/kind` and the kind targets of the `Makefile` deploy it locally.
- Switch-over on an existing installation: one `helm upgrade` of the release with the converted
  values creates the API on the same database, routes `/api` and `/uploads` to it and removes the
  three services. Rollback is `helm rollback`: the schema is unchanged but for the manager group
  mapping, so the old services still work. The chart's `README.md` has the steps.
- Done when the chart installs on an empty cluster and upgrades a release of the reference chart
  with its data, and `helm lint --strict` and `helm unittest` pass in its CI.

## After

The desktop application (`pupitre-desktop`) comes after the backend, as planned.

## Rules

- [`AGENTS.md`](../../AGENTS.md) applies: commit and push only when asked, feature branches,
  Conventional Commits, no direct work on `main`.
- A batch that finds behaviour not described in 100 to 108 adds it to the specs before changing it.
