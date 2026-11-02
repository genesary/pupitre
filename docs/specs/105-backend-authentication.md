# 105 - Backend: authentication and authorization

Authentication lived in user-service; course-service only verified tokens. In the monolith the
`users` domain keeps every sign-in method and token minting, and the shared `platform/auth`
package verifies tokens for every domain. Nothing changes for a client: a token issued by
user-service at `3cdf7b2` is accepted by the monolith, and the reverse.

The new frontend signs in through Keycloak only ([001](001-keycloak-authentication.md)). The
backend keeps **every** method below anyway: they are configured by settings, other clients may
use them, and dropping one would lose behaviour. Disabling one is a setting, not a code change.

## Session token

| Property | Value |
| --- | --- |
| Format | JWT, `HS256` only (any other `alg` is refused before the signature is checked) |
| Secret | `JWT_SECRET` ([101](101-backend-configuration.md)) |
| Lifetime | `JWT_EXPIRY_HOURS`, 8 h by default |
| Claims | `sub` (user id), `email`, `role`, `preferred_username`, `iss`, `aud`, `exp`, `iat` |
| Issuer | `user-service`: **kept**, though the service is gone, so live tokens stay valid |
| Audience | `pupitre-api`, required on parse |
| Transport | `Authorization: Bearer <token>`; the frontend keeps it in `sessionStorage` ([003](003-security.md)) |

Guards, one implementation for every route:

| Guard | Allows | Refusals |
| --- | --- | --- |
| `Auth` | any valid token with a subject | `401 Missing Authorization header`, `401 Invalid token`, `401 Invalid token: missing user ID` |
| `Admin` | role `admin` | as `Auth`, then `403` |
| `Manager` | role `manager` | as `Auth`, then `403` |
| `ManagerOrAdmin` | role `manager` or `admin` | as `Auth`, then `403` |

Roles are `student`, `manager` and `admin`. Group roles (manager of a group) are checked by the
handlers, not by the guards.

### Session check and revocation

Since [190](190-backend-defects.md) B7, every guard also reads the user's row (one primary-key
read per request):

- an unknown or deactivated user, or a token issued at or before the user's
  `sessionsvalidafter`, is refused with `401 Session expired`;
- the role checked is the row's, not the token's, so a role change takes effect at once;
- `POST /api/auth/logout`, a password change and a deactivation set `sessionsvalidafter` to now:
  every token issued until then stops working. JWT times have a one-second resolution, so a token
  issued in the same second is revoked too.

Tokens issued before the fix have no revocation date to compare with and stay valid until they
expire.

## Sign-in methods

All the routes are in [103](103-backend-http-api.md), under `/api/auth`.

### Dedicated OIDC (Keycloak)

The full-featured single provider, used by the frontend.

- Settings `oidc_enabled`, `oidc_provider_url`, `oidc_issuer_url`, `oidc_client_id`,
  `oidc_client_secret`, `oidc_scopes` (default `openid email profile groups`), `oidc_group_claim`
  (default `groups`), `oidc_group_admins`, `oidc_redirect_base`, `oidc_browser_base_url`,
  `oidc_insecure_skip_verify`. They can be edited in `/admin/settings` and are **seeded at
  start-up** from the `OIDC_*` variables ([101](101-backend-configuration.md)).
- `GET /api/auth/oidc/authorize` redirects to the provider with a signed `state`
  (`OAUTH_STATE_SECRET`); `POST /api/auth/oidc/callback` exchanges the code and returns a token.
- The `state` is bound to the browser ([190](190-backend-defects.md) B13): `authorize` also sets
  the HttpOnly cookie `pupitre_oauth_state` (`SameSite=Lax`, path `/api/auth`, 10 minutes,
  `Secure` when the redirect base is HTTPS) holding a nonce that the `state` carries. A callback
  without that cookie, or with another nonce, is refused with `401`; the cookie is cleared after
  use. The OAuth providers' `authorize` and `callback` work the same way. The frontend's calls
  are same-origin, so the browser sends the cookie on its own.
- **Split horizon**: discovery uses the internal URL, browser redirects use
  `oidc_browser_base_url`, and the token's issuer is checked against `oidc_issuer_url`.
