# The frontend image: the learner app and the admin app, built once and
# served as static files on one origin (docs/specs/000-scope-and-stack.md,
# docs/specs/108-backend-operations-and-testing.md).
#
# The build context is the repository root:
#   docker build -f docker/front/Dockerfile -t pupitre-front .

# Stage 1: Build
FROM docker.io/library/node:24.21.0-alpine AS builder

WORKDIR /src

RUN corepack enable

# Install from the manifests only, so a source change does not reinstall.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/app/package.json apps/app/
COPY apps/admin/package.json apps/admin/
COPY packages/core/package.json packages/core/
COPY packages/ui/package.json packages/ui/
# KICS reads the filters below as unpinned packages: --frozen-lockfile
# installs the exact versions of pnpm-lock.yaml.
# kics-scan ignore-line
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile --filter @pupitre/app... --filter @pupitre/admin...

COPY tsconfig.base.json ./
COPY apps/app apps/app
COPY apps/admin apps/admin
COPY packages packages
RUN pnpm build

# Stage 2: Runtime
FROM docker.io/nginxinc/nginx-unprivileged:1.30.6-alpine

# Take the Alpine fixes published since the base image was built, so that
# the image scan (Trivy) does not depend on when nginx last rebuilt it.
USER root
RUN apk upgrade --no-cache

COPY docker/front/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/front/security-headers.inc /etc/nginx/security-headers.inc
COPY --from=builder /src/apps/app/dist /usr/share/nginx/html
COPY --from=builder /src/apps/admin/dist /usr/share/nginx/html/admin

# nginx-unprivileged runs as user 101; the root filesystem can be read-only
# when /tmp is writable (the deployment mounts it).
USER 101

EXPOSE 3000

HEALTHCHECK NONE
