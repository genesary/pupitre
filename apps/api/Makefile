# ─────────────────────────────────────────────────────────────────────────────
# pupitre-api — build, test and local dependencies
# (docs/specs/108-backend-operations-and-testing.md)
#
# The same commands exist as `pnpm api:*` scripts at the repository root.
# The kind targets deploy the API with the pupitre2-helm chart.
# ─────────────────────────────────────────────────────────────────────────────

# Local overrides (git-ignored), e.g. DOCKER=podman.
-include local.mk

DOCKER ?= docker
IMAGE  ?= pupitre-api:local
TEST_DATABASE_URL ?= postgres://pupitre:pupitre@localhost:5432/pupitre?sslmode=disable

.PHONY: build
build:
	go build ./...
	go vet ./...

.PHONY: test
test:
	go test -race ./...

# Needs PostgreSQL: `make up` starts it.
.PHONY: test-integration
test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -p 1 -tags integration ./...

# Needs a running API: `make up`, then `go run ./cmd/pupitre-api` (ADMIN_PASSWORD from .env).
.PHONY: test-e2e
test-e2e:
	go test -count=1 -tags e2e ./e2e/

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: run
run:
	go run ./cmd/pupitre-api

.PHONY: image
image:
	$(DOCKER) build -t $(IMAGE) .

.PHONY: up
up:
	$(DOCKER) compose up -d --wait

.PHONY: down
down:
	$(DOCKER) compose down

# ── KinD ─────────────────────────────────────────────────────────────────────
# The API in a local kind cluster, on http://localhost:8080 (NodePort); the
# frontend runs on the host with `pnpm dev`. Needs kind, kubectl and helm.

KIND_CLUSTER       ?= pupitre
HELM_RELEASE       ?= pupitre
# The published chart, or a local clone of jojo91550/pupitre2-helm.
HELM_CHART         ?= oci://ghcr.io/jojo91550/charts/pupitre
HELM_CHART_VERSION ?=
HELM_VALUES        ?= infra/kind/kind-values.yaml

.PHONY: kind-create
kind-create:
	kind create cluster --name $(KIND_CLUSTER) --config infra/kind/kind-config.yaml

.PHONY: kind-delete
kind-delete:
	-kind delete cluster --name $(KIND_CLUSTER)

.PHONY: kind-load
kind-load:
	$(DOCKER) save $(IMAGE) | kind load image-archive /dev/stdin --name $(KIND_CLUSTER)

.PHONY: helm-install
helm-install:
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		$(if $(HELM_CHART_VERSION),--version $(HELM_CHART_VERSION)) \
		--values $(HELM_VALUES)

.PHONY: helm-delete
helm-delete:
	-helm uninstall $(HELM_RELEASE)

# Rebuilds the image and restarts the API on it.
.PHONY: kind-dev
kind-dev: image kind-load helm-install
	kubectl rollout restart deploy/$(HELM_RELEASE)-api
	kubectl rollout status deploy/$(HELM_RELEASE)-api --timeout=120s
	@echo "API on http://localhost:8080; run 'pnpm dev' from the repository root."
	@echo "Admin password: kubectl get secret $(HELM_RELEASE)-secrets -o jsonpath='{.data.ADMIN_PASSWORD}' | base64 -d"

# Replaces the demo catalogue with the embedded definitions.
.PHONY: seed-courses
seed-courses:
	kubectl set env deploy/$(HELM_RELEASE)-api SEED_DEV_COURSES=overwrite
	kubectl rollout status deploy/$(HELM_RELEASE)-api --timeout=120s
	kubectl set env deploy/$(HELM_RELEASE)-api SEED_DEV_COURSES=true