- **Group to role**: the groups of `oidc_group_claim` are synced as platform groups; a group
  mapped in `group_role_mappings` (`admin`, `manager` or `student`, [190](190-backend-defects.md)
  B20) sets the role. A group mapped to `admin` makes the user `admin`; otherwise a mapped group
  replaces the role of a user who is not `admin`, so a manager in a group mapped to `student`
  becomes `student`. Without a mapped group, the stored role is kept.
- **`oidc_group_admins`**, when set, is authoritative: a member of one of those groups is `admin`,
  anyone else is `student`, whatever the mappings or the stored role. A manager named by hand
  loses the role at the next OIDC sign-in, as in the reference chart (`sso.oidc.groupAdmins`).
- `authProvider` stored on the user: `oidc`.

### OAuth providers (`sso.providers`)

- Several providers at once, declared in the configuration file (`providers[]`), secrets from
  `SSO_<ID>_CLIENT_SECRET[_FILE]`.
- `GET /api/auth/oauth/{id}/authorize`, `POST /api/auth/oauth/callback`.
- `github` uses OAuth2 directly; any other id uses OIDC discovery on its `issuerUrl`. Scopes are
  fixed (`openid email profile`).
- **No group to role mapping**: users get the default group and role. `authProvider` is the
  provider id, so the id must never change.

### LDAP / Active Directory

- Settings `ldap_enabled`, `ldap_server_url`, `ldap_bind_dn`, `ldap_bind_password`,
  `ldap_user_base_dn`, `ldap_user_filter` (`%s` is the email), `ldap_group_base_dn`,
  `ldap_group_filter` (`%s` is the user DN).
- `POST /api/auth/ldap/login`: bind with the service account (or anonymously), search the user by
  email, bind again as the user with the given password, sync groups if `ldap_group_base_dn` is
  set (role mappings apply), issue a token.

### Local accounts

- `POST /api/auth/register` (when `registration_enabled` is true and the email domain matches
  `registration_email_whitelist`, if set), `POST /api/auth/login`.
- A failed sign-in answers `401 Invalid email or password`, whether the email is unknown, belongs
  to an SSO account or the password is wrong, in about the same time
  ([190](190-backend-defects.md) B14).
- Password policy from settings: `password_min_length` (default 8),
  `password_require_uppercase`, `password_require_number`. Passwords are hashed with bcrypt.
- Password change refused for SSO-managed accounts. Username change allowed unless
  `profile_allow_username_change` is `false`.
- The bootstrap `admin` account is described in [102](102-backend-data-and-migrations.md).

### Profile sync on SSO sign-in

Avatar always updated from the provider; bio filled on first sign-in only, never overwritten
afterwards; display name set at creation only (`name`, then `preferred_username`, then email).
For OIDC, UserInfo claims win over ID token claims.

## Rate limits

`register`, `login`, `oauth/callback`, `oidc/callback` and `ldap/login` are limited per client IP:
`AUTH_RATE_LIMIT_REQUESTS` per `AUTH_RATE_LIMIT_WINDOW_SECONDS`, 10 per 60 s by default, `429`
beyond. The same limit applies per email named in the body, whatever the address the attempts come
from. The limiters are in memory, per replica.

The client IP is the peer's address, unless the peer is in `TRUSTED_PROXIES`: then it is the
right-most address of `X-Forwarded-For` that is not a trusted proxy, or `X-Real-IP`
([190](190-backend-defects.md) B12). A client cannot choose its address by sending the header
itself.

## Acceptance criteria

- A token minted by the `3cdf7b2` user-service with the same secret is accepted by every guarded
  route of the monolith; a token with `alg: none`, without `aud`, or expired is refused with the
  messages above.
- The existing tests of `internal/middleware`, `handlers/auth*`, `oidc*`, `oauth*` and `ldap*`
  pass unchanged apart from import paths.
- The Playwright sign-in through Keycloak works against the monolith ([108](108-backend-operations-and-testing.md)).

## Sources

`internal/middleware/auth.go`; `user-service/internal/handlers/auth.go`, `oidc.go`, `oauth.go`,
`ldap.go`, `settings.go`; `user-service/internal/db/seed.go`; `docs/SSO.md`, `docs/ARCHITECTURE.md`
("JWT").
