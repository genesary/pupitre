# 108 - Backend: operations, tooling and tests

How the monolith is built, run, tested and deployed, next to the frontend in this repository.

## Repository

- `apps/api` is a Go module of its own (`go.mod`, `go.sum`, `.golangci.yml`), with no
  `package.json`: pnpm ignores it, so a frontend contributor needs no Go toolchain.
- Root scripts call it through `scripts/api.mjs`:

  | Script | Runs, in `apps/api` |
  | --- | --- |
  | `pnpm api:up` / `pnpm api:down` | `docker compose up -d --wait` / `down` |
  | `pnpm api:dev` | Builds `bin/pupitre-api` and runs it, on `http://localhost:8080`; creates `.env` from `.env.example` the first time |
  | `pnpm api:test` | `go test -race ./...` |
  | `pnpm api:lint` | `golangci-lint run ./...` |
  | `pnpm api:build` | `go build ./...` and `go vet ./...` |

  `api:dev` runs a built binary rather than `go run`, whose program outlives it on Windows when
  it is stopped. The frontend scripts (`dev`, `test`, `build`…) do not change.
- `apps/api/Makefile` has the same targets for a shell (`build`, `test`, `test-integration`,
  `test-e2e`, `lint`, `run`, `image`, `up`, `down`).
- Go version: the one of `genesary/pupitre` (1.26); dependencies keep their versions at `3cdf7b2`
  and are updated later on purpose, not during the move.
- `.golangci.yml` is kept as is; `golangci-lint run ./...` passes in `apps/api`.

## Local development

| Command | Effect |
| --- | --- |
| `pnpm api:up` | PostgreSQL 18 on 5432 and Keycloak 26 on 8180, from `apps/api/compose.yaml` |
| `pnpm api:dev` | The API on `http://localhost:8080`, configured by `apps/api/.env` |
| `pnpm dev` | The two frontend apps, proxying `/api` and `/uploads` to the API |
| `pnpm dev:mock` | Unchanged: no backend at all ([006](006-mock-mode.md)) |

- **Keycloak** imports the test realm `apps/api/infra/keycloak/pupitre-realm.json`: client
  `pupitre` (redirects to `http://localhost:3000/*`), groups `pupitre-admins`,
  `pupitre-managers`, `pupitre-students`, and three accounts, `alice`, `marc.manager` and
  `admin.kc`, whose passwords are in the file. They are **local test values**, never deployed. A
  one-shot `keycloak-setup` container then adds the `groups` client scope that the API asks for
  (`oidc_scopes`): importing it with the realm would replace Keycloak's built-in scopes.
- The API runs on the host, not in compose, so the browser and the API reach Keycloak at the same
  address and no split-horizon setting is needed.
- `apps/api/.env.example` holds the development settings matching compose. `pupitre-api` loads
  a `.env` of its working directory without overriding the environment.
- Roles: `OIDC_GROUP_ADMINS` is not set; `pupitre-admins` is mapped to `admin` in
  `/admin/groups` and a manager is made by hand in `/admin/users`
  ([105](105-backend-authentication.md)). The Playwright backend suite does both.

The frontend's dev proxy (`apps/app/vite.config.ts`) sends `/api` and `/uploads` to **one**
target, `VITE_API_URL` (default `http://localhost:8080`). `VITE_USER_SERVICE_URL` and
`VITE_COURSE_SERVICE_URL` are gone ([004](004-api-and-routing.md)).

## Image

- One `apps/api/Dockerfile`, built from `apps/api`: build stage on `golang:1.26.10-alpine` with
  module and build caches, `CGO_ENABLED=0`, `-ldflags="-s -w"`; runtime stage `FROM scratch`
  with the CA certificates, user `1000:1000`, `EXPOSE 8080`, no `ENV` defaults (they live in the
  code, [101](101-backend-configuration.md)). About 46 MB.
- The deployment mounts the writable paths: `/tmp` (git cache) and `UPLOADS_DIR`.
- Image name `pupitre-api`. It replaces `user-service`, `course-service` and `checker-service`.
- The frontend image, `pupitre-front`: `docker/front/Dockerfile`, built from the repository root
  (`docker build -f docker/front/Dockerfile .`). A `node:24` stage installs the two apps with
  pnpm and builds them; the runtime stage is `nginx-unprivileged` (user 101), listening on
  **3000** like the reference's frontend, with no configuration at run time (the apps read none).
  `docker/front/nginx.conf` routes `/admin/*` and `/manager/*` to the admin app's page and every
  other path to the learner app's ([000](000-scope-and-stack.md)), sends the headers of
  [003](003-security.md) S6 on every response, never caches the pages and caches the hashed
  assets for a year. `/api` and `/uploads` are routed to `pupitre-api` by the ingress, not by this
  server. The root filesystem can be read-only with `/tmp` mounted. About 75 MB.

## Deployment

