# pupitre2

The pupitre e-learning platform: a React frontend (two apps) and its Go backend, `pupitre-api`.
They replace the Astro frontend and the three Go services of
[`genesary/pupitre`](https://github.com/genesary/pupitre), with the same API.

- Stack: React 19, TypeScript, Vite, React Router, Tailwind CSS v4, shadcn/ui, TanStack Query,
  pnpm.
- Authentication: OIDC through Keycloak only.
- Specs: [`docs/specs`](docs/specs/README.md).
- Calling the admin API to add courses (PowerShell and Linux): [`docs/guides/admin-api-courses.md`](docs/guides/admin-api-courses.md).
- Agent rules: [`AGENTS.md`](AGENTS.md).

## Requirements

- Node.js 24 LTS (`.nvmrc`).
- pnpm, enabled with `corepack enable`.
- For `pnpm dev` only: the backend. Go 1.26 and Docker (`pnpm api:up` starts PostgreSQL and
  Keycloak, `pnpm api:dev` the API on port 8080). `pnpm dev:mock` needs no backend.

## Getting started

### With mise

[mise](https://mise.jdx.dev/) installs Node.js, pnpm and Go from `mise.toml` and runs the tasks. Each
task installs the dependencies first when needed.

```sh
mise install          # Node.js 24, pnpm, prek, Go and golangci-lint, versions from mise.toml
mise run setup:prek        # Git hooks, see "Contributing"
mise run dev:mock     # http://localhost:3000, no backend needed (docs/specs/006-mock-mode.md)
mise run dev          # http://localhost:3000, against the Go services
# Both start the two apps; the admin app is opened through http://localhost:3000/admin
mise run check        # typecheck, lint, test and build
mise run api:dev      # pupitre-api on http://localhost:8080 (after mise run api:up)
mise tasks            # every task: build, lint, typecheck, test, test:e2e, api:test…
```

### In a dev container

`.devcontainer/` builds an image with mise, Node.js, pnpm, Go, golangci-lint and Playwright's Chromium already
installed. Open the folder in VS Code, run "Dev Containers: Reopen in Container", then
`mise run dev:mock`. `node_modules` lives in a Docker volume, apart from the host's.

With Podman, set `"dev.containers.dockerPath": "podman"` in the VS Code user settings: through the
Docker CLI, VS Code can stay stuck after "Container started".

### With pnpm only

```sh
corepack enable
pnpm install
pnpm dev:mock   # http://localhost:3000, no backend needed (docs/specs/006-mock-mode.md)

pnpm api:up     # PostgreSQL and Keycloak (apps/api/compose.yaml)
pnpm api:dev    # pupitre-api on http://localhost:8080 (settings: apps/api/.env)
pnpm dev        # http://localhost:3000, against the API
# Both start the two apps; the admin app is opened through http://localhost:3000/admin
```

| Command | Effect |
| --- | --- |
| `pnpm build` | Type check, then production build in `apps/app/dist/` and `apps/admin/dist/` |
| `pnpm test:e2e` | End-to-end tests (Playwright) against the mock mode |
| `pnpm test:e2e:backend` | End-to-end tests against the real backend (after `pnpm api:up`) |
| `pnpm api:test`, `pnpm api:lint`, `pnpm api:build` | Go tests, lint and build of `apps/api` |
| `pnpm test` | Unit and component tests (Vitest), every package |
| `pnpm lint` | ESLint, including the security rules of `docs/specs/003-security.md` |
| `pnpm typecheck` | TypeScript only, every package |

The dev proxy target can be changed with `VITE_API_URL`. Keycloak test accounts (local only):
`alice`, `marc.manager` and `admin.kc`, see `apps/api/compose.yaml`.

## Layout

| Path | Package | Content |
| --- | --- | --- |
| `apps/app` | `@pupitre/app` | The learner app: sign-in, catalogue, lessons, learner space, agenda |
| `apps/admin` | `@pupitre/admin` | The admin app, under `/admin` and `/manager`: administration and manager space |
| `packages/core` | `@pupitre/core` | Shared by both apps: API client, session, top bar, guards, markdown, mocks |
| `packages/ui` | `@pupitre/ui` | Design system: shadcn components, pupitre CSS, security helpers |
| `apps/api` | Go module | The backend, `pupitre-api`: one application on port 8080 |
| `docs/specs` | | Functional and technical specs |

Details: [`docs/specs/000-scope-and-stack.md`](docs/specs/000-scope-and-stack.md).

## Contributing

- Branches: [Conventional Branch](https://conventional-branch.github.io/) (`feature/…`,
  `bugfix/…`, `chore/…`).
- Commits: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
  `docs:`…).
- No direct commit on `main`.
- Commits are signed (`git config commit.gpgsign true`).

`mise run setup:prek` installs the Git hooks of `.pre-commit-config.yaml`, run by
[prek](https://github.com/j178/prek):

| Hook | Checks |
| --- | --- |
| `pre-commit` | No commit on `main`; `mise run lint` and `mise run build` when code changes |
| `commit-msg` | [Conventional Commits](https://www.conventionalcommits.org/) subject (`feat(scope): …`) |
| `pre-push` | Every pushed commit has a valid signature |

`mise x -- prek run --all-files` runs the pre-commit checks by hand (prek 0.5.6 or later, from mise.toml).
