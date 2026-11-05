# Contributing — Dev Environment Setup

## Prerequisites

- [Node.js](https://nodejs.org/) 24 (`.nvmrc`) and pnpm, enabled with `corepack enable`
- [Go](https://go.dev/dl/) 1.26+ and [Docker](https://docs.docker.com/engine/install/), to run the backend
- Optional, to run the API in a local cluster: [KinD](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), [Helm](https://helm.sh/docs/intro/install/) v3 and [kubectl](https://kubernetes.io/docs/tasks/tools/)

[mise](https://mise.jdx.dev/) installs the tool versions of `mise.toml` with `mise install`.

---

## Quick start

### Without a backend

The mock mode answers `/api/*` in the browser:

```bash
corepack enable
pnpm install
pnpm dev:mock
```

Open <http://localhost:3000>.

### With the backend

```bash
pnpm api:up    # PostgreSQL and Keycloak (apps/api/compose.yaml)
pnpm api:dev   # pupitre-api on http://localhost:8080 (settings: apps/api/.env)
pnpm dev       # http://localhost:3000, /api and /uploads proxied to the API
```

`pnpm api:dev` creates `apps/api/.env` from `.env.example` when it is missing. Both `pnpm dev` and
`pnpm dev:mock` start the two apps: open the admin app through <http://localhost:3000/admin>, never
on its own port.

Sign in with a test account of the local Keycloak realm: `alice` (student), `marc.manager` (to be
made manager in `/admin/users`) or `admin.kc` (admin). The passwords are in
`apps/api/infra/keycloak/pupitre-realm.json`.

To stop the dependencies:

```bash
pnpm api:down
```

---

## Before opening a pull request

```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
```

When `apps/api` changes:

```bash
pnpm api:lint
pnpm api:test
pnpm api:build
```

| Command | What it runs |
|---|---|
| `pnpm test:e2e` | Playwright against the mock mode (first time: `pnpm exec playwright install chromium`) |
| `pnpm test:e2e:backend` | Playwright against the real backend, after `pnpm api:up` |
| `make test-integration` (in `apps/api`) | Go integration tests, after `pnpm api:up` |

`pnpm api:test` uses the Go race detector, which needs cgo: on Windows without a C compiler, run
`go test ./...` in `apps/api` instead.

### Git

- Branches follow [Conventional Branch](https://conventional-branch.github.io/) (`feature/…`, `bugfix/…`, `chore/…`).
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/) and are signed.
- Changes reach `main` through a pull request.

`mise run setup:prek` installs the Git hooks of `.pre-commit-config.yaml`; the README describes them.

---

## Courses

`SEED_DEV_COURSES=true` is set in `apps/api/.env.example` and in the KinD values, so the API seeds a
demo catalogue of 17 courses on startup. The definitions live in
`apps/api/internal/courses/db/seed/courses/` and are embedded in the binary.

| Value | Behaviour |
|---|---|
| `true` | Creates only the courses that do not exist yet: edits made through the admin UI survive a restart |
| `overwrite` | Replaces every seed course, discarding local edits to them |

Anything else, including unset, disables seeding, so a production deployment cannot pick up demo
content by accident.

To create your own course, use the admin UI at `/admin/courses` or the API: see
[`docs/guides/admin-api-courses.md`](docs/guides/admin-api-courses.md). The format of a course is in
[`docs/Course.md`](docs/Course.md) and [`docs/Module.md`](docs/Module.md).

---

## Running the API in a local cluster

The KinD targets of `apps/api/Makefile` deploy the API and PostgreSQL with the
[`pupitre-helm`](https://github.com/genesary/pupitre-helm) chart. The frontend still runs on the
host, with `pnpm dev`. From `apps/api`:

```bash
make kind-create    # creates the cluster, named pupitre
make kind-dev       # builds the image, loads it, deploys, restarts the API
```

The API answers on <http://localhost:8080>. `make kind-dev` also rebuilds and restarts the API after
a change. To use a local clone of the chart instead of the published one, add
`HELM_CHART=<path to the clone>`.

| Command | What it does |
|---|---|
| `make kind-create` | Create the KinD cluster |
| `make kind-delete` | Delete the KinD cluster |
| `make image` | Build the API image |
| `make kind-load` | Load the image into the cluster |
| `make helm-install` | Deploy or upgrade the chart |
| `make helm-delete` | Uninstall the release |
| `make kind-dev` | `image`, `kind-load`, `helm-install`, then restart the API |
| `make seed-courses` | Re-run the course seed in overwrite mode |

The password of the bootstrapped admin account (`admin@pupitre.local`) is generated at install time:

```bash
kubectl get secret pupitre-secrets -o jsonpath='{.data.ADMIN_PASSWORD}' | base64 -d
```

### Private course repositories

If course modules reference a private git repository, create the secret the chart mounts:

```bash
kubectl create secret generic course-repo-secret \
  --from-file=git-credentials.yaml=./git-credentials.yaml
```

See `apps/api/infra/examples/course-service/course-secret.yaml` for the format.

---

## Troubleshooting

**The login page shows no Keycloak button**: OIDC is not enabled. Locally, check `OIDC_ENABLED=true`
in `apps/api/.env` and that `pnpm api:up` is running. See [`docs/SSO.md`](docs/SSO.md).

**The API does not start and asks for `JWT_SECRET`**: it refuses a secret shorter than 32 characters
and the old public default.

**Connection refused to PostgreSQL**: the database is not ready yet. Run `pnpm api:up`, which waits
for it, then start the API again.

**Port already in use**: the local stack uses 3000 and 3001 (the apps), 8080 (the API), 5432
(PostgreSQL) and 8180 (Keycloak).

**Git content not loading**: the API cannot reach the repository, or has no token for it. Check
`GIT_TOKEN` or `GIT_CREDENTIALS_PATH` locally, and the `course-repo-secret` secret in a cluster.

**Configuration change in a cluster**: restart the API to pick it up. See
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) (section "Configuration").

```bash
kubectl rollout restart deploy/pupitre-api
```
