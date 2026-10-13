# ─────────────────────────────────────────────────────────────────────────────
# Pupitre Platform — KinD Dev Environment
# ─────────────────────────────────────────────────────────────────────────────

KIND_CLUSTER   ?= pupitre
HELM_DIR       := helm
HELM_RELEASE   ?= pupitre

# Local overrides (git-ignored) — create local.mk to set KIND_CLUSTER, HELM_RELEASE, etc.
-include local.mk
HELM_VALUES    := infra/kind/kind-values.yaml
PORT_FWDS_LOG  := /tmp/pupitre-port-forwards.log
DOCKER         ?= podman

REGISTRY        := ghcr.io/genesary
IMAGE_COURSE    := localhost/pupitre-course-service:local
IMAGE_USER      := localhost/pupitre-user-service:local
IMAGE_FRONTEND  := localhost/pupitre-frontend:local
IMAGE_CHECKER   := localhost/pupitre-checker-service:local

# ── KinD ────────────────────────────────────────────────────────────────────

.PHONY: kind-create
kind-create:
	kind create cluster --name $(KIND_CLUSTER) --config infra/kind/kind-config.yaml

.PHONY: kind-delete
kind-delete:
	-kind delete cluster --name $(KIND_CLUSTER)

# ── Docker ───────────────────────────────────────────────────────────────────

.PHONY: docker-build docker-build-course docker-build-user docker-build-frontend docker-build-checker

docker-build: docker-build-course docker-build-user docker-build-frontend docker-build-checker

# The Go services share the root go.mod and internal/, so they build from the
# repository root with an explicit Dockerfile path. The frontend is unchanged.
docker-build-course:
	$(DOCKER) build -f course-service/Dockerfile -t $(IMAGE_COURSE) .

docker-build-user:
	$(DOCKER) build -f user-service/Dockerfile -t $(IMAGE_USER) .

docker-build-frontend:
	$(DOCKER) build -t $(IMAGE_FRONTEND) frontend

docker-build-checker:
	$(DOCKER) build -f checker-service/Dockerfile -t $(IMAGE_CHECKER) .

# ── Kind load ───────────────────────────────────────────────────────────────

.PHONY: kind-load kind-load-course kind-load-user kind-load-frontend kind-load-checker

kind-load: kind-load-course kind-load-user kind-load-frontend kind-load-checker

kind-load-course:
	$(DOCKER) save $(IMAGE_COURSE) | kind load image-archive /dev/stdin --name $(KIND_CLUSTER)

kind-load-user:
	$(DOCKER) save $(IMAGE_USER) | kind load image-archive /dev/stdin --name $(KIND_CLUSTER)

kind-load-frontend:
	$(DOCKER) save $(IMAGE_FRONTEND) | kind load image-archive /dev/stdin --name $(KIND_CLUSTER)

kind-load-checker:
	$(DOCKER) save $(IMAGE_CHECKER) | kind load image-archive /dev/stdin --name $(KIND_CLUSTER)

# ── Quick rebuild + reload (single service) ────────────────────────────────

.PHONY: rebuild rebuild-course rebuild-user rebuild-frontend rebuild-checker

rebuild: rebuild-course rebuild-user rebuild-frontend rebuild-checker

rebuild-course: docker-build-course kind-load-course
	kubectl rollout restart deployment/$(HELM_RELEASE)-course-service
	kubectl rollout status deployment/$(HELM_RELEASE)-course-service --timeout=120s

rebuild-user: docker-build-user kind-load-user
	kubectl rollout restart deployment/$(HELM_RELEASE)-user-service
	kubectl rollout status deployment/$(HELM_RELEASE)-user-service --timeout=120s

rebuild-frontend: docker-build-frontend kind-load-frontend
	kubectl rollout restart deployment/$(HELM_RELEASE)-frontend
	kubectl rollout status deployment/$(HELM_RELEASE)-frontend --timeout=120s

