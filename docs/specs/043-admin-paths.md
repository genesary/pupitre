# 043 - Admin: learning paths

Features: F126 (path management), F127 (path enrollments).

Access: `admin`. Page: `/admin/paths` (« Parcours »).

## Model

A path `spec` (struct `definition.Path`): `title`, `description`, `kind`, `level`, `courses[]`
(slugs, for `kind: course`) or `skills[]` (for `kind: skill`).

| `kind` | Label | Content |
| --- | --- | --- |
| `course` | « Suite de cours » | Ordered list of courses |
| `skill` | « Suite de compétences » | Ordered list of skills |

Levels: `beginner`, `intermediate`, `advanced`, or none (« - Aucun - »).

Read and write **only** through the admin API: `GET` and
`PUT /api/admin/courses/paths/{slug}/definition`, `POST /api/admin/courses/paths` with
`{ slug, spec }`, `DELETE /api/admin/courses/paths/{slug}/definition`. The public route
`GET /api/paths/{slug}` replaces the `skills` of a course-kind path with the aggregate of its
courses' skills: an editor reading it would write that aggregate back as if it had been typed.

## List

- Data: `GET /api/paths` (list), `GET /api/admin/courses` (available courses),
  `GET /api/admin/users` (for enrollments).
- « + Nouveau parcours ». One row per path: title, slug, kind, level, number of steps, and the
  actions « Modifier », « Définition », « Inscriptions », « Supprimer ».

## Create or edit

Dialog: slug (rule of [042](042-admin-courses.md), locked afterwards), title (required),
description, kind, level, then:

- `course`: selection and order of the courses (searchable list, then drag and drop or ↑ ↓
  buttons);
- `skill`: skills entered as chips, with suggestions taken from the courses' skills.

« Créer le parcours » or « Enregistrer ». « Définition » shows the JSON or YAML read only, with
« Copier ».

## Enrollments

« Parcours : <title> » panel:

- `GET /api/admin/paths/{slug}/enrollments` → user, e-mail, role, enrollment date, progress
  (`completedCourses` / `totalCourses`).
- Enroll: « Sélectionner un utilisateur… », then « Inscrire » →
  `POST /api/admin/paths/{slug}/enrollments` with `{ userId }`.
- Unenroll: confirmation, then `DELETE /api/admin/paths/{slug}/enrollments/{userId}`.

## Acceptance criteria

- Create a path of three courses in a given order: an enrolled user's `/my-paths` shows the three
  steps in that order.
- Editing a course-kind path does not add skills that were not entered.

## Sources

`frontend/src/pages/admin/paths/index.astro`; `course-service/internal/handlers/paths_admin.go`,
`paths.go`; `user-service/internal/handlers/paths.go`.
