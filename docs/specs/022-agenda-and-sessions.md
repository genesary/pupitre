# 022 - Agenda, in-person sessions, booking and attendance

Features: F150 (in-person courses and sessions), F151 (session management), F152 (booking), F153
(my bookings), F154 (agenda), F155 (attendance).

## Model

An `inPerson` course holds a `sessions[]` list: `id`, `title`, `date` (ISO 8601), `location`,
`capacity` (0 = unlimited). Sessions are read from `GET /api/courses` and
`GET /api/courses/{slug}`. Bookings live in user-service.

## Agenda - `/agenda` (signed in)

Data: `GET /api/courses` (`inPerson` courses and their sessions), `GET /api/my/session-bookings`.

- **Monthly calendar**: a "month year" header with ‹ and ›, an « Aujourd'hui » button, and a month
  and year picker (click on the month label). Weeks run Monday to Sunday.
- Each day shows up to three session pills (session title, with the tooltip « course title -
  session title »), then « +N autres ». The current day is highlighted.
- **Side panel**:
  - by default, « Sessions à venir »: the next sessions, any date;
  - after clicking a day: the date as the title (« LUNDI 3 NOVEMBRE ») and the sessions of that
    day.
- A session in the panel: the course (link), the title, the time, the location (📍), the
  capacity, and:
  - a « Réserver » or « ✓ Réservé - Annuler » button (`POST` or `DELETE`
    `/api/session-bookings/{slug}/{sessionId}`) if the session is not in the past;
  - for an admin or a manager: « Modifier », « Supprimer » and « Présence » (link to the
    attendance page).
- The booking count **may** be shown next to the capacity
  (`GET /api/session-bookings/{slug}/{sessionId}/count`).

### Session management (admin or manager)

- « + Nouvelle session » button at the top of the agenda.
- « Nouvelle session » or « Modifier la session » dialog:
  - course (list of `inPerson` courses, locked when editing);
  - title, date and time, location: required;
  - capacity: a number ≥ 0.
- Creation: `POST /api/admin/courses/{slug}/sessions` with `{ title, date, location, capacity }`.
- Update: `PUT /api/admin/courses/{slug}/sessions/{sessionId}` with the same body.
- Deletion: confirmation `AlertDialog`, then `DELETE /api/admin/courses/{slug}/sessions/{sessionId}`.
- After each action, the courses are read again. Errors are shown in a toast (no more `alert()`).
  A missing value is shown below the field concerned.

## Attendance - `/admin/courses/sessions/:slug/:sessionId` (admin or manager)

The page has no admin sidebar (the Astro page uses the learner layout), with a « ← Retour à
l'agenda » link.

- Data: `GET /api/courses/{slug}` (course and session titles) and the participant list:
  - admin: `GET /api/admin/session-bookings/{slug}/{sessionId}`;
  - manager: `GET /api/manager/session-bookings/{slug}/{sessionId}`.
- Header: session title, course, date.
- Three counters: « Inscrits », « Présents », « Absents ».
- Table: participant, e-mail, booking date, « Présent » checkbox.
- Checking or unchecking: `PATCH …/{userId}/presence` with `{ present }`. Optimistic update; on
  error, the box goes back to its state and a toast is shown.
- No participant: « Aucun participant pour cette session. »

On the server, a manager can today read and change the attendance of **any** session (audit
finding H2). The frontend shows these links only for the sessions of the agenda, but the
restriction must be enforced by the backend.

## Acceptance criteria

- Booking a session from the agenda marks it « Réservé » in the agenda and on the course page.
- A learner sees neither « + Nouvelle session » nor the management buttons.
- Creating a session without a location shows an error below the field, with no API call.
- Unchecking attendance while offline checks the box again and shows an error toast.

## Sources

`frontend/src/pages/agenda.astro`, `pages/admin/courses/sessions/[slug]/[sessionId].astro`, the
sessions part of `pages/courses/[slug]/index.astro`;
`course-service/internal/handlers/sessions_admin.go`; `user-service/internal/handlers/sessions.go`.
