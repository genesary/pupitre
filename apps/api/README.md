# apps/api

The Go backend of pupitre, imported without its history from
[`genesary/pupitre`](https://github.com/genesary/pupitre) at commit `3cdf7b2`, and merged into one
application: `cmd/pupitre-api` serves every route of user-service, course-service and
checker-service on one port, with the domains calling each other in-process. See the backend specs
[`100`-`199`](../../docs/specs/100-backend-scope-and-architecture.md) and the plan in
[`199`](../../docs/specs/199-backend-migration-plan.md).

| Path | Content |
| --- | --- |
| `cmd/pupitre-api` | The entry point |
| `internal/server` | Wiring: migrations, router, in-process adapters, start-up |
| `internal/users`, `internal/courses`, `internal/checker` | The three domains, as they were in their services |
| `internal/platform` | Shared plumbing: merged configuration, JWT guards, HTTP helpers, metrics |
| `e2e` | Go end-to-end tests against a running API (`API_URL`) |
| `compose.yaml`, `infra/keycloak` | Local PostgreSQL and Keycloak with a test realm |
| `infra/kind` | A local kind cluster running the API through the [`pupitre2-helm`](https://github.com/jojo91550/pupitre2-helm) chart |
| `infra/manifests` | A standalone PostgreSQL manifest |

From the repository root:

```sh
pnpm api:up      # PostgreSQL and Keycloak
pnpm api:dev     # the API on http://localhost:8080, settings in apps/api/.env
pnpm api:test    # go test -race ./...
pnpm api:lint    # golangci-lint run ./...
pnpm api:build   # go build and go vet
```

Or, in `apps/api`, the `Makefile`: `make build test lint`, `make test-integration` (needs
PostgreSQL), `make test-e2e` (needs a running API), `make image`.

The API in a local kind cluster, with `kind`, `kubectl` and `helm` installed:

```sh
make kind-create       # the cluster, with the API's NodePort on localhost:8080
make kind-dev          # builds the image, loads it, installs the chart, waits for the API
pnpm dev               # from the repository root: the frontend, proxied to the cluster's API
make kind-delete
```

The chart comes from `oci://ghcr.io/jojo91550/charts/pupitre`; `HELM_CHART` points at a local
clone instead (`make helm-install HELM_CHART=../../../pupitre2-helm`).
