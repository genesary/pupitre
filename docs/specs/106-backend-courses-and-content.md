# 106 - Backend: courses, content and progress

The business rules of `genesary/pupitre` that the move must keep. They come from `docs/Course.md`,
`docs/Module.md`, `docs/Skills.md`, `docs/ARCHITECTURE.md` and `AGENTS.md` of that repository,
rewritten here in English; the code at `3cdf7b2` stays the reference where this text is shorter.
The pages that show them are in specs 010 to 047.

## Courses

- **Source of truth: PostgreSQL.** No in-memory store, no filesystem loading, no Kubernetes
  controller. Every request reads what it needs from the database.
- Managed through the admin API: `POST /api/admin/courses` (`{slug, spec}`),
  `GET|PUT|DELETE /api/admin/courses/{slug}/definition`.
- `spec` fields: `title`, `description`, `public`, `hidden`, `category`, `difficulty`
  (`beginner`, `intermediate`, `advanced`), `modules[]`, `prerequisites`, `badge`, `skills`
  (computed).
- `public: true`: listed in the catalogue, and a learner is **auto-enrolled** on the first visit of
  a module (detached call, [104](104-backend-in-process-calls.md)). `public: false`: reachable only
  by enrolled learners (directly, through a group, or through a path).
- Course prerequisites must be completed before the course opens.

## Modules

| Type | Body |
| --- | --- |
| `text` | `content` (markdown stored on the module) **or** `src`/`ref`/`path` (a file in a git repository); git wins when both are set, so writers clear one when they set the other |
| `video`, `image` | URL served by the platform (`/uploads/…`); `replication: true` downloads a remote file once and serves it locally |
| `quiz` | Questions inline, or a YAML file from git |
| `lab` | Steps checked against GitLab ([107](107-backend-labs-and-checker.md)) |
| `modules` | An index YAML file in git, **expanded in place** at request time into a flat list of modules: entries inherit `src` and `ref` from the parent; `type` defaults to `text`; `hidden` and `prerequisites` are allowed; a missing or invalid index is skipped with a warning |

Every module has a `slug` (DNS-safe form of its name), an `index` in the expanded list, optional
`hidden` (admins only), `prerequisites` (module slugs), `skills` (free kebab-case tags).

`GET /api/courses/{slug}/modules` lists modules with the learner's `viewed` flag;
`GET /api/courses/{slug}/modules/{index}` returns one with its `content`.

## Git sources

- Files are fetched with go-git, authenticated by `GIT_TOKEN` or the per-URL tokens of
  `GIT_CREDENTIALS_PATH` ([101](101-backend-configuration.md)).
- A **bounded** cache keeps fetched files for `GIT_CACHE_TTL` minutes (10). Admins clear it with
  `POST /api/admin/cache/clear` or per course with `POST /api/admin/courses/{slug}/cache/clear`.
- The cache is per replica, as it was per course-service replica.

## Markdown import and export

A whole course round-trips through one markdown document:

- Optional front matter: the course `spec` keys, plus `slug` (else the request's, else derived from
  the title: accents folded, `[a-z0-9-]`) and `split` (`none`, `h1`…`h6`; the request may override
  it).
- Modules are cut at the `split` heading level; text before the first heading becomes a leading
  module unless it holds only the document title; headings inside fenced code do not cut.
- A `<!--pupitre … -->` YAML comment under a heading carries what markdown cannot express (`type`,
  git source, `questions`, `skills`, `prerequisites`, `hidden`, `passingScore`, `steps`…). When a
  module takes its body elsewhere, the markdown under its heading is ignored with a warning.
- Import modes: `create` (default, `409` if the slug exists), `replace` (modules, and course fields
  if the document has front matter), `append` (front matter ignored). `replace` and `append`
  answer `404` for an unknown slug.
- `POST /api/admin/courses/import/preview` resolves exactly like the import but writes nothing:
  target slug, course fields, modules with body sizes and an `existing` marker, `warnings`.
