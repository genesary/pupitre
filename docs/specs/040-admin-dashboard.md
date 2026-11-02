# 040 - Admin: dashboard, monitoring and leaderboard

Features: F115 (score leaderboard), F160 (dashboard), F161 (monitoring), F184 (progress
resynchronization).

Access: `admin`. Sidebar: [005](005-ui-theme-navigation.md).

## Dashboard - `/admin`

- Data: `GET /api/admin/stats`, `GET /api/admin/leaderboard`.
- Statistic cards:

  | Returned key | Label |
  | --- | --- |
  | `total_users` | « Utilisateurs » |
  | `totalCourses` | « Cours » |
  | `total_enrollments` | « Inscriptions » |

  The server also returns `total_labs`, `total_submissions` and `success_rate`, but always as `0`
  (hard-coded in `AdminStats`). They are **not** shown until the backend computes them
  ([090](090-gaps-and-defects.md)).
- « Classement » block: the top five of the admin leaderboard (rank, avatar, username, score),
  with a link to `/admin/leaderboard`.
- Shortcuts to the sidebar sections, as cards.

## Monitoring - `/admin/monitoring`

- Data: `GET /api/admin/stats`, `GET /api/admin/courses`, `GET /api/admin/leaderboard`.
- Same statistic cards as the dashboard.
- « Aperçu des cours »: table of title, category, number of modules, number of enrolled users,
  access (public or private: the course list does not say whether a course is hidden), sorted by
  enrolled users, descending.
- « Top 10 - Classement ».
- No « Recalculer la progression » button: `POST /api/admin/sync-progress` marks **one** lesson as
  completed for one user (body `{ userId, courseSlug, lessonSlug }`), and the Astro button sent no
  body, so it always failed ([090](090-gaps-and-defects.md) A23). A global recalculation needs a
  backend route.

## Leaderboard - `/admin/leaderboard`

- Data: `GET /api/admin/leaderboard` → `id`, `username`, `email`, `avatarUrl`, `totalScore`,
  `passedModules`, `enrolledCourses`.
- « Classement général » title, table of rank, user (avatar, username, e-mail), score, passed
  modules, enrolled courses.
- No recalculation button (A23).

## Acceptance criteria

- No card shows a value the server hard-codes to `0`.
- No button calls `POST /api/admin/sync-progress` without its body.

## Sources

`frontend/src/pages/admin/index.astro`, `admin/monitoring/index.astro`,
`admin/leaderboard/index.astro`, `admin/[...slug].astro`;
`user-service/internal/handlers/admin.go` (`AdminStats`, `AdminLeaderboard`, `SyncProgress`).
