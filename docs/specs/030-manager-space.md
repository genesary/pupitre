# 030 - Manager space ("Mon espace")

Features: F140 (manager scope), F141 (members), F142 (groups), F143 (learning paths).

Access: `manager` role only ([002](002-roles-and-guards.md)). Sidebar:
[005](005-ui-theme-navigation.md).

## Scope

A manager's scope is the set of members of the groups the manager belongs to, except the default
group `everyone`. It is computed **by the server**; the frontend shows what `/api/manager/*`
returns.

## My members - `/manager/users`

- Data: `GET /api/manager/users` → `{ users[], total, groups[] {id, name, memberIds} }`, and
  `GET /api/courses`, `GET /api/paths` for the choice lists.
- Table: member (avatar, username, e-mail), enrolled courses, actions.
- Filter by group (the groups of the scope; the filter of the Astro page never worked,
  [090](090-gaps-and-defects.md) A21) and search by name or e-mail.
- « Gérer les inscriptions » opens a panel for the chosen member:
  - enrolled courses: `GET /api/manager/users/{userId}/enrollments`; enroll (« Choisir un
    cours… », « Inscrire ») through `POST /api/manager/enrollments/{slug}` with `{ userId }`;
    unenroll through `DELETE /api/manager/enrollments/{slug}/users/{userId}`;
  - learning paths: `GET /api/manager/users/{userId}/path-enrollments` → `pathEnrollments[]` with
    the Go field names `Slug` and `EnrolledAt` (the struct has no json tags, A22); enroll through
    `POST /api/manager/paths/{slug}/enrollments` with `{ userId }`; unenroll through
    `DELETE /api/manager/paths/{slug}/enrollments/{userId}`.
- Unenrolling asks for confirmation.

## My groups - `/manager/groups`

- Data: `GET /api/manager/groups` → groups of the scope (`id`, `name`, `parentId`, `depth`,
  `memberCount`), shown as a tree.
- « + Nouveau groupe »: `POST /api/manager/groups` with `{ name }` (the manager becomes a member).
- Subgroup: `POST /api/manager/groups/{groupId}/subgroups` with `{ name }` (the manager becomes a
  member, so the subgroup stays in their scope and their list: [190](190-backend-defects.md) B1).
- Delete: confirmation, then `DELETE /api/manager/groups/{groupId}`.
- Members of the selected group: `GET /api/manager/groups/{groupId}/members`; add (« Choisir un
  utilisateur… », from `/api/manager/users`) through `POST /api/manager/groups/{groupId}/members`
  with `{ userId }`; remove through `DELETE /api/manager/groups/{groupId}/members/{userId}`.

The « Choisir un utilisateur… » list offers only members of the scope. On the server, adding
accepts **any** `userId`, which lets a manager widen their scope (audit finding H2): the fix is on
the backend.

## Learning paths - `/manager/paths`

- Data: `GET /api/paths` (list), `GET /api/manager/users` (members).
- Choose a path, then a table of its enrolled members within the scope:
  `GET /api/manager/paths/{slug}/enrollments` → member, enrollment date, progress
  (`completedCourses` / `totalCourses`).
- Enroll a member (« Sélectionner un membre… », « Inscrire »):
  `POST /api/manager/paths/{slug}/enrollments`.
- Unenroll: confirmation, then `DELETE /api/manager/paths/{slug}/enrollments/{userId}`.

## Acceptance criteria

- A manager with no group sees empty lists with a message, and no error.
- Enrolling a member in a course updates their course count in the table without a reload.
- Every name (members, groups) is shown as text ([003](003-security.md) S1).

## Sources

`frontend/src/pages/manager/users/index.astro`, `manager/groups/index.astro`,
`manager/paths/index.astro`, `layouts/Manager.astro`; `user-service/internal/handlers/manager.go`,
`paths.go`.
