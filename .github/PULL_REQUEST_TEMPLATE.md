### Description of your changes

<!--
Adapted from genesary/pupitre at 3cdf7b2 (.github/PULL_REQUEST_TEMPLATE.md),
with the commands and rules of pupitre2 (AGENTS.md).

Briefly describe what this pull request does. Be sure to direct your reviewers'
attention to anything that needs special consideration.

If it resolves an open issue, keep the line below with its number, for example
"Fixes #500".
-->
Fixes #

I have:

- [ ] Read and followed `AGENTS.md` and the specs in `docs/specs`.
- [ ] Followed the Conventional Commits message format and a Conventional Branch name.
- [ ] Updated the specs when the behaviour or the design changes (`docs/specs/005-ui-theme-navigation.md`, `parity.md`, `docs/specs/090-gaps-and-defects.md`, `docs/specs/190-backend-defects.md`).
- [ ] Run `pnpm typecheck`, `pnpm lint`, `pnpm test` and `pnpm build`.
- [ ] Run `pnpm api:lint`, `pnpm api:test` and `pnpm api:build` when `apps/api` changes.

### How has this code been tested

I have:

- [ ] Added or updated unit tests and Playwright tests (`pnpm test:e2e`).
- [ ] Checked a UI change in the browser with `pnpm dev:mock`, in light and dark mode.
- [ ] Run `pnpm test:e2e:backend` when the change crosses the frontend and the API.
