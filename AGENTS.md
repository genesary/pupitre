# pupitre2

React frontend of the pupitre e-learning platform. It replaces the Astro frontend of
[`genesary/pupitre`](https://github.com/genesary/pupitre) and talks to its Go backend
(user-service, course-service) **as it is**. Specs: [`docs/specs`](docs/specs/README.md), in
English; they are the source of truth for behaviour.

## Git: commit and push only when asked

- Work on a Conventional Branch feature branch (`feature/<name>`), never on `main`.
- Commit **only when the user asks**, on the feature branch, with an English Conventional Commits
  message. Push that branch only when the user asks.
- Never commit or push to `main`, never force-push, never rewrite published history. Changes reach
  `main` through a pull request, squash merged by the user.
- Before committing, `typecheck`, `lint`, `test` and `build` pass.
- Git hooks (prek, `.pre-commit-config.yaml`, installed by `mise run setup:prek`) refuse commits
  on `main`, run lint and build on commit, check the Conventional Commits message, and refuse to
  push unsigned commits.
  Never bypass them (`--no-verify`): fix the cause instead.

## Constraints

- **The Go backend's API is not changed.** Every request matches a route and a struct of
  `genesary/pupitre` at commit `3cdf7b2`, the reference of every spec. A
  backend defect is recorded in [`190`](docs/specs/190-backend-defects.md), never worked around
  by inventing a route.
- **The backend moves into `apps/api` as one application** (specs
  [`100`-`199`](docs/specs/100-backend-scope-and-architecture.md)), batch by batch
  ([`199`](docs/specs/199-backend-migration-plan.md)). The move keeps every route, table and rule;
  a defect is fixed only after it, as a separate change.
- **The pupitre design is kept as is**: colours, fonts, spacing, markup and class names of the
  Astro pages ([`005`](docs/specs/005-ui-theme-navigation.md), "Visual design").
  `packages/ui/src/styles/pupitre/*.css` are copies of the original CSS: do not edit them. shadcn
  components are themed with the pupitre tokens, never the other way round.
- **Code reused from [`ebuildy/pupitre2`](https://github.com/ebuildy/pupitre2)** follows the rules
  of [`090`](docs/specs/090-gaps-and-defects.md) ("Reusing the reference frontend"): adapt it to
  the Go API, restyle it with pupitre's classes, keep its file name, credit it in a comment.
- Interface text is in French. Code, comments, specs, branches and commits are in English.
- Authentication is OIDC through Keycloak only ([`001`](docs/specs/001-keycloak-authentication.md)).

## Layout

pnpm monorepo, same layout as `ebuildy/pupitre2`: two applications on one origin.

| Path | Package | Content |
| --- | --- | --- |
| `apps/app` | `@pupitre/app` | The learner app: sign-in, catalogue, lessons, quizzes, learner space, agenda |
| `apps/admin` | `@pupitre/admin` | The admin app, served under `/admin` and `/manager`: administration, manager space, CSV exports, attendance |
| `packages/core` | `@pupitre/core` | Shared by both apps: API client and types, session, top bar, guards, markdown, mock mode |
| `apps/api` | Go module | The Go backend, imported from `genesary/pupitre` at `3cdf7b2`, as one application: `cmd/pupitre-api` on port 8080 ([`100`](docs/specs/100-backend-scope-and-architecture.md)) |
| `packages/ui` | `@pupitre/ui` | Design system: shadcn components, pupitre CSS, security helpers |
| `docs/specs` | | Specs, one file per feature area |
| `e2e` | | Playwright tests |
| `docker/front` | | The frontend image: both apps behind nginx ([`108`](docs/specs/108-backend-operations-and-testing.md)) |

Inside `apps/app/src` and `apps/admin/src`:

- `routes/` - one **PascalCase** file per page (`Catalogue.tsx`, `CourseDetail.tsx`), its
  component named after the file; in the admin app, `admin/` and `manager/` hold each side's pages.
- `components/` - app components, PascalCase, grouped by domain (`quiz/`, `lab/`, `admin/`).
- `lib/` - kebab-case modules used by that app only.

Inside `packages/core/src`:

- `lib/` - `api.ts` (one function per route), `api-client.ts`, `api-types.ts`, `session.ts`,
  `markdown.ts`, `site.ts` (which app serves a path), shared helpers.
- `components/` - what both apps show: top bar, side menu, guards, dialogs, page states.
- `mocks/` - MSW handlers and fixtures, used by both apps.

`packages/ui` and `packages/core` never import from `apps/*`, and `packages/ui` never imports from
`packages/core`. The apps import them through their exports: `@pupitre/ui/components/<name>`,
`@pupitre/ui/lib/<name>`, `@pupitre/ui/globals.css`, `@pupitre/core/lib/<name>`,
`@pupitre/core/components/<Name>`, `@pupitre/core/mocks/<name>`. Inside the packages, imports are
relative; after `shadcn add`, rewrite the generated `@/` imports.

Both apps share the origin, so the session (sessionStorage) is shared. A link to a page of the
other app is a page load: use `SiteLink`, `SiteNavLink`, `SiteNavigate` or `useSiteNavigate` from
`@pupitre/core`, never a router `Link`.

## Commands

Node 24 (`.nvmrc`), pnpm through corepack (`corepack enable`). From the repository root:

```sh
pnpm install
pnpm dev:mock    # http://localhost:3000, no backend (docs/specs/006-mock-mode.md)
pnpm api:up      # PostgreSQL and Keycloak (apps/api/compose.yaml)
pnpm api:dev     # pupitre-api on http://localhost:8080 (settings: apps/api/.env)
pnpm dev         # http://localhost:3000, /api and /uploads proxied to the API (VITE_API_URL)
                 # both start the two apps: the admin app (port 3001) is reached through
                 # http://localhost:3000/admin and /manager, never on its own port
pnpm typecheck
pnpm lint
pnpm test
pnpm build
pnpm test:e2e    # Playwright, against the mock mode (first time: pnpm exec playwright install chromium)
pnpm test:e2e:backend  # Playwright against the real backend, after pnpm api:up
```

Run `typecheck`, `lint`, `test` and `build` before handing work back. In Claude Code, `/review`
(`.claude/skills/review`) reviews the branch with the project's reviewers, as the `AI review`
workflow does on pull requests. A UI change is also checked
in the browser with `pnpm dev:mock`, in light and dark mode.

In `apps/api` (Go 1.26): `pnpm api:build`, `pnpm api:lint` and `pnpm api:test` (or the
`Makefile`) pass before handing work back. Integration tests need PostgreSQL (`pnpm api:up`):
`make test-integration`. A change that crosses the frontend and the API is also checked with
`pnpm test:e2e:backend`.

Dependencies are pinned to exact versions. pnpm's `minimumReleaseAge` policy rejects versions that
are too recent: pick an older version, never add an exclusion.

## Security rules

[`003`](docs/specs/003-security.md) is enforced by lint and tests:

- No `dangerouslySetInnerHTML`, `innerHTML` or `outerHTML` outside
  `packages/ui/src/components/safe-html.tsx`, `packages/core/src/lib/markdown.ts` and shadcn's
  `components/ui/`. API data is rendered as JSX text; markdown goes through `renderMarkdown`
  (DOMPurify) and `<SafeHtml>`.
- URLs from the API go through `safeUrl` / `safeExternalUrl`, redirects through `safeRedirect`
  (`@pupitre/ui/lib/security`).
- The token lives in `sessionStorage`, behind `packages/core/src/lib/session.ts` only. `localStorage`
  is reached through `@pupitre/ui/lib/storage` (lint rule).
- No secret in `VITE_*` variables: they ship to the browser.
- The dev proxy forwards only the allowed headers (`apps/app/vite.config.ts`).

## Mock mode

`pnpm dev:mock` answers `/api/*` with MSW, mirroring the Go handlers. A page that needs a new route
gets its handler and fixtures in `packages/core/src/mocks/` in the same change. The mock carries no
business rule beyond what the Go code does. Test accounts: `alice` (student), `marc.manager`,
`admin`, and an XSS account whose payloads must never run.

## Git conventions

- Branches: [Conventional Branch](https://conventional-branch.github.io/) (`feature/…`,
  `bugfix/…`, `chore/…`), lowercase, English.
- Commits: [Conventional Commits](https://www.conventionalcommits.org/), English.
- Pull requests into `main`, squash merged.
