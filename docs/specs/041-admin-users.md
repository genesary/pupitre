# 041 - Admin: users

Features: F162 (user list), F163 (edit, deactivate, delete).

Access: `admin`.

## List - `/admin/users`

- Data: `GET /api/admin/users[?provider=<id>]` → `{ users[], total }`;
  `GET /api/admin/users/providers` → `providers[] {provider, count}`.
- Filter by provider (tabs or a select: « Tous », then each provider with its count). With
  Keycloak only, this will mostly be `oidc` (and `local` for the bootstrap admin account).
- Local search by username or e-mail.
- Table: user (avatar, username, e-mail), role, provider, enrolled courses, sign-up date, active,
  actions.
- Displayed roles: `student` « Étudiant », `manager` « Manager de groupe », `admin`
  « Administrateur ».
- Client-side pagination beyond 50 rows.

## Editing

« Modifier l'utilisateur » dialog:

- role (`student`, `manager`, `admin`);
- active account (switch).

Saving: `PUT /api/admin/users/{userId}` with `{ role, isActive }`. The list is read again
afterwards.

Username, bio and avatar are not editable from the admin: they come from Keycloak or from the
profile.

**Note**: a role change or a deactivation only takes effect when the user's token expires
(8 hours), because the backend does not revoke tokens (finding H3). The dialog says so: « Le
changement s'appliquera à la prochaine connexion de l'utilisateur (au plus tard dans 8 h). »

## Deletion

`AlertDialog` « Supprimer <username> ? Cette action est irréversible. », then
`DELETE /api/admin/users/{userId}`.

## Acceptance criteria

- Making a user a `manager` updates the row without a reload.
- A malicious username is shown as text in the table and in the deletion dialog.
- An admin cannot delete themselves nor remove their own admin role from the interface (the
  buttons are disabled on their own row).

## Sources

`frontend/src/pages/admin/users/index.astro`; `user-service/internal/handlers/admin.go`
(`ListUsers`, `ListAuthProviders`, `GetUser`, `UpdateUser`, `DeleteUser`).