- `GET /api/admin/courses/{slug}/export/markdown` picks a heading level no module body uses and
  records it in `split`, so export then import is lossless; `?split=h2` forces a level.
- Admin bodies are capped at 10 MB ([103](103-backend-http-api.md)).

## Quizzes

- Question types `single` and `multiple`, with `points`; the score is compared to `passingScore`
  (percent).
- `maxAttemptsPerQuestion`, `lockOnMaxAttempts` and a `cooldown` between graded attempts
  (`strategy`, `baseSeconds`, `multiplier`, `maxSeconds`), tracked per question in
  `quiz_question_attempts`.
- Correct answers never leave the server before grading.
- A submission records the module result through `Progress.RecordModule` (detached).

## Progress, completion, badges and XP

- `text`, `video` and `image` modules complete when viewed ("lesson completed"); `quiz` and `lab`
  modules complete when passed.
- When the last module completes, the course is complete: the course **badge** is awarded (default
  icon 🏅), skill levels are updated, and XP is granted.
- XP ledger, idempotent per (`user`, `source`, `source_slug`): lesson **10**, passed module **25**,
  course **200**. Levels: Novice (0), Apprenant (200), Confirmé (600), Expert (1500), Maître
  (3500).
- Leaderboard limited to `LEADERBOARD_MAX_ENTRIES` (20).

## Skills

- A course's skills are the union of its modules' `skills`, denormalized on the course row so the
  catalogue filters without reading modules.
- `GET /api/skills/{slug}/modules` and `GET /api/my/skills/{slug}`: every module teaching the
  skill, with a status `completed`, `available` (previous module completed, or first) or `locked`.

## Paths

- Managed under `/api/admin/courses/paths` (`POST`, `GET|PUT|DELETE …/{slug}/definition`).
- `kind: course` paths list ordered courses; `kind: skill` paths list skills unlocked
  **sequentially**: a skill completes when all its graded modules (quiz, lab) are passed.
- `level` is `beginner`, `intermediate` or `advanced`.
- The public `GET /api/paths/{slug}` replaces a course path's stored `skills` with the aggregate of
  its courses; editors read the admin route instead, so the aggregate is never written back.
- Enrollment in a path opens its courses, even private ones.

## Groups, sessions, exports

- Groups are nested (`parent_id`, deletion of a parent with children refused). Members have a role
  in the group; managers see their groups' members, progress and sessions ([030](030-manager-space.md)).
- A group can be enrolled in a course or a path; its members inherit the enrollment.
- Course sessions are dated; learners book them, managers record attendance ([022](022-agenda-and-sessions.md)).
- CSV exports **stream** rows as they are read and are logged in `export_logs`
  ([047](047-admin-exports-and-labs.md)).
- Markdown patterns are shared editor snippets; any signed-in user reads them, admins edit them.

## Performance invariants

Kept and enforced by the `scaling_test.go` of both domains, which count queries and calls while
the data grows:

- **No per-item I/O**: a handler rendering N things issues a number of queries and calls that does
  not depend on N.
- **One progress read per request** (`learnerView` in the courses domain).
- **Outbound HTTP through `httpx`** (git, GitLab, identity providers, media replication): timeouts
  and a shared pool, never `http.DefaultClient`.
- **Large results stream** (`ExportRepository.StreamRows`).
- **Caches are bounded** (catalog cache, git cache).

In the monolith the scaling tests count interface calls where they counted HTTP calls
([104](104-backend-in-process-calls.md)).

## Acceptance criteria

- The handler, repository, content, definition and markdown tests of both services pass in their
  new packages with only import paths changed.
- `wire_contract_test.go` (the fields the frontend reads) passes unchanged.

## Sources

`docs/Course.md`, `docs/Module.md`, `docs/Skills.md`, `docs/ARCHITECTURE.md`, `AGENTS.md` ("Course
source of truth", "Markdown import/export", "Performance invariants"); `course-service/internal/`
(`content`, `definition`, `markdown`, `handlers`); `user-service/internal/handlers/xp.go`,
`badges.go`, `internal.go`; `user-service/internal/repository/xp.go`.
