# 042 - Admin: courses, definition, modules, markdown import / export, enrollments, cache

Features: F145 (course enrollments), F165-F168 (list, creation, metadata, deletion), F169-F172
(definition, viewer, module editor, reordering), F173-F176 (markdown document, import, preview,
export), F177 (git cache), F185 (slug rules).

Access: `admin`. Page: `/admin/courses`.

## Definition model

A course is read and written as a whole through its definition:

- `GET /api/admin/courses/{slug}/definition` → `{ slug, spec }`;
- `PUT /api/admin/courses/{slug}/definition` with `{ spec }` (full replacement);
- `POST /api/admin/courses` with `{ slug, spec }` (creation);
- `DELETE /api/admin/courses/{slug}/definition`.

`spec` (struct `definition.Course`): `title`, `description`, `public`, `hidden`, `category`,
`difficulty`, `scope`, `xpRequired`, `inPerson`, `badge {name, icon}`, `modules[]`,
`prerequisites[] {course, minScore, modules}`, `sessions` (object keyed by identifier).

A module (`content.Module`): `name`, `type` (`text`, `video`, `image`, `quiz`, `lab`, `modules`),
`content` (markdown) **or** `src` / `ref` / `path` (git), `labUrl`, `replication`, `hidden`,
`inline`, `prerequisites[]`, `questions[]`, `passingScore`, `cooldown`, `maxAttemptsPerQuestion`,
`lockOnMaxAttempts`, `checkProvider`, `checkType`, `checkParams`, `steps[]`, `skills[]`.

**Concurrency rule**: every change reads the definition again, applies the change and sends it
back whole. Two admins editing the same course can overwrite each other: the editor **must** read
the definition again right before writing and warn if it changed since it was opened (JSON
comparison).

## Course list

- Data: `GET /api/admin/courses` → `{ courses[], total }`, and `GET /api/admin/groups` for group
  enrollments.
- Header: « N cours », buttons « + Nouveau cours », « ⬆ Importer du markdown »,
  « 🗑 Vider tout le cache ».
- One row per course: title, slug, category, difficulty, public or private, number of modules,
  and the actions « Modifier », « ⎈ Définition », « ↓ MD », « Supprimer ».
- Clicking the row expands it, with two tabs: **Modules** and **Inscriptions**.

## Create or edit a course

Dialog (or panel) with:

- slug: required on creation, locked afterwards. Format: lowercase letters, digits, hyphens
  (`^[a-z0-9]+(-[a-z0-9]+)*$`). It must not be `batch` or another reserved segment;
- title (required), description, category, difficulty (« Débutant », « Intermédiaire »,
  « Avancé »);
- public (checkbox), hidden (checkbox);
- in person (checkbox), required XP (number ≥ 0), badge (name and emoji).

Creation: `POST /api/admin/courses`. Update: read the definition again, then `PUT …/definition`
with the changed metadata and the original modules.

## Definition viewer

« ⎈ Définition » opens a « Définition - <slug> » dialog that shows the definition as YAML (read
only), with a « Copier » button.

## Module editor (Modules tab)

- List: the `modules` of the definition (hidden modules and git sources included), so the list
  and the changes come from the same read.