rebuild-checker: docker-build-checker kind-load-checker
	kubectl rollout restart deployment/$(HELM_RELEASE)-checker-service
	kubectl rollout status deployment/$(HELM_RELEASE)-checker-service --timeout=120s

# ── Helm ─────────────────────────────────────────────────────────────────────

.PHONY: helm-deps
helm-deps:
	helm dependency update $(HELM_DIR)

.PHONY: helm-install
helm-install:
	helm upgrade --install $(HELM_RELEASE) $(HELM_DIR) --values $(HELM_VALUES)

.PHONY: helm-delete
helm-delete:
	-helm uninstall $(HELM_RELEASE)

# ── Dev courses ──────────────────────────────────────────────────────────────
#
# The demo catalogue lives in course-service/internal/db/seed/courses/ and is
# embedded in the binary. course-service seeds it on startup when
# SEED_DEV_COURSES is set, which the KinD values files already do.

.PHONY: seed-courses
seed-courses:
	@echo "Re-seeding dev courses (overwrite mode)..."
	kubectl set env deploy/$(HELM_RELEASE)-course-service SEED_DEV_COURSES=overwrite
	kubectl rollout status deploy/$(HELM_RELEASE)-course-service --timeout=120s
	kubectl set env deploy/$(HELM_RELEASE)-course-service SEED_DEV_COURSES=true
	@echo "Dev courses re-seeded from the embedded definitions."

# ── Git secret ───────────────────────────────────────────────────────────────

.PHONY: create-git-secret
create-git-secret:
	@echo "Create a git-credentials.yaml file first, then run:"
	@echo "  kubectl create secret generic course-repo-secret \\"
	@echo "    --from-file=git-credentials.yaml=./git-credentials.yaml"
	@echo ""
	@echo "See infra/examples/course-service/course-secret.yaml for the format."

# ── Port-forwards ───────────────────────────────────────────────────────────

.PHONY: port-forward
port-forward:
	kubectl port-forward svc/$(HELM_RELEASE)-course-service  18082:8082 &
	kubectl port-forward svc/$(HELM_RELEASE)-user-service   18081:8081 &
	kubectl port-forward svc/$(HELM_RELEASE)-frontend       3000:3000  &
	kubectl port-forward svc/$(HELM_RELEASE)-checker-service 18083:8083 &
	sleep 2
	@echo "Port-forwards started (course:18082, user:18081, frontend:3000, checker:18083)"

.PHONY: port-forward-stop
port-forward-stop:
	-pkill -f "kubectl port-forward" 2>/dev/null
	@echo "Port-forwards stopped"

# ── Full lifecycle ────────────────────────────────────────────────────────────

.PHONY: dev
dev: kind-delete kind-create docker-build kind-load helm-install
	@echo ""
	@echo "Waiting for deployments to be ready..."
	@kubectl rollout status deploy/$(HELM_RELEASE)-course-service  --timeout=120s
	@kubectl rollout status deploy/$(HELM_RELEASE)-user-service   --timeout=120s
	@kubectl rollout status deploy/$(HELM_RELEASE)-frontend       --timeout=120s
	@kubectl rollout status deploy/$(HELM_RELEASE)-checker-service --timeout=120s
	@kubectl rollout status statefulset/$(HELM_RELEASE)-postgresql --timeout=60s
	@echo ""
	@echo "=== Deployment ready ==="
	@echo "Run 'make port-forward' to expose services locally."
	@echo ""
	@echo "  Admin login: admin@pupitre.local"
	@echo "  Password:    kubectl get secret $(HELM_RELEASE)-secrets \\"
	@echo "                 -o jsonpath='{.data.ADMIN_PASSWORD}' | base64 -d"
	@echo "  Courses:     17 demo courses seeded (SEED_DEV_COURSES=true)"
	@echo "  Frontend:    http://localhost:3000"
	@echo "  Course API:  http://localhost:18082"
	@echo "  User API:    http://localhost:18081"
	@echo ""
	@echo "Next: create a git secret if you use private repos:"
	@echo "  make create-git-secret"

