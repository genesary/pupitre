# 000 - Scope, stack and repository conventions

## Scope

pupitre2 holds the platform **frontend**. It talks to the existing Go API, which is not changed.
The repository is a frontend monolith: a single React application that holds the learner space,
the manager space and the administration. There is no separate admin application.

## Stack

| Role | Choice | Note |
| --- | --- | --- |
| Development runtime | Node.js **24 LTS** (`.nvmrc`) | Node 18 is end-of-life and not supported by Vite or the current CLIs |
| Package manager | **pnpm** workspaces (through `corepack enable`) | `packageManager` pinned in `package.json` |
| Language | **TypeScript**, `strict` mode | All components are `.tsx` files |
| UI library | **React 19** | |
| Build tool and dev server | **Vite** | |
| Routing | **React Router** (data mode: `createBrowserRouter`, loaders) | Single-page application, rendered in the browser |
| Components | **shadcn/ui** (Radix + Tailwind) | Components are copied into `packages/ui/src/components/ui` |
| Styles | **Tailwind CSS v4** | The tokens of [005](005-ui-theme-navigation.md) become the shadcn theme variables |
| Server state | **TanStack Query** | Cache, invalidation after mutations, loading states |
| Forms | react-hook-form + zod | Used by the shadcn form components |
| Icons | lucide-react | Replaces the hand-copied SVGs |
| Toasts | sonner (shadcn component) | |
| Markdown | marked + **DOMPurify** | See [012](012-lesson-player.md) and [003](003-security.md) |
| Diagrams | mermaid, loaded on demand | |
| Unit tests | Vitest + Testing Library | |
| End-to-end tests | Playwright | |

The frontend is **two applications** served as static files on **one origin**: the learner app,
and the admin app (administration and manager space) under `/admin` and `/manager`. In production,
a reverse proxy sends `/api/*` to the Go services following the table in
[004](004-api-and-routing.md), `/admin/*` and `/manager/*` to the admin app's `index.html`, and
every other route to the learner app's `index.html`. In development, the learner app's Vite proxy
does the same. Sharing the origin shares the session (sessionStorage); going from one app to the
other is a page load. A learner never downloads the admin app's code.

## Repository layout

A pnpm monorepo with the layout of the reference rewrite
([`ebuildy/pupitre2`](https://github.com/ebuildy/pupitre2)), so its code can be reused file for
file ([090](090-gaps-and-defects.md)). `apps/api` is reserved for a later backend rewrite; the Go
services stay as they are.

```
pupitre2/
├── AGENTS.md                    # rules for coding agents (CLAUDE.md imports it)
├── docs/specs/                  # this folder
├── apps/
│   ├── app/                     # @pupitre/app: the learner app
│   │   ├── index.html
│   │   ├── vite.config.ts       # dev proxy: /api (004), /admin and /manager to the admin app
│   │   ├── public/              # pioupitre.png, theme-init.js, mockServiceWorker.js
│   │   └── src/
│   │       ├── main.tsx         # entry point, providers (Query, Router, Toaster, Tooltip)
│   │       ├── router.tsx       # route table and guards
│   │       ├── routes/          # one PascalCase file per page
│   │       ├── components/      # <domain>/ folders (quiz/, lab/, sessions/…)
│   │       └── lib/             # learner-only helpers
│   └── admin/                   # @pupitre/admin: the admin app, base /admin/
│       ├── vite.config.ts
│       └── src/
│           ├── main.tsx, router.tsx
│           ├── routes/          # admin/, manager/, AdminLayout, Attendance
│           ├── components/admin/
│           └── lib/             # admin-only helpers (course definition, settings, slug…)
├── packages/
│   ├── core/                    # @pupitre/core: shared by both apps
│   │   └── src/
│   │       ├── lib/             # API client and types, session, markdown, site (which app serves a path)
│   │       ├── components/      # top bar, side menu, guards, dialogs, page states
│   │       └── mocks/           # MSW handlers and fixtures (006)
│   └── ui/                      # @pupitre/ui: design system, no app logic
│       └── src/
│           ├── components/ui/   # shadcn components
│           ├── components/      # brand, icons, SafeHtml, UserAvatar
│           ├── lib/             # security (sanitization, URL checks), storage, theme
│           └── styles/          # globals.css + pupitre/*.css (the original design, 005)
├── e2e/                         # Playwright tests
├── package.json                 # workspace scripts
├── pnpm-workspace.yaml
└── tsconfig.base.json
```

`packages/ui` and `packages/core` never import from `apps/*`. The apps import them as
`@pupitre/ui/components/<name>`, `@pupitre/ui/lib/<name>`, `@pupitre/ui/globals.css`,
`@pupitre/core/lib/<name>`, `@pupitre/core/components/<Name>` and `@pupitre/core/mocks/<name>`.

## Configuration

| Variable | Use | Development default |
| --- | --- | --- |
| `VITE_API_URL` | Dev proxy target for `/api` and `/uploads` (the backend, [108](108-backend-operations-and-testing.md)) | `http://localhost:8080` |

No secret may reach the browser: every `VITE_*` variable is public.

## Scripts

| Command | Effect |
| --- | --- |
| `pnpm dev` | Both apps' dev servers; open http://localhost:3000 (the admin app through `/admin`) |
| `pnpm dev:mock` | The same, API answered by MSW ([006](006-mock-mode.md)) |
| `pnpm build` | Type check, then production build of both apps (`apps/*/dist`) |
| `pnpm lint` | ESLint, whole repository |
| `pnpm typecheck` | TypeScript, every package |
| `pnpm test` | Vitest, every package |
| `pnpm test:e2e` | Playwright |

## Git conventions

- Branches follow [Conventional Branch](https://conventional-branch.github.io/): `feature/…`,
  `bugfix/…`, `hotfix/…`, `release/…`, `chore/…`, lowercase with hyphens.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`,
  `docs:`, `chore:`, `refactor:`, `test:`, `ci:`, `style:`, `perf:`, with an optional scope
  (`feat(catalogue): …`).
- No direct commit on `main`: everything goes through a branch and a pull request.

## Acceptance criteria

- `pnpm install && pnpm build` passes on Node 24 without a version warning.
- `pnpm dev` shows the catalogue when the Go services run locally.
- No dependency is loaded from a CDN at runtime, fonts included (they are served by the
  application, see [003](003-security.md)).
