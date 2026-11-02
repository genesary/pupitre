# 011 - Course page, enrollment, prerequisites, badge and sessions

Features: F045 (course page), F046 (visibility), F047 (self-enrollment), F048 (automatic
enrollment in public courses), F049 (XP-gated enrollment), F050 (prerequisites).

## Route

`/courses/:slug`, public. A private course the user may not see returns 404: the page shows
« Cours introuvable » with a link to the catalogue.

## Data

| Call | When |
| --- | --- |
| `GET /api/courses/{slug}` (with the token when signed in) | Always |
| `GET /api/my/courses` and `GET /api/my/badges` | Signed in, in parallel |
| `GET /api/courses/{slug}/modules` | Signed in and enrolled |
| `GET /api/my/session-bookings` | Signed in, `inPerson` course with sessions |

The user is enrolled if `GET /api/my/courses` contains a course whose `slug` (or `id`) equals
`:slug`. A 403 on the module list means the prerequisites are not met.

## Header

- Thumbnail (see [010](010-catalogue.md)), title, description.
- Tags: difficulty, category, skills (links to `/skills/:slug`).
- **Course badge** (if `badge`): icon (🏅 by default) and name. If the user owns this badge
  (`/api/my/badges` contains `courseSlug === :slug`): « ✓ Badge obtenu », link to `/my-badges`.
  Otherwise « Badge à obtenir », with no link.

## Enrollment block

| Situation | Display |
| --- | --- |
| Not enrolled, public course | « S'inscrire au cours » button. Not signed in → `/login?redirect=/courses/:slug`. Signed in → `POST /api/enrollments/{slug}`, then the data is refreshed (no page reload) |
| Not enrolled, private course | « 🔒 Accès restreint - contactez un administrateur » |
| Enrolled, no module | Nothing (the list shows « Aucun contenu disponible pour le moment. ») |
| Enrolled, with modules | Progress, XP card and action button (see below) |

An enrollment failure is shown in an error toast with the API message (prerequisites, not enough
XP…). Today the error is ignored and the page reloads.

### Progress

- A module counts as done if: quiz → `passed`, otherwise `viewed`. Inline quizzes (`inline`)
  count too.
- « X/Y complétés » and the percentage, with a progress bar.
- « ⚡ XP de ce cours » card: `round(pct × 200 / 100)` / 200 XP, or « +200 XP ✓ » at 100 %. Link
  to `/my-xp`. The 200 value is hard-coded in the frontend ([090](090-gaps-and-defects.md)).

### Action button

- Label: « Commencer le cours ▸ » (0 %), « Continuer le cours ▸ », « Revoir le cours ▸ » (100 %).
- Destination: the first module that is neither done nor locked. For an inline quiz, it is the
  lesson that hosts it (see below). Otherwise, the first module of the list.

## Module list

- Section title with « N modules » (inline quizzes excluded).
- **Inline quizzes** (`inline: true`) have no row: they are shown at the end of the lesson before
  them (the last non-inline module with a lower index, called the "host"). Their state appears on
  the host row: « ✓ Quiz » or « 🧩 Quiz à valider ».
- Each row:
  - icon: 🔒 locked, 🎬 video, 🖼️ image, 🧩 quiz, 🧪 lab; ✓ when done;
  - module name and module skills (links);
  - badges: type (Quiz, Vidéo, Image, Lab, Lecture), then « Verrouillé », or for a quiz
    « ✓ bestScore/maxScore pts », « N tentative(s) » or « ★ quiz », or « ✓ Vu »;
  - a link to `/courses/:slug/quiz/:index` (quiz) or `/courses/:slug/lessons/:index` (others),
    absent if the module is locked.
- Prerequisites not met (403): instead of the list, « Terminez d'abord les cours prérequis pour
  accéder à ce contenu », with the course's `prerequisites` list (links to these courses).
- Not enrolled: « Inscrivez-vous pour accéder au contenu du cours. »

## In-person sessions

If `inPerson` and `sessions` is not empty (see also [022](022-agenda-and-sessions.md)):

- One card per session: title, date (« lundi 3 novembre 2026 à 14:00 »), location (📍), capacity
  (« 🪑 N places » if > 0).
- Past session: greyed-out card, no button.
- Signed in: « Réserver » button (`POST /api/session-bookings/{slug}/{sessionId}`) or
  « ✓ Réservé - Annuler » (`DELETE`). After the action, the bookings are read again.
- An error is shown in a toast (no longer in an `alert()`).

## Acceptance criteria

- Not signed in, public course: « S'inscrire au cours » leads to `/login?redirect=/courses/:slug`,
  then back to the page after signing in.
- Enrolled, course with an inline quiz after lesson 2: no row for the quiz, lesson 2's row shows
  « 🧩 Quiz à valider », and the action button leads to lesson 2 until the quiz is passed.
- A locked module has no link.
- A description containing HTML is shown as text.

## Sources

`frontend/src/pages/courses/[slug]/index.astro`; `course-service/internal/handlers/courses.go`
(`GetCourse`), `modules.go` (`ListModules`, `ensureModuleAccess`);
`user-service/internal/handlers/enrollments.go`, `sessions.go`.
