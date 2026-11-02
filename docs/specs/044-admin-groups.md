# 044 - Admin: groups, subgroups and role mappings

Features: F130 (model), F131 (CRUD), F132 (subgroups and move), F133 (membership), F134 (role
mappings), F135 (group enrollment in a course, see [042](042-admin-courses.md)).

Access: `admin`. Page: `/admin/groups`.

## Model

A group: `id`, `name`, `source` (`local` or coming from SSO), `parentId`, `depth`, `memberCount`,
`mappedRole`. The `everyone` group is the default group of every user.

With Keycloak, SSO groups are synchronized at each sign-in. A **mapping** links an SSO group name
to a platform role.

## Group tree

- Data: `GET /api/admin/groups`.
- Expandable tree (parent → children through `parentId`), with for each group: name, source
  (« SSO » badge for a non-local group), member count, mapped role, and the actions « Renommer »,
  « Nouveau sous-groupe », « Déplacer », « Supprimer ».
- « + Nouveau groupe »: « Nouveau groupe local » dialog (name), then `POST /api/admin/groups` with
  `{ name }`.
- Rename: `PUT /api/admin/groups/{groupId}` with `{ name }`.
- Subgroup: `POST /api/admin/groups/{groupId}/subgroups` with `{ name }`.
- Move: « Déplacer le groupe » dialog, « Nouveau parent » (list without the group and its
  descendants, plus « Racine (aucun parent) »), then `PATCH /api/admin/groups/{groupId}/parent`
  with `{ parentId }` (`null` for the root). Descendants are computed from the group list
  (`parentId`): `GET /api/admin/groups/{groupId}/descendants` returns a struct without json tags
  ([090](090-gaps-and-defects.md) A22).
- Delete: confirmation, then `DELETE /api/admin/groups/{groupId}`. Only local groups can be
  deleted (the server returns 404 otherwise): the button is disabled for an SSO group.

## Members

« Membres du groupe » panel for the selected group:

- `GET /api/admin/groups/{groupId}/members[?recursive=true]`, with the « Inclure les membres des
  sous-groupes » checkbox. In recursive mode, each member has a « Direct » or « Hérité » badge.
- Each member: initial, username, e-mail, role, « Retirer » button (direct members only).
- Add: « Choisir un utilisateur… » (list from `GET /api/admin/users`, searchable), then
  `POST /api/admin/groups/{groupId}/members` with `{ userId }`.
- Remove: `DELETE /api/admin/groups/{groupId}/members/{userId}`.

## Role mappings

« Mappings de rôles » section:

- `GET /api/admin/groups/mappings` → `mappings[] {groupName, platformRole}`.
- « Ajouter un mapping »: « Nom du groupe SSO » and « Rôle plateforme » (« Étudiant »,
  « Manager de groupe », « Administrateur »), then `POST /api/admin/groups/mappings` (create or
  update).
- Delete: `DELETE /api/admin/groups/mappings/{groupName}` (name URL-encoded).
- A note recalls that the `oidc_group_admins` setting ([046](046-admin-settings.md)) is
  authoritative for the admin role: a user outside these groups becomes a student again at their
  next sign-in.

## Acceptance criteria

- Moving a group under one of its descendants is impossible (the option does not exist).
- The « Inclure les membres des sous-groupes » checkbox adds the inherited members, with their
  badge.
- A malicious SSO group name is shown as text.

## Sources

`frontend/src/pages/admin/groups/index.astro`; `user-service/internal/handlers/groups.go`
(groups, mappings), `admin.go` (members).
