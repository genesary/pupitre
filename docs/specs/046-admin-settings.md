# 046 - Admin: platform settings

Feature: F179.

Access: `admin`. Page: `/admin/settings` (« Paramètres »).

## API

- `GET /api/admin/settings` → `settings[] {key, value, description}`. Secrets
  (`oidc_client_secret`, `ldap_bind_password`) come back masked: `********`.
- `PUT /api/admin/settings` with an object `{ key: "value" }` (every value is a string; an unknown
  key returns 400). The response is the updated list.

## Secret rule

A secret field shows `********` when a value exists. It is **sent only if the user changed it**:
sending `********` back would overwrite the real secret with that string. The field offers
« Modifier » to enter a new value.

## Sections

Each section has its own « Enregistrer » button, which sends only the changed keys of the section.

| Section | Keys | Note |
| --- | --- | --- |
| 🔐 « SSO & Authentification » | `sso_local_login_enabled` | Must be **disabled** (Keycloak only, finding H7) |
| 📝 « Inscription » | `registration_enabled`, `registration_email_whitelist` (« Domaines autorisés ») | `registration_enabled` must be **disabled** |
| 🔑 « Politique de mot de passe » | `password_min_length`, `password_require_uppercase`, `password_require_number` | No effect with Keycloak only: section collapsed, with this note |
| 👤 « Profils utilisateurs » | `profile_allow_username_change` | Must be **disabled** |
| 🌐 « OIDC (OpenID Connect) » | `oidc_enabled`, `oidc_provider_url` (« URL de découverte du provider »), `oidc_issuer_url` (« Override issuer URL »), `oidc_client_id`, `oidc_client_secret`, `oidc_scopes`, `oidc_group_claim` (« Claim des groupes »), `oidc_group_admins`, `oidc_redirect_base` (« URL de base pour redirect »), `oidc_browser_base_url` (« URL navigateur ») | The main section |
| 🗂️ « LDAP / Active Directory » | `ldap_enabled`, `ldap_server_url`, `ldap_bind_dn`, `ldap_bind_password`, `ldap_user_base_dn`, `ldap_user_filter`, `ldap_group_base_dn`, `ldap_group_filter` | Not used: section collapsed, « Non utilisé avec Keycloak » |
| GitLab | `gitlab_url` (« URL instance GitLab ») | Used by labs |

The `oidc_insecure_skip_verify` setting exists on the server but is not in the list of writable
keys: it is not shown.

## Warnings

A banner at the top of the page flags the settings that are dangerous for a Keycloak-only
platform:

- `registration_enabled` not `false`;
- `sso_local_login_enabled` not `false`;
- `profile_allow_username_change` not `false`;
- `oidc_enabled` not `true`.

Each line of the banner has a « Corriger » button that saves the right value.

## Acceptance criteria

- Saving the OIDC section without touching the secret does not send `oidc_client_secret`.
- An instance with the default values shows the three local-authentication warnings.

## Sources

`frontend/src/pages/admin/settings/index.astro`; `user-service/internal/handlers/settings.go`.
