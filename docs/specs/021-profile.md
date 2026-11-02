# 021 - Profile, avatar and bio

Features: F035 (edit profile), F036 (change password).

## Route

`/profile`, signed in. Reached from the avatar in the top bar.

## Data

- `GET /api/auth/me`: the user ([001](001-keycloak-authentication.md)).
- `GET /api/my/badges`: the emojis available for the avatar.
- `PUT /api/auth/profile` with `{ bio }` or `{ avatarUrl }`: returns the updated user, which
  replaces the one in the session (the top bar updates).

## Display

- Avatar ([003](003-security.md) S3), username, e-mail.
- Badges: « Admin » for an admin, and the identity provider (« Keycloak » for `oidc`).
- Read-only fields: username, e-mail, bio (« - » when empty).

## Editing

- **Bio**: a `Textarea`, with « Enregistrer » (« Enregistrement… » during the call) and
  « Annuler » buttons. Error → message below the field.
- **Avatar**: clicking the avatar opens a menu with the emojis of the earned badges. Choosing one
  saves it (`{ avatarUrl: "<emoji>" }`) and shows the toast « Photo de profil mise à jour ».
  Without an earned badge, the menu shows « Obtenez des badges pour débloquer des avatars ».
- Entering an **external URL** is not offered ([003](003-security.md) S3, issue
  genesary/pupitre#187).
- **Username**: not editable (Keycloak is the source, and `profile_allow_username_change` must be
  `false`, see [001](001-keycloak-authentication.md)).
- **E-mail**: not editable.
- **Password**: no form. The « Sécurité » section shows « Votre compte est géré par Keycloak. La
  gestion du mot de passe se fait dans votre compte Keycloak. » If the API one day exposes the
  Keycloak account console URL, the section **may** add a link to it.

## Acceptance criteria

- Choosing an emoji updates the avatar on the profile and in the top bar without a reload.
- A bio containing `<script>` is shown as text after saving.
- No field allows entering an avatar URL.

## Sources

`frontend/src/pages/profile.astro`; `user-service/internal/handlers/auth.go` (`Me`,
`UpdateProfile`).