.PHONY: clean
clean: helm-delete kind-delete
	@echo "Cleaned up"

# ── Logs ────────────────────────────────────────────────────────────────────

.PHONY: logs
logs:
	@echo "=== course-service ==="
	@kubectl logs deploy/$(HELM_RELEASE)-course-service --tail=20 2>&1 | grep -v health
	@echo ""
	@echo "=== user-service ==="
	@kubectl logs deploy/$(HELM_RELEASE)-user-service --tail=20 2>&1 | grep -v health
	@echo ""
	@echo "=== checker-service ==="
	@kubectl logs deploy/$(HELM_RELEASE)-checker-service --tail=20 2>&1 | grep -v health
	@echo ""
	@echo "=== frontend ==="
	@kubectl logs deploy/$(HELM_RELEASE)-frontend --tail=10 2>&1
	@echo ""
	@echo "=== postgresql ==="
	@kubectl logs statefulset/$(HELM_RELEASE)-postgresql --tail=5 2>&1 || true

# ── OpenAPI ──────────────────────────────────────────────────────────────────
#
# Prerequisites:
#   go install github.com/swaggo/swag/cmd/swag@latest

.PHONY: openapi/gen openapi/gen/course-service openapi/gen/user-service

openapi/gen: openapi/gen/course-service openapi/gen/user-service
	@echo "OpenAPI JSON files regenerated from code."

openapi/gen/course-service:
	@which swag > /dev/null 2>&1 || (echo "swag not found — run: go install github.com/swaggo/swag/cmd/swag@latest" && exit 1)
	@echo "Generating course-service/openapi.json from code..."
	@swag init -g main.go -d ./course-service,./internal --output ./course-service --outputTypes json --parseInternal --quiet && mv course-service/swagger.json course-service/openapi.json

openapi/gen/user-service:
	@which swag > /dev/null 2>&1 || (echo "swag not found — run: go install github.com/swaggo/swag/cmd/swag@latest" && exit 1)
	@echo "Generating user-service/openapi.json from code..."
	@swag init -g main.go -d ./user-service,./internal --output ./user-service --outputTypes json --parseInternal --quiet && mv user-service/swagger.json user-service/openapi.json

# ── Go Tests ────────────────────────────────────────────────────────────────

.PHONY: go/test

# One root module: a single run covers every service and the shared packages
# under internal/.
go/test:
	@echo "=== go tests ==="
	@CGO_ENABLED=1 go test ./... -race -covermode=atomic -coverpkg=./... -coverprofile=coverage.out -count=1 && go tool cover -func=coverage.out | tail -1

# ── Go Integration Tests ───────────────────────────────────────────────────
#
# The `internal/repository` and `internal/db` packages of course-service and
# user-service are GORM/Postgres code; their tests are behind the `integration`
# build tag and need a live Postgres. Point TEST_DATABASE_URL at one, e.g.:
#
#   docker run -d --rm -e POSTGRES_PASSWORD=pupitre -e POSTGRES_USER=pupitre \
#     -e POSTGRES_DB=pupitre -p 55432:5432 postgres:18-alpine
#   TEST_DATABASE_URL=postgres://pupitre:pupitre@localhost:55432/pupitre?sslmode=disable \
#     make go/test/integration

.PHONY: go/test/integration

go/test/integration:
	@echo "=== integration tests ==="
	@CGO_ENABLED=1 go test -tags=integration -p 1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.integration.out ./... -count=1 && go tool cover -func=coverage.integration.out | tail -1

# ── Go Lint ─────────────────────────────────────────────────────────────────

.PHONY: go/lint

go/lint:
	@echo "=== Linting ==="
	@golangci-lint run ./...

# ── Pre-commit (prek) ───────────────────────────────────────────────────────

.PHONY: precommit-setup precommit-run

precommit/setup:
	prek install

precommit/run:
	prek run --all-files

