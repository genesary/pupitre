# 045 - Admin: markdown patterns

Features: F180 (markdown patterns), F181 (seeding from configuration).

Access: `admin`. Page: `/admin/patterns` (« Markdown Patterns »).

## Model

A pattern: `name` (identifier used in markdown: `|||blink`), `label`, `scope` (default `global`),
`html` (required; `{{content}}` is replaced by the content), `css`, `js`.

Syntax and rendering are described in [012](012-lesson-player.md). HTML and CSS are sanitized when
displayed, and the `js` field is **never executed** ([003](003-security.md) S2). The current
frontend labels it "executed after rendering", but runs it nowhere.

## API

- Read: `GET /api/patterns` (signed in) and `GET /api/patterns/{id}`.
- Write (admin): `POST /api/admin/patterns`, `PUT /api/admin/patterns/{name}`,
  `DELETE /api/admin/patterns/{name}`.

## Page

- Pattern list: name, label, scope, excerpt of the HTML, actions « Modifier », « ⎈ YAML »,
  « Supprimer ».
- « + Nouveau pattern » or « Modifier »: form with name (required, locked when editing), label
  (required by the server on creation; the name is used when it is left empty, A25),
  scope, HTML (required, monospace editor), CSS (monospace editor), JS (monospace editor, with the
  note « non exécuté par la plateforme »).
- Live **preview**: a block example and an inline example, rendered with the pattern being edited,
  by the same pipeline as lessons.
- « ⎈ YAML »: the pattern definition in the format accepted by seeding, with « Copier ».
- Delete: confirmation, then `DELETE`.

## Acceptance criteria

- Editing the HTML updates the preview without saving.
- A pattern whose HTML contains `<script>` is shown in the preview without running it.

## Sources

`frontend/src/pages/admin/patterns/index.astro`; `user-service/internal/handlers/patterns.go`.
