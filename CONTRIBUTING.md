# Contributing — Dev Environment Setup

## Prerequisites

You need a container engine, [Docker](https://docs.docker.com/engine/install/) or Podman. The Makefile
defaults to Podman; to use Docker, set `DOCKER=docker` in a git-ignored `local.mk`.

Every other tool is pinned in [`mise.toml`](mise.toml) and installed by mise.

### Install the toolchain with mise

[mise](https://mise.jdx.dev/) installs the exact tool versions the project uses: Go, Node.js, `kind`,
`kubectl`, `helm`, `golangci-lint`, `hadolint`, `prek` and a few more.

1. [Install mise](https://mise.jdx.dev/getting-started.html).
2. Activate it in your shell, for example for zsh (replace `zsh` with `bash` or `fish` as needed):

   ```bash
   echo 'eval "$(mise activate zsh)"' >> ~/.zshrc
   ```

3. From the repository root, install the tools:

   ```bash
   mise trust
   mise install
   ```

Without mise, install the tools listed in `mise.toml` manually, at the versions it pins.

### Check your setup

```bash
mise ls --current                 # every tool should be installed
docker --version && kind version && helm version --short && kubectl version --client && go version && node --version
```

## Pre-commit hooks

[prek](https://github.com/j178/prek), a fast drop-in replacement for `pre-commit`, runs the checks
declared in [`.pre-commit-config.yaml`](.pre-commit-config.yaml) before each commit:

- file hygiene (whitespace, end-of-file newline, line endings, valid YAML/JSON/TOML, merge-conflict
  markers, large files)
- `hadolint` on Dockerfiles
- `markdownlint-cli2` on Markdown, configured in [`.markdownlint-cli2.yaml`](.markdownlint-cli2.yaml)
- `golangci-lint` on Go code, reporting only issues introduced since `HEAD`

`prek` is installed by `mise install`. Enable the git hook once per clone:

```bash
make precommit-setup    # prek install
```

Run all hooks on the whole repository at any time, for example before opening a pull request:

```bash
make precommit-run      # prek run --all-files
```

If a hook fails, fix the reported issue (some hooks fix files automatically), stage the changes and
commit again.

---

## Quick Start (first time)

### 1. Build all Docker images

```bash
make docker-build
```

This builds 3 images:

- `localhost/pupitre-course-service:latest`
- `localhost/pupitre-user-service:latest`
- `localhost/pupitre-frontend:latest`

If a build fails, check the Dockerfile in each service directory.

### 2. Create the KinD cluster

```bash
make kind-create
```

This creates a local Kubernetes cluster named `pupitre` inside Docker.

### 3. Load images into the cluster

```bash
make kind-load
```

### 4. Deploy everything with Helm

```bash
make helm-install
```

This installs all services and PostgreSQL.

### 5. Wait for pods to be ready

```bash
kubectl wait --for=condition=ready pod --all --timeout=180s
```

Check the status:

```bash
kubectl get pods
```

You should see 4 pods running:

- `pupitre-course-service-...`
- `pupitre-user-service-...`
- `pupitre-postgresql-...`
- `pupitre-frontend-...`

> **Troubleshooting:** If `pupitre-postgresql` stays `Pending`, run:
>
> ```bash
> kubectl describe pod pupitre-postgresql-0
> ```
>
> If you see `Insufficient ephemeral-storage`, disable Bitnami PostgreSQL and use the standalone one:
>
> ```bash
> # Edit kind-values.yaml and set: postgresql.enabled: false
> # Then:
> make helm-install
> kubectl apply -f infra/manifests/postgresql.yaml
> ```

### 6. Courses

The KinD values files set `SEED_DEV_COURSES=true`, so course-service seeds a
demo catalogue of 17 courses on startup and the app is browsable immediately —
nothing to apply by hand.

The seed definitions live in `course-service/internal/db/seed/courses/` and are
embedded in the binary. After editing one, rebuild the image and re-seed:

```bash
make rebuild-course
make seed-courses      # re-runs the seed in overwrite mode
```

`SEED_DEV_COURSES` takes two values:

| Value       | Behaviour                                                                                                |
| ----------- | -------------------------------------------------------------------------------------------------------- |
| `true`      | Creates only the courses that do not exist yet — local edits made through the admin UI survive a restart |
| `overwrite` | Replaces every seed course, discarding local edits to them                                               |

Anything else (including unset) disables seeding entirely, so a production
deployment cannot pick up demo content by accident.

To create your own course, use the admin UI at `/admin/courses` or POST a
definition:

```bash
curl -X POST http://localhost:8082/api/admin/courses \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
        "slug": "my-course",
        "spec": {
          "title": "My Course",
          "public": true,
          "category": "linux",
          "difficulty": "beginner",
          "modules": [
            { "name": "Introduction", "type": "text", "content": "# Hello" }
          ]
        }
      }'
```

### 7. Set up git credentials (for private course repos)

If the course modules reference a private git repo, create a secret:

```bash
kubectl create secret generic course-repo-secret \
  --from-file=git-credentials.yaml=./git-credentials.yaml
```

See `infra/examples/course-service/course-secret.yaml` for the format.

### 8. Expose services locally

```bash
make port-forward
```

### 9. Open the app

Open <http://localhost:3000> in your browser.

Default admin login: `admin@pupitre.local`. The password is generated at
install time — retrieve it with:

```bash
kubectl get secret pupitre-secrets -o jsonpath='{.data.ADMIN_PASSWORD}' | base64 -d
```

---

## One-command full reset

```bash
make dev
```

This deletes the old cluster, creates a new one, builds images, loads them, runs Helm install, and seeds the demo courses.

---

## Access

| Service    | URL                      | Auth |
| ---------- | ------------------------ | ---- |
| Frontend   | <http://localhost:3000>  | —    |
| Course API | <http://localhost:18082> | —    |
| User API   | <http://localhost:18081> | —    |

---

## Makefile commands

| Command                  | What it does                       |
| ------------------------ | ---------------------------------- |
| `make kind-create`       | Create KinD cluster (`pupitre`)    |
| `make kind-delete`       | Delete KinD cluster                |
| `make docker-build`      | Build all 3 Docker images          |
| `make kind-load`         | Load images into KinD nodes        |
| `make helm-install`      | Deploy/upgrade Helm chart          |
| `make helm-delete`       | Uninstall Helm release             |
| `make port-forward`      | Expose services on localhost       |
| `make port-forward-stop` | Stop port-forwards                 |
| `make logs`              | Tail service logs                  |
| `make status`            | Show pods and services             |
| `make clean`             | Delete Helm release + KinD cluster |

---

## Rebuilding a single service

```bash
# Build and reload course-service
docker build -f course-service/Dockerfile -t localhost/pupitre-course-service:latest .
kind load docker-image localhost/pupitre-course-service:latest --name pupitre
kubectl rollout restart deploy/pupitre-course-service
```

Same pattern for `user-service` and `frontend`.

---

## Adding a new course

Courses are stored in PostgreSQL. Create one from the admin UI, or POST the
definition to the course-service:

```bash
curl -X POST http://localhost:8082/api/admin/courses \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
        "slug": "my-new-course",
        "spec": {
          "title": "My New Course",
          "description": "Description here",
          "hidden": false,
          "category": "linux",
          "difficulty": "beginner",
          "modules": [
            { "name": "Introduction", "type": "text",
              "src": "https://github.com/user/repo", "ref": "main",
              "path": "lessons/intro.md" }
          ]
        }
      }'
```

`GET /api/admin/courses/{slug}/definition` returns the stored definition in the
same shape, so it can be fetched, edited, and sent back with `PUT`.

---

## Troubleshooting

**Port-forwards fail ("address already in use")**:

```bash
make port-forward-stop
make port-forward
```

**Enrollment fails with "Session expired"**: the admin JWT token references a stale user UUID. Log out and log back in.

**Course not found after deployment**: courses are read from the database on
each request, so a missing course means it was never created. Verify:

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8082/api/admin/courses
```

**Blank page / JS errors**: hard refresh the browser (Ctrl+Shift+R) to clear the SvelteKit cache.

**PostgreSQL not starting**: check pod events:

```bash
kubectl describe pod -l app=postgresql
```

**User service CrashLoopBackOff**: check logs:

```bash
kubectl logs -l app.kubernetes.io/name=pupitre --tail=20
```

If it says "connection refused" to PostgreSQL, the database isn't ready yet.

**Git content not loading (500 error)**: the git secret `course-repo-secret` may be missing or the token is invalid. Recreate it:

```bash
kubectl delete secret course-repo-secret
kubectl create secret generic course-repo-secret --from-file=git-credentials.yaml=./git-credentials.yaml
```

**ConfigMaps**: after modifying a ConfigMap, restart the pod to pick up the changes:

```bash
kubectl rollout restart deploy/pupitre-course-service
kubectl rollout restart deploy/pupitre-user-service
kubectl rollout restart deploy/pupitre-frontend
```

The Go services read the mounted YAML at startup. The frontend entrypoint sources the `.env` file on each container start. All three also respect env var overrides — see `docs/ARCHITECTURE.md` (section "Configuration des services").