The Helm chart lives in its own repository,
[`jojo91550/pupitre2-helm`](https://github.com/jojo91550/pupitre2-helm), adapted from
`genesary/pupitre-helm` at `48e9ecb` (batch B4 of [199](199-backend-migration-plan.md)). Its
`README.md` holds the install and the switch-over from the reference chart; `helm unittest`
checks what follows.

- **One API**: one Deployment, Service, ServiceAccount, ConfigMap (the keys of the user-service and
  course-service ConfigMaps merged, the service URLs removed), and the optional HPA, PDB and
  NetworkPolicy, under `api.*`. Image `pupitre-api` at the chart's `appVersion`, port 8080,
  `/health` for the startup, liveness and readiness probes, `terminationGracePeriodSeconds` 30
  for the 15 s graceful shutdown. Resources and database pools are the sums of the services'.
- **The frontend**: image `pupitre-front` on port 3000, user 101, read-only root filesystem with
  `/tmp` mounted, no configuration and no egress.
- **Routing**: one table for the Ingress and the HTTPRoute: `/api`, `/uploads` and `/metrics`
  (prefix) and `/health` (exact) to the API, `/` to the frontend. `/metrics` stays public, as the
  reference chart routes it ([190](190-backend-defects.md) B15, "Kept as the reference"). Cookies reach the API unchanged, as the OAuth state check needs
  ([105](105-backend-authentication.md)).
- **Secrets**: the internal secret is gone. `JWT_SECRET` comes from `jwtExistingSecret`, from
  `jwtSecret` (refused by the chart when shorter than 32 characters or equal to the reference's
  placeholder, as the API refuses them, [190](190-backend-defects.md) B4), or is generated
  (64 characters) on install and kept across upgrades, like the admin password. A release
  upgraded from the reference chart with its placeholder gets a generated secret instead.
- **Uploads**: `UPLOADS_DIR` is `/data/uploads`, a `PersistentVolumeClaim` by default (kept on
  uninstall, `fsGroup` 1000), `ReadWriteMany` when the API has several replicas, or an existing
  claim; an emptyDir when persistence is disabled ([190](190-backend-defects.md) B22).
- **Unchanged from the reference**: `admin.*`, `sso.*` (OIDC seeding, client secrets as files,
  `groupAdmins` authoritative), `database.*`, the bundled PostgreSQL, `ingress.*`, `httpRoute.*`,
  the git token and the optional `course-repo-secret` of per-URL git credentials.
- **Switch-over**: `helm upgrade` of an existing release with the converted values creates the API
  and removes the three services, on the same database. With the bundled PostgreSQL and
  persistence, the StatefulSet is deleted first with `--cascade=orphan`: the reference puts its
  chart version in the `volumeClaimTemplates`, which an upgrade cannot change (this chart does
  not). Rollback is `helm rollback`, after the same StatefulSet deletion and after removing the
  group mappings to `manager`, which the reference's schema refuses. Both ways were run on a
  minikube cluster from a release of the reference chart with data.
- **Releases**: a tag `<version>` (no `v`) on the chart repository publishes it to
  `oci://ghcr.io/jojo91550/charts/pupitre`, as the reference does.

`apps/api/infra/kind` deploys the chart on a local kind cluster, with the bundled PostgreSQL and
the demo catalogue: `make kind-create`, then `make kind-dev` (image, load, `helm upgrade
--install`, rollout). The API answers on `http://localhost:8080` through its NodePort, so
`pnpm dev` runs the frontend against it. `HELM_CHART` names the chart: the published one by
default, or a local clone.

## Tests

| Suite | Where | Runs |
| --- | --- | --- |
| Unit and handler tests of the domains | `apps/api/internal/**` | `go test ./... -race` |
| Performance invariants (`scaling_test.go`) | users and courses domains | same, counting queries and calls |
| Wire contract (`wire_contract_test.go`) | users domain | same |
| Route inventory and guards | `internal/server/router_test.go` | same: every route of [103](103-backend-http-api.md), no other |
| Cross-domain wiring | `internal/server/wiring_test.go` | same: a learner flow across domains on fakes, failure semantics, lab check limit |
| Merged configuration | `internal/platform/config` | same |
| Schema and start-up | `internal/server/*_integration_test.go`, tag `integration` | needs PostgreSQL: same schema as the two services ([102](102-backend-data-and-migrations.md)); `Run` serves, then stops on `SIGTERM` |
| Repository and migration tests | `internal/*/repository`, `internal/*/db`, tag `integration` | needs PostgreSQL |
| Go end-to-end tests | `apps/api/e2e`, tag `e2e` | against a running binary at `API_URL`; also checks the lab check routes need a token and the service routes are gone |
| Frontend Playwright, mock | `e2e/*.spec.ts`, `pnpm test:e2e` | against the mock mode, as before |
| Frontend Playwright, backend | `e2e/backend/`, `pnpm test:e2e:backend` | against both apps, `pupitre-api`, and compose's PostgreSQL and Keycloak: real Keycloak sign-in of each role, every page of each role, a learner taking a course to the badge, GitLab lab step checks against a fake GitLab (`e2e/backend/fake-gitlab.mjs`, adapted from `ebuildy/pupitre2`; `GITLAB_BASE_URL` points the API at it, the lab's text comes from a local git repository). Written to pass again on the data of an earlier run |