# ── E2E Tests ───────────────────────────────────────────────────────────────
#
# Runs e2e tests against services already port-forwarded to localhost.
# Requires: USER_SERVICE_URL, COURSE_SERVICE_URL (optional), CHECKER_SERVICE_URL,
#           ADMIN_EMAIL, ADMIN_PASSWORD.
# Example:
#   USER_SERVICE_URL=http://localhost:8081 \
#   COURSE_SERVICE_URL=http://localhost:8082 \
#   CHECKER_SERVICE_URL=http://localhost:8083 \
#   ADMIN_PASSWORD=<password> \
#   make e2e/test

.PHONY: e2e/test
e2e/test:
	@echo "=== E2E tests ==="
	@go test -tags e2e -v -timeout 120s ./e2e/...

# ── Status ──────────────────────────────────────────────────────────────────

.PHONY: status
status:
	@kubectl get pods
	@echo ""
	@kubectl get svc

# ── Renovate ────────────────────────────────────────────────────────────────
#
#   make renovate/validate   # lint renovate.json (Renovate container)
#   make renovate            # apply dep upgrades to the working tree
#   make renovate/preview     # show what Renovate itself would do (no writes)
#
# `make renovate` upgrades Go module + npm deps in place with native tooling so
# you get a real `git diff` to review — Renovate's own `local` platform only
# reports, it never edits a checkout. Not covered by native updaters (take these
# from the Renovate PR): container base images, GitHub Actions versions, the
# go.mod `go` directive, and Go major bumps (import-path changes).
#
#   git diff                          # review
#   git checkout -- . && git clean -fd  # discard
#
# $GITHUB_TOKEN / $GH_TOKEN (or a logged-in `gh`) lifts the GitHub API rate
# limit for renovate-preview.

RENOVATE_VERSION ?= 44
RENOVATE_IMAGE   ?= ghcr.io/renovatebot/renovate:$(RENOVATE_VERSION)
LOG_LEVEL        ?= info
GO_MODULES       ?= course-service user-service checker-service e2e

# keep-id + label=disable let the rootless container read the bind-mounted repo
# (SELinux / non-1000 uid). Docker users can override: RENOVATE_RUN_OPTS=
RENOVATE_RUN_OPTS ?= --userns=keep-id --security-opt label=disable
RENOVATE_DOCKER = $(DOCKER) run --rm $(RENOVATE_RUN_OPTS) \
	-v "$(CURDIR)":/repo -w /repo \
	-e GIT_CONFIG_COUNT=1 \
	-e GIT_CONFIG_KEY_0=safe.directory -e GIT_CONFIG_VALUE_0=/repo \
	-e LOG_LEVEL=$(LOG_LEVEL)

.PHONY: renovate renovate/validate renovate/preview

renovate/validate:
	@echo "=== Validating renovate.json ==="
	@$(RENOVATE_DOCKER) $(RENOVATE_IMAGE) renovate-config-validator --strict

renovate: renovate/go renovate/npm
	@echo
	@echo "=== Done. Review: git diff  |  Discard: git checkout -- . && git clean -fd ==="

renovate/go:
	@for m in $(GO_MODULES); do \
		echo "=== $$m: go get -u ./... && go mod tidy ==="; \
		( cd "$$m" && go get -u ./... && go mod tidy ) || exit 1; \
	done

renovate/npm:
	@echo "=== frontend: npm-check-updates -u && npm install ==="
	@cd frontend && npx --yes npm-check-updates -u && npm install

renovate/preview:
	@echo "=== Renovate preview (local platform, no changes written) ==="
	@$(RENOVATE_DOCKER) \
		-e RENOVATE_PLATFORM=local \
		-e RENOVATE_ONBOARDING=false \
		-e RENOVATE_REQUIRE_CONFIG=optional \
		-e RENOVATE_CONFIG_FILE=/repo/renovate.json \
		-e GITHUB_COM_TOKEN="$${GITHUB_TOKEN:-$${GH_TOKEN:-$$(gh auth token 2>/dev/null)}}" \
		$(RENOVATE_IMAGE)