- Each module: type, name, badges (hidden, inline, git), actions « Modifier », « Supprimer », and
  « 🗑 » (clear this module's cache) for a git module.
- **Edit mode**: a toggle that shows the drag handles for reordering. A changed order is saved in
  a single `PUT …/definition`.
- « Ajouter » or « Modifier » module form:
  - name (required), type;
  - for `text` and `lab`: a markdown editor (monospace text area, with a **preview** rendered by
    the pipeline of [012](012-lesson-player.md)), with the hint « Laissez vide pour le lire depuis
    un dépôt git à la place. »;
  - git source: repository URL, branch or tag (default `main`), path in the repository;
  - hidden, inline quiz (`inline`, for the `quiz` type only).
  - If both `content` and `src` are filled, git wins on the server: the form **clears one when the
    other is filled**, and says so.
  - The « Quiz ref » field of the current frontend does not exist in the definition (the server
    ignores it): it is not carried over ([090](090-gaps-and-defects.md)).
- Deleting a module: confirmation « Supprimer le module « <nom> » ? », then `PUT …/definition`
  without that module.
- Advanced fields (quiz questions, lab steps, module prerequisites, skills) are edited **in a JSON
  editor** for the module, validated before sending. A dedicated question editor **may** come
  later.

## Markdown import

« Importer depuis du markdown » dialog:

- **Document**: pasted text or a `.md` file (read in the browser, with its size in KB). The
  server limit is 10 MB.
- **Destination** (`mode`):
  - « Nouveau cours » (`create`);
  - « Remplacer les modules d'un cours » (`replace`);
  - « Ajouter des modules à un cours » (`append`).
  For `replace` and `append`, choose the « Cours cible ». For `create`, an optional slug that
  overrides the frontmatter one.
- **Split** (`split`): « Auto (frontmatter, sinon un seul module) » (empty), « Un seul module »
  (`none`), « # - un module par titre de niveau 1 » (`h1`), « ## - niveau 2 » (`h2`),
  « ### - niveau 3 » (`h3`).
- « Prévisualiser »: `POST /api/admin/courses/import/preview` with
  `{ markdown, slug, split, mode }` →
  `{ slug, mode, course, modules[] {name, slug, type, bytes, hidden, src, path, existing}, warnings[] }`.
  The preview shows the resulting slug, the number of modules, the list (type, name, size, or
  repository and path for a git module, "existing" in `append` mode) and the warnings.
- « Importer »: `POST /api/admin/courses/import` with the same body →
  `{ slug, mode, moduleCount, warnings }`. Success toast, dialog closed, list read again.

## Markdown export

« ↓ MD »: `GET /api/admin/courses/{slug}/export/markdown`, downloaded as `<slug>.md` (non-JSON
response, see [004](004-api-and-routing.md)).

## Enrollments (Inscriptions tab)

- Data: `GET /api/admin/enrollments/{slug}` → `enrollments[] {userId, username, email, enrolledAt}`;
  `GET /api/admin/enrollments/{slug}/groups` → `groups[] {id, name, source, memberCount, enrolledAt}`.
- **Groups**: choose a group and « Inscrire » → `POST /api/admin/enrollments/{slug}/groups` with
  `{ groupId }` (the response gives the number of members enrolled, shown in the toast); remove →
  `DELETE /api/admin/enrollments/{slug}/groups/{groupId}`.
- **Individual users**: search with suggestions (`GET /api/admin/users/search?q=`, 250 ms
  debounce, at least 2 characters), then « Inscrire » → `POST /api/admin/enrollments/{slug}` with
  `{ userId }`; remove → `DELETE /api/admin/enrollments/{slug}/users/{userId}`.
- Each removal asks for confirmation.

## Git cache

- « 🗑 Vider tout le cache »: confirmation « Vider tout le cache Git ? », then
  `POST /api/admin/cache/clear`.
- Per course: `POST /api/admin/courses/{slug}/cache/clear` (the toast shows the number of
  repositories cleared).
- Per module: `POST /api/admin/courses/{slug}/modules/{index}/cache/clear`.

## Acceptance criteria

- Create a course `test-cours`, add two modules, reorder them, reload: the order is kept.
- A slug `Test Cours` is refused before sending, with a message below the field.
- Importing a document with `h2` shows a preview with one module per `##` heading, then creates
  the course.
- If the definition changed since the form was opened, saving warns instead of overwriting.
- Every title and name is shown as text ([003](003-security.md) S1).

## Sources

`frontend/src/pages/admin/courses/index.astro`;
`course-service/internal/handlers/admin_definition.go`, `courses_admin.go`, `courses_import.go`,
`internal/definition/definition.go`, `internal/content/types.go`;
`user-service/internal/handlers/admin.go` (enrollments).