The handler tests of each domain used `httptest` servers standing in for the other services. They
keep them: each domain's `standalone_test.go` holds, for the tests only, the old service's router
(with its internal routes, as shims over the in-process methods) and its HTTP clients of the other
services ([104](104-backend-in-process-calls.md)). Production has none of them.

## Continuous integration

Three workflows, in `.github/workflows`:

- `api.yaml`, on changes under `apps/api`: `golangci-lint`; `govulncheck` (known vulnerabilities
  of the code pupitre-api calls); `go build`, `go vet`, `go test -race`
  with coverage; integration tests against PostgreSQL 18; then the binary is started and the Go
  e2e suite runs against it.
- `web.yaml`, on changes outside `apps/api` and the docs: `typecheck`, `lint`, `test`, `build`,
  and Playwright against the mock mode.
- `fullstack.yaml`, on every change: `pnpm api:up`, then Playwright against the real backend.
- `containers.yaml`, on `main`, on tags `v*` and on pull requests that touch the apps, the
  packages or `docker/`: KICS scans both Dockerfiles (fails on high or medium findings), and
  both images are built. On a tag, they are pushed to GHCR and scanned by Trivy (fails on a
  critical or high vulnerability that has a fix).
- `ai-review.yaml`, on pull requests: Claude reviews the change with the project's reviewers
  (`.claude/skills/review`, `.claude/agents/security-reviewer.md` and `parity-reviewer.md`)
  and comments on the pull request: each finding with its fix, then at most five
  non-blocking best-practice suggestions. It never pushes a change. Needs the
  `CLAUDE_CODE_OAUTH_TOKEN` secret (a Claude subscription token, from `claude setup-token`) or
  the `ANTHROPIC_API_KEY` secret.
- `strix.yaml`, on pull requests: [Strix](https://github.com/usestrix/strix) runs a quick AI
  penetration test of the changed files and fails on a medium or worse finding; run by hand,
  it scans the whole repository. Needs the `LLM_API_KEY` secret; the model is the `STRIX_LLM`
  variable. Neither runs on `main`: what reaches it was reviewed in its pull request. Both are
  skipped without their secret and on pull requests from forks.
- `codeql.yaml`, on `main`, on pull requests and weekly: CodeQL (`security-extended` queries)
  on the TypeScript, the Go and the workflows; findings go to the Security tab.
  `.github/codeql/codeql-config.yml` leaves out what only gives false positives: the generated
  MSW worker, the course YAML and `go/log-injection` (zap escapes its fields). An alert checked
  and found harmless is dismissed in the Security tab with the reason.
- `.github/dependabot.yml`: one grouped update pull request per month for npm, Go modules and
  actions, for versions at least 7 days old. Dependabot alerts and security updates are turned
  on in the repository settings.
- `release.yaml`, on tags `v*`: the GitHub Release, with the changelog of the commits since the
  previous tag (git-cliff, `cliff.toml`); `-rc`, `-beta` and `-alpha` tags are pre-releases.

Both are adapted from the reference's `ci-containers.yaml` and `release.yaml`.

Third-party actions are pinned to a commit SHA, with the version in a comment; GitHub's own
(`actions/*`, `github/*`) use their major tag. Dependabot updates both.

## Releases

A release is a tag `v<major>.<minor>.<patch>` pushed on `main`, by hand, as in the reference:
`git tag v0.1.0` then `git push origin v0.1.0`, or a release published in GitHub's interface
with a new tag. It publishes:

| Image | Tags |
| --- | --- |
| `ghcr.io/<owner>/pupitre-api` | `0.1.0`, `0.1`, `0`, `latest`, short commit SHA |
| `ghcr.io/<owner>/pupitre-front` | the same |

The packages are private on their first push; a cluster that pulls them without credentials
needs them made public in the packages' settings on GitHub. The deployment chart pins a version
tag, never `latest`.

## Acceptance criteria

- On a clean clone: `pnpm install`, `pnpm api:test`, `pnpm test`, `pnpm build` pass; `pnpm api:up`,
  `pnpm api:dev` and `pnpm dev` sign in through Keycloak and show the catalogue.
- `pnpm test:e2e:backend` passes on an empty database and again on the data it left.
- `docker build apps/api` produces an image under 50 MB that answers `/health`.

## Sources

`Makefile`, `*/Dockerfile`, `infra/`, `e2e/`, `.github/workflows/` and `.golangci.yml` of
`genesary/pupitre`; `apps/app/vite.config.ts` of this repository.
