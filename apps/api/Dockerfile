# Stage 1: Build
#
# The build context is apps/api: one module, one binary.
# Build with:  docker build -t pupitre-api apps/api
FROM docker.io/library/golang:1.26.10-alpine AS builder

WORKDIR /src

# Cache module downloads: the cache mounts below keep a dependency change
# from costing a full re-download and re-compile each time.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o /out/pupitre-api ./cmd/pupitre-api

# Stage 2: Runtime
FROM scratch

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/pupitre-api /app/pupitre-api

# No ENV config defaults here: env vars override the mounted ConfigMap file
# (precedence: code defaults → CONFIG_PATH file → env). Defaults live in
# internal/platform/config and each domain's config package
# (docs/specs/101-backend-configuration.md).
#
# Writable paths are mounted by the deployment: /tmp (git cache) and the
# uploads directory (UPLOADS_DIR).

USER 1000:1000

EXPOSE 8080

HEALTHCHECK NONE

ENTRYPOINT ["/app/pupitre-api"]
