# Architecture — E-Learning Platform

## Decision: a single application

The backend is a single Go application, `pupitre-api` (`apps/api`), on port 8080. It brings three domains together in one process: `users`, `courses` and `checker`. They call each other directly, without an internal API. The frontend is made of two React applications, served on the same origin.

## Source of truth for courses: PostgreSQL

The only source of truth is the **database**. Courses are created and edited through the admin API; there is no YAML file on disk, no Kubernetes resource and no in-memory cache.

Each request reads what it needs:

- the catalogue (`GET /api/courses`) filters in SQL and loads no module row;
- displaying a course loads its modules with one query indexed on `(course_slug, position)`.

No replica keeps content in memory: a replica that has just started serves exactly the same catalogue as one that has been running for a week, and a redeployment loses nothing.

```jsonc
// POST /api/admin/courses
{
  "slug": "kubernetes-basics",
  "spec": {
    "title": "Kubernetes Basics",
    "description": "Learn the basics of Kubernetes",
    "public": true,
    "category": "kubernetes",
    "difficulty": "beginner",
    "modules": [
      { "name": "Introduction", "type": "text",
        "src": "https://github.com/org/courses", "ref": "main", "path": "intro.md" },
      { "name": "Video demo", "type": "video", "src": "/uploads/demo.mp4" }
    ]
  }
}
```

### Schema

| Table | Role |
|---|---|
| `courses` | Course metadata; columns indexed for catalogue filtering (`category`, `difficulty`, `public`, `title`) and denormalized `skills` |
| `course_modules` | One module = one row, ordered by `position`; opaque payloads (`questions`, `steps`, `check_params`) as `jsonb` |
| `course_prerequisites` | Enrollment conditions |
| `course_sessions` | In-person sessions, key `(course_slug, session_id)` — a replayed write overwrites the same row |
| `paths`, `path_courses`, `path_skills` | Paths and their ordered members |
| `quiz_question_attempts` | Attempt counter and end of cooldown per question |
| `lab_checks` | History of lab checks |

## Architecture

```
                     Browser
                        │
                        ▼
              Ingress or HTTPRoute (Helm chart)
                 │                  │
    /api, /uploads, /metrics,     the rest
    /health      │                  │
                 ▼                  ▼
  ┌─────────────────────────┐  ┌───────────────────────┐
  │       pupitre-api       │  │   Frontend (nginx)    │
  │                         │  │                       │
  │  users                  │  │  apps/app: learners   │
  │  - OIDC auth, JWT       │  │  apps/admin:          │
  │  - profile, enrollments │  │    /admin and /manager│
  │  - progress, settings   │  └───────────────────────┘
  │                         │
  │  courses                │
  │  - catalogue (SQL)      │
  │  - modules, git         │
  │  - media upload         │
  │  - lab_checks           │
  │                         │
  │  checker (OPA/Rego)     │
  │  - GitLab state via API │
  │  - evaluates the policy │
  └────────────┬────────────┘
               ▼
          PostgreSQL
```

In development, the Vite server plays the role of the ingress: it sends `/api` and `/uploads` to the API.

## Paths

A path is an ordered sequence of courses (`kind: course`) or skills (`kind: skill`), stored in `paths` + `path_courses` / `path_skills`. See `docs/Skills.md` for the details.

```jsonc
// POST /api/admin/courses/paths
{
  "slug": "devops-path",
  "spec": {
    "title": "DevOps path",
    "kind": "skill",
    "level": "intermediate",
    "skills": ["linux", "docker", "kubernetes", "ci-cd"]
  }
}
```

## Path and skill endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/paths` | Lists every path |
| `GET` | `/api/paths/{slug}` | Details of a path |
| `GET` | `/api/skills/{slug}/modules` | Modules teaching a skill |
| `GET` | `/api/my/paths` | The user's paths, with a status per course or skill |
| `GET` | `/api/my/skills/{slug}` | Modules of a skill, with the user's status |
| `POST` | `/api/admin/paths/{slug}/enrollments` | Enrolls a user in a path (admin) |
| `DELETE` | `/api/admin/paths/{slug}/enrollments/{userId}` | Unenrolls a user (admin) |

## Module content

The API reads courses and modules from PostgreSQL on demand, then resolves their content:

- `type: video/image` → serves the URL
- `type: text` with a git `src` → clones the repository and reads the file
- `type: quiz` → inline questions or YAML from git, with a score
- `type: modules` → fetches a YAML index file from git and expands its entries in place (`src`/`ref` inherited from the parent module)

## Authentication

The interface signs in through OIDC with Keycloak only: see `docs/SSO.md`. The API issues and validates its own JWT tokens, signed with `JWT_SECRET` (32 characters minimum).

## Configuration

The API loads its configuration from an optional YAML file, set by `CONFIG_PATH`. Environment variables **override** the values of the file.

The Helm chart mounts this file from a ConfigMap at `/etc/pupitre/config.yaml`. Sensitive values (JWT, database) stay in Kubernetes Secrets. Locally, `pnpm api:dev` reads `apps/api/.env`.

**Behind an ingress, set `TRUSTED_PROXIES`** to the addresses of the ingress, for example through `api.extraEnv`. By default only loopback is trusted: all clients then share the address of the ingress and a single rate limit, and sign-ins end in `429` from the 11th attempt within a minute.

The frontend is static: the `docker/front` image serves both applications with nginx. Put no secret in `VITE_*` variables: they ship to the browser.

### Customization

```bash
# Override a value through Helm
helm upgrade pupitre oci://ghcr.io/genesary/charts/pupitre --reuse-values --set api.env.GIT_CACHE_TTL=30

# Or edit the values.yaml, then run helm upgrade
```

## Git secret — course-repo-secret

For course modules that reference a **private git repository**, the API needs an access token. This token comes from a Kubernetes Secret mounted in the pod:

```
course-repo-secret (type: Opaque)
  └── git-credentials.yaml
        └── credentials:
              - url: "github.com/org/*"        # glob pattern
                token: "github_pat_xxx..."
```

The file is YAML, with a list of **url → token** entries. The first matching entry (through `path.Match`) is used.

**Creation:**

```bash
kubectl create secret generic course-repo-secret \
  --from-file=git-credentials.yaml=./git-credentials.yaml
```

**Mount path in the pod:** `/etc/git-credentials/git-credentials.yaml`

The chart gives this location to the API (`gitCredentialsPath`, or the `GIT_CREDENTIALS_PATH` variable). If the secret does not exist, the pod starts, but cloning private repositories fails.

See `apps/api/infra/examples/course-service/course-secret.yaml` for a complete example.
