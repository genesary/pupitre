# SSO / OIDC — Configuration Guide

The web interface signs in through **one OIDC provider, Keycloak**: the login page shows a single
"Se connecter avec Keycloak" button when OIDC is enabled. If it is not, nobody can sign in from the
interface.

The API keeps the other routes of the original services, but **no page calls them**:

| Mechanism | Route | Interface |
|---|---|---|
| Dedicated OIDC | `/api/auth/oidc/authorize`, `/api/auth/oidc/callback` | ✅ Login page |
| Email and password | `POST /api/auth/login`, `POST /api/auth/register` | ❌ API only |
| LDAP / Active Directory | `POST /api/auth/ldap/login` | ❌ API only |
| `sso.providers` (GitHub, GitLab, …) | `/api/auth/oauth/{provider}/authorize`, `/api/auth/oauth/callback` | ❌ API only |

Since those routes stay reachable, close the ones you do not use in `/admin/settings`: the page warns
about `sso_local_login_enabled` and `registration_enabled` while they are still open.

> **Secrets are never hardcoded in the chart.** The OIDC client secret comes from a Kubernetes
> Secret: rendered into the release Secret from an inline value (dev), or referenced with
> `existingSecret` (production). It is mounted as a file and never appears in the ConfigMap.

---

## How it works

```
Browser → GET /api/auth/oidc/authorize → redirect to Keycloak
       ← Keycloak redirects to /auth/callback?code=...
       → POST /api/auth/oidc/callback {code, state}
       ← JWT token
```

The redirect URI registered with Keycloak is `{redirectBase}/auth/callback`.

---

## Profile sync

On every OIDC login the platform syncs the user's profile from the identity provider:

| Field | Behaviour |
|---|---|
| **Avatar** | Always updated from the provider (`picture` claim). Falls back to initials if the URL is unreachable. |
| **Bio** | Populated from the provider on first login only (`bio` / `description` / `about` claims). Once a user writes their own bio it is **never overwritten** by a subsequent login. |
| **Display name** | Used only when creating a new account (`name` → `preferred_username` → email). Not updated on subsequent logins to preserve user edits. |

The platform fetches claims from **both** the ID token and the
[UserInfo endpoint](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo). The UserInfo
endpoint takes priority.

---

## OIDC provider via Helm (`sso.oidc`)

The values are seeded into the platform settings on startup (the same settings the admin UI edits),
so OIDC works immediately after `helm install`.

```yaml
sso:
  enabled: true
  redirectBase: "https://your-app.com"
  oidc:
    enabled: true
    providerURL: "https://keycloak.company.com/realms/{realm}"  # OIDC discovery URL
    clientID: "pupitre"
    clientSecret: "yyy"          # dev: rendered into the release Secret
    scopes: "openid email profile groups"
    groupClaim: "groups"
    # Optional comma-separated groups authorized as platform admins.
    groupAdmins: "pupitre-admins"
    # issuerURL: ""              # split-horizon: set when discovery URL ≠ token issuer
    # browserBaseURL: ""         # split-horizon: rewrite internal URL for browser redirects
    # redirectBase: ""           # per-OIDC override (defaults to sso.redirectBase)
    # insecureSkipVerify: false  # custom CA or self-signed certificate, trusted environments only
```

**Production — source the client secret from a Secret you manage:**

```yaml
sso:
  oidc:
    enabled: true
    providerURL: "https://keycloak.company.com/realms/{realm}"
    clientID: "pupitre"
    existingSecret: keycloak-oidc       # Secret you manage
    existingSecretKey: OIDC_CLIENT_SECRET
```

The OIDC client secret is mounted as a file (`OIDC_CLIENT_SECRET_FILE`) and seeded into the database
on startup: it never appears in the pod environment or the ConfigMap.

**Behaviour notes:**

- When `sso.oidc.enabled=true`, the Helm values are **authoritative** and re-seeded on every pod
  start (GitOps). Editing these fields in the admin UI works but is reset on the next restart or
  upgrade. Leave `sso.oidc.enabled=false` to manage OIDC purely from the admin UI.
- Rotating the client secret requires a pod restart so the new value is re-seeded.
- When `groupAdmins` is non-empty, it is authoritative for OIDC users: every OIDC login synchronizes
  the stored role to `admin` for a matching group and `student` otherwise, managers included. Leave
  it empty when the platform has managers, and map the admin group in `/admin/groups` instead.
  Removing a user from an admin group takes effect on their next OIDC login; existing JWTs remain
  valid until they expire.

**Keycloak client setup** (Clients → Create):

| Field | Value |
|---|---|
| Client type | OpenID Connect |
| Client authentication | On (to get a secret) |
| Valid redirect URIs | `{redirectBase}/auth/callback` |
| Client scopes | ensure a `groups` mapper is added if you use group → role mapping |

For local development, `pnpm api:up` starts a Keycloak with a ready-made realm
(`apps/api/infra/keycloak/pupitre-realm.json`), and `apps/api/.env.example` holds the matching
`OIDC_*` variables.

---

## Admin UI OIDC (runtime)

The OIDC provider can be configured at runtime in `/admin/settings` (or bootstrapped at deploy time
via `sso.oidc`, which seeds these same keys):

| Setting key | Description |
|---|---|
| `oidc_enabled` | `true` / `false` |
| `oidc_provider_url` | OIDC discovery URL |
| `oidc_issuer_url` | Issuer override, when the discovery URL differs from the token issuer |
| `oidc_redirect_base` | Base URL of the redirect URI |
| `oidc_browser_base_url` | Override issuer base URL for browser redirects (split-horizon DNS) |
| `oidc_client_id` | Client ID |
| `oidc_client_secret` | Client Secret |
| `oidc_scopes` | Space-separated scopes (default: `openid email profile groups`) |
| `oidc_group_claim` | JWT claim containing group names (default: `groups`) |
| `oidc_group_admins` | Comma-separated groups authorized as platform admins |

---

## Routes without an interface

### `sso.providers`

The chart still accepts a list of OAuth2/OIDC providers, and the API serves their routes. Users
created this way get the default role: this flow never reads group claims. Each client secret is
injected as the `SSO_<ID>_CLIENT_SECRET` environment variable. See the comments of `sso.providers` in
the chart's `values.yaml`.

### LDAP / Active Directory

LDAP is built into the API and configured in `/admin/settings`, in a section folded by default:

| Setting key | Description | Example |
|---|---|---|
| `ldap_enabled` | Enable LDAP login | `true` |
| `ldap_server_url` | LDAP server URL | `ldap://openldap:389` or `ldaps://ad.company.com:636` |
| `ldap_bind_dn` | Service account DN (leave empty for anonymous bind) | `cn=svc-pupitre,ou=service,dc=company,dc=com` |
| `ldap_bind_password` | Service account password | `secret` |
| `ldap_user_base_dn` | Base DN for user search | `ou=users,dc=company,dc=com` |
| `ldap_user_filter` | Search filter — `%s` is replaced by the user's email | `(mail=%s)` |
| `ldap_group_base_dn` | Base DN for group search (leave empty to skip group sync) | `ou=groups,dc=company,dc=com` |
| `ldap_group_filter` | Group membership filter — `%s` is the user's DN | `(&#124;(member=%s)(uniqueMember=%s)(memberUid=%s))` |

The API binds with the service account, searches for the user by email, then binds again with the
user's DN and password. LDAP groups are synced as platform groups on every login; `/admin/groups`
assigns their roles.
