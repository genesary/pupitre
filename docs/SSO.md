# SSO / OAuth — Configuration Guide

The platform supports three authentication mechanisms in addition to local email/password:

- **`sso.providers`** (Helm) — OAuth2/OIDC providers configured at deploy time (GitHub, GitLab, Google, etc.)
- **OIDC** — a single OIDC provider (e.g. Keycloak). Can be bootstrapped at deploy time via **`sso.oidc`** (Helm) and/or configured at runtime in `/admin/settings`.
- **LDAP / Active Directory** — authenticate against any LDAP directory, configured at runtime in `/admin/settings`

All three can be active simultaneously.

> **Secrets are never hardcoded in the chart.** For both `sso.oidc` and `sso.providers`,
> client secrets are sourced from a Kubernetes Secret — either rendered into the
> release Secret from an inline value (dev) or referenced from an `existingSecret`
> you manage with Vault / External Secrets / SOPS (production). Provider secrets are
> injected as env vars and the OIDC secret is mounted as a file; neither appears in
> the ConfigMap.

---

## Two OIDC paths — which to use

OIDC can be configured **two different ways**, backed by **two separate code paths**
with **different feature sets**. This matters: an OIDC provider configured in the
`sso.providers` list does **not** get group → role mapping or split-horizon support.

| | `sso.providers` list | Dedicated OIDC (`sso.oidc` / admin UI `oidc_*`) |
|---|---|---|
| **How many** | Multiple, simultaneously | Exactly **one** |
| **Configured via** | Helm `sso.providers` (deploy time, config file) | Helm `sso.oidc` **or** `/admin/settings` (runtime) |
| **Routes** | `/api/auth/oauth/{id}/authorize` · `/api/auth/oauth/callback` | `/api/auth/oidc/authorize` · `/api/auth/oidc/callback` |
| **Scopes** | Fixed: `openid email profile` | Configurable (`oidc_scopes`, default incl. `groups`) |
| **Group → role mapping** | ❌ **Not supported** | ✅ via `oidc_group_claim` |
| **Split-horizon** (internal discovery URL ≠ public issuer) | ❌ Not supported | ✅ via `issuerUrl` / `browser_base_url` |
| **`authProvider` stored** | the provider `id` (e.g. `keycloak`) | always `oidc` |

**Rule of thumb:**

- Need **several** IdPs at once, or simple social login (GitHub/GitLab/Google) where
  everyone gets the default role → use **`sso.providers`**.
- Need **group-based roles** (map IdP groups to `admin`/`student`) or you're behind
  split-horizon DNS (typical in-cluster Keycloak) → use the **dedicated OIDC** path
  (`sso.oidc`). You can only have one, but it's the full-featured one.

> ⚠️ Do **not** expect group → role mapping to work for IdPs listed under
> `sso.providers` (Keycloak, Authentik, Azure AD, Okta, Auth0, …). Even though the
> login succeeds and `groups` may be in the token, that flow never reads the claim.
> For role mapping, configure the IdP through `sso.oidc` instead.

---

## Profile sync

On every SSO login the platform automatically syncs the user's profile from the identity provider:

| Field | Behaviour |
|---|---|
| **Avatar** | Always updated from the provider (`picture` claim / GitHub `avatarUrl`). Displayed on the profile page with a graceful fallback to initials if the URL is unreachable. |
| **Bio** | Populated from the provider on first login only (`bio` / `description` / `about` claims). Once a user writes their own bio it is **never overwritten** by a subsequent login. |
| **Display name** | Used only when creating a new account (`name` → `preferred_username` → email). Not updated on subsequent logins to preserve user edits. |

For OIDC providers, the platform fetches claims from **both** the ID token and the [UserInfo endpoint](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo) — the UserInfo endpoint takes priority, which gives access to richer or more up-to-date attributes.

**Profile page** shows an SSO provider badge (GitHub, GitLab, Google, etc.) and hides the password-change section for SSO-managed accounts.

---

## How it works (`sso.providers` flow)

> This section describes the **`sso.providers`** (multi-provider) flow. The
> dedicated single-provider OIDC flow is described under
> [OIDC provider via Helm](#oidc-provider-via-helm-ssooidc) and
> [Admin UI OIDC](#admin-ui-oidc-runtime). See
> [Two OIDC paths](#two-oidc-paths--which-to-use) for the difference.

```
Browser → GET /api/auth/oauth/{id}/authorize  → redirect to provider
       ←   … + Set-Cookie: pupitre_pkce (HttpOnly, never sent to the provider)
       ← provider redirects to /auth/callback?code=...
       → POST /api/auth/oauth/callback {code, state} + pupitre_pkce cookie
       ← JWT token
```

The backend reads `sso.providers` from the Helm configmap. For each provider with a non-empty `clientId`, a button appears on the login page.

> **No group → role mapping in this flow.** Users created through `sso.providers`
> are added to the default group and get the default role; IdP group claims are
> **not** read. If you need group-based roles, configure that IdP through
> [`sso.oidc`](#oidc-provider-via-helm-ssooidc) instead.

**Two internal flows:**

| Provider `id` | Flow |
|---|---|
| `github` | OAuth2 direct (GitHub has no OIDC discovery) |
| anything else | OIDC discovery via `issuerUrl` |

The `id` value is stored in the database as `authProvider` for each user — choose a stable, meaningful value and don't change it once users have signed in.

---

## Authorization code protection (PKCE)

Both flows implement PKCE ([RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636))
with the `S256` challenge method. Nothing to configure — it is always on.

**How it works**

1. `/authorize` generates a random `code_verifier`.
2. The verifier goes to the browser in a `pupitre_pkce` cookie: `HttpOnly`,
   `SameSite=Lax`, `Path=/api/auth`, 10-minute lifetime, and `Secure` whenever
   the redirect base is `https://`.
3. Only the derived `code_challenge` is sent to the identity provider, and only
   its `S256` hash is committed inside the signed `state` token.
4. `/callback` reads the cookie back, checks it hashes to the challenge the
   `state` token was issued for, clears the cookie, and sends the verifier as
   `code_verifier` in the token exchange.

**Why the verifier lives in a cookie and not in `state`**

`state` is signed but not encrypted, and it travels to the identity provider and
back through the same front channel as the authorization code. A verifier carried
inside it would be captured by exactly the interceptor PKCE exists to defeat.
The cookie never leaves our own origin, so an attacker holding a stolen `code`
and `state` cannot complete the exchange.

That check is enforced **locally, before any code is exchanged**, not delegated to
the provider: GitHub's OAuth apps ignore `code_challenge` entirely, so a callback
that cannot present the matching verifier is rejected here with `401` regardless of
what the provider would have accepted.

**Deployment requirement**

The cookie is what binds a callback to the browser that started the login, so the
frontend and `/api` must be reachable on the **same origin** — which is what the
chart's single-host, path-routed Ingress gives you. Serving the API from a
different origin than the frontend will drop the cookie on the callback request
and every SSO login will fail with `401 Invalid or expired OAuth state` (or
`… OIDC state` on the dedicated OIDC route).

One-off note on upgrades: a login started on a pre-PKCE release and finished
after the rollout fails once with the same error, because the `/authorize` half
issued no cookie and the `/callback` half now requires one. Retrying the login
succeeds. The window is at most the 10-minute `state` lifetime.

---

## Helm configuration

```yaml
sso:
  enabled: false          # set to true to enable the providers list
  redirectBase: ""        # public URL of the frontend — defaults to ingress host
  oidc:
    enabled: false        # single OIDC provider (e.g. Keycloak) — see below
  providers: []           # OAuth2/OIDC providers list (GitHub, GitLab, …)
```

`redirectBase` must match the **Callback URL** registered with each provider.
The actual redirect URI sent to providers is `{redirectBase}/auth/callback`.

### Sourcing provider client secrets from a Secret

Each entry in `sso.providers` accepts its `clientSecret` inline (rendered into the
release Secret, fine for dev) **or** a reference to an existing Secret you manage:

```yaml
sso:
  enabled: true
  providers:
    - id: gitlab
      name: GitLab
      clientId: "xxx"
      issuerUrl: "https://gitlab.com"
      existingSecret: gitlab-oauth      # Secret you manage (Vault / ESO / SOPS)
      existingSecretKey: clientSecret  # key inside that Secret (default: clientSecret)
```

The client secret is injected into user-service as the `SSO_<ID>_CLIENT_SECRET`
env var and is **never** written to the ConfigMap.

---

## OIDC provider via Helm (`sso.oidc`)

This is the recommended path for a single enterprise IdP such as **Keycloak**. The
values are seeded into the platform settings on startup (the same settings the admin
UI edits), so OIDC works immediately after `helm install` — no manual configuration.

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

The OIDC client secret is mounted as a file (`OIDC_CLIENT_SECRET_FILE`) and seeded
into the database on startup — it never appears in the pod environment or the
ConfigMap.

**Behaviour notes:**

- When `sso.oidc.enabled=true`, the Helm values are **authoritative** and re-seeded
  on every pod start (GitOps). Editing these fields in the admin UI works but is
  reset on the next restart/upgrade. Leave `sso.oidc.enabled=false` to manage OIDC
  purely from the admin UI.
- Rotating the client secret requires a pod restart so the new value is re-seeded.
- When `groupAdmins` is non-empty, it is authoritative for OIDC users: every
  OIDC login synchronizes the stored role to `admin` for a matching group and
  `student` otherwise. Removing a user from an admin group takes effect on
  their next OIDC login; existing JWTs remain valid until they expire.

**Keycloak client setup** (Clients → Create):

| Field | Value |
|---|---|
| Client type | OpenID Connect |
| Client authentication | On (to get a secret) |
| Valid redirect URIs | `{redirectBase}/auth/callback` |
| Client scopes | ensure a `groups` mapper is added if you use group → role mapping |

---

## Supported providers

### GitHub

GitHub does not support OIDC. The backend uses the GitHub OAuth2 API directly.

```yaml
sso:
  enabled: true
  redirectBase: "https://your-app.com"
  providers:
    - id: github
      name: GitHub
      clientId: "Iv23li..."
      clientSecret: "abc123..."
```

**GitHub App setup** (`github.com/settings/apps` → New GitHub App):

| Field | Value |
|---|---|
| Homepage URL | your frontend URL |
| Callback URL | `{redirectBase}/auth/callback` |
| Webhook | disabled |
| Account permissions → Email addresses | Read-only |
| Where can it be installed | Any account |

After creation: copy **Client ID** and generate a **Client Secret**.

---

### GitLab (SaaS or self-hosted)

Uses OIDC discovery. `issuerUrl` defaults to `https://gitlab.com` when omitted —
only set it for self-hosted instances.

```yaml
providers:
  # GitLab SaaS — issuerUrl optional
  - id: gitlab
    name: GitLab
    clientId: "xxx"
    clientSecret: "yyy"

  # GitLab self-hosted — set issuerUrl to your instance
  - id: gitlab
    name: GitLab
    clientId: "xxx"
    clientSecret: "yyy"
    issuerUrl: "https://gitlab.company.com"
```

**GitLab setup** (User Settings → Applications):

| Field | Value |
|---|---|
| Redirect URI | `{redirectBase}/auth/callback` |
| Scopes | `openid`, `email`, `profile` |

---

### Google

```yaml
providers:
  # issuerUrl optional for Google, defaults to https://accounts.google.com
  - id: google
    name: Google
    clientId: "xxx.apps.googleusercontent.com"
    clientSecret: "yyy"
```

**Google Cloud Console** (APIs & Services → Credentials → OAuth 2.0 Client ID):

| Field | Value |
|---|---|
| Application type | Web application |
| Authorised redirect URIs | `{redirectBase}/auth/callback` |

---

### Microsoft / Azure AD

```yaml
providers:
  - id: microsoft
    name: Microsoft
    clientId: "xxx"
    clientSecret: "yyy"
    # Organisation accounts only:
    issuerUrl: "https://login.microsoftonline.com/{tenant-id}/v2.0"
    # Personal + org accounts:
    # issuerUrl: "https://login.microsoftonline.com/common/v2.0"
```

**Azure Portal** (App registrations → New registration):

| Field | Value |
|---|---|
| Redirect URI | `{redirectBase}/auth/callback` |
| Supported account types | as needed |

Add a client secret in **Certificates & secrets**.

---

### Authentik

```yaml
providers:
  - id: authentik
    name: "Company SSO"
    clientId: "xxx"
    clientSecret: "yyy"
    issuerUrl: "https://authentik.company.com/application/o/{app-slug}/"
```

The `issuerUrl` is shown in the Authentik provider detail page.
Required scopes: `openid`, `email`, `profile`.

> For group → role mapping with Authentik, use the dedicated
> [`sso.oidc`](#oidc-provider-via-helm-ssooidc) path — group claims are not read in
> the `sso.providers` flow.

---

### Keycloak

> 💡 For Keycloak with **group → role mapping** or **split-horizon DNS** (the usual
> in-cluster case), prefer the dedicated [`sso.oidc`](#oidc-provider-via-helm-ssooidc)
> path instead — the `sso.providers` entry below logs users in but ignores group
> claims.

```yaml
providers:
  - id: keycloak
    name: "Company SSO"
    clientId: "pupitre"
    clientSecret: "yyy"
    issuerUrl: "https://keycloak.company.com/realms/{realm-name}"
```

**Keycloak admin** (Clients → Create):

| Field | Value |
|---|---|
| Client type | OpenID Connect |
| Valid redirect URIs | `{redirectBase}/auth/callback` |
| Client authentication | On (to get a secret) |

---

### Okta

```yaml
providers:
  - id: okta
    name: Okta
    clientId: "xxx"
    clientSecret: "yyy"
    issuerUrl: "https://your-domain.okta.com"
```

---

### Auth0

```yaml
providers:
  - id: auth0
    name: Auth0
    clientId: "xxx"
    clientSecret: "yyy"
    issuerUrl: "https://your-domain.auth0.com"
```

---

### Any OIDC-compatible provider

Any provider that exposes a standard OIDC discovery endpoint at
`{issuerUrl}/.well-known/openid-configuration` works out of the box:

```yaml
providers:
  - id: my-sso
    name: "My SSO"
    clientId: "xxx"
    clientSecret: "yyy"
    issuerUrl: "https://sso.company.com"
```

---

## Multiple providers

You can list as many providers as needed — each gets its own button on the login page.
This is the **only** way to run several OIDC providers at once; the dedicated
[`sso.oidc`](#oidc-provider-via-helm-ssooidc) path supports a single provider but adds
group → role mapping and split-horizon support
([comparison](#two-oidc-paths--which-to-use)).

```yaml
sso:
  enabled: true
  redirectBase: "https://your-app.com"
  providers:
    - id: github
      name: GitHub
      clientId: "..."
      clientSecret: "..."
    - id: gitlab
      name: GitLab
      clientId: "..."
      clientSecret: "..."
      issuerUrl: "https://gitlab.com"
    - id: google
      name: Google
      clientId: "..."
      clientSecret: "..."
      issuerUrl: "https://accounts.google.com"
```

---

## Adding a provider icon (frontend)

The login page renders an SVG icon for each provider. Icons for `github` and `gitlab`
are built in. For any other provider, the fallback is `🔑`.

To add an icon, edit `frontend/src/routes/login/+page.svelte` and add an entry to the
`providerIcon` map (around line 88):

```typescript
const providerIcon: Record<string, string> = {
  gitlab: `<svg ...>...</svg>`,
  github: `<svg ...>...</svg>`,

  // Add your provider here — inline SVG, class="w-5 h-5"
  google: `<svg viewBox="0 0 24 24" class="w-5 h-5">
    <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
    <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
    <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/>
    <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
  </svg>`,

  microsoft: `<svg viewBox="0 0 24 24" class="w-5 h-5">
    <path fill="#F25022" d="M1 1h10v10H1z"/>
    <path fill="#00A4EF" d="M13 1h10v10H13z"/>
    <path fill="#7FBA00" d="M1 13h10v10H1z"/>
    <path fill="#FFB900" d="M13 13h10v10H13z"/>
  </svg>`,

  okta: `<svg viewBox="0 0 24 24" class="w-5 h-5" fill="#007DC1">
    <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm0 18a6 6 0 110-12 6 6 0 010 12z"/>
  </svg>`,
};
```

**Rules:**

- Use `class="w-5 h-5"` on the `<svg>` to match the button size
- The key must exactly match the `id` used in `sso.providers`
- Inline SVG only — no external images (CSP)
- If no entry exists for a provider `id`, the fallback `🔑` is shown automatically

---

## Admin UI OIDC (runtime)

One OIDC provider can be configured at runtime in `/admin/settings` (or bootstrapped
at deploy time via [`sso.oidc`](#oidc-provider-via-helm-ssooidc), which seeds these
same keys):

| Setting key | Description |
|---|---|
| `oidc_enabled` | `true` / `false` |
| `oidc_provider_url` | OIDC issuer URL |
| `oidc_client_id` | Client ID |
| `oidc_client_secret` | Client Secret |
| `oidc_scopes` | Space-separated scopes (default: `openid email profile groups`) |
| `oidc_group_claim` | JWT claim containing group names (default: `groups`) |
| `oidc_browser_base_url` | Override issuer base URL for browser redirects (split-horizon DNS) |

When enabled, a **"Continue with SSO (OIDC)"** button appears on the login page.
This is useful for an enterprise IdP (Authentik, Keycloak, Azure AD) managed by an admin
without needing a Helm upgrade.

---

## LDAP / Active Directory (runtime, no Helm)

LDAP authentication is built into user-service — no separate microservice required.
Configure it at runtime in `/admin/settings`:

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

When enabled, a **LDAP** login option is shown on the login page alongside the email/password form.

### How it works

1. User enters their email and password
2. The service binds with the service account (if configured), then searches for the user by email using `ldap_user_filter`
3. The service re-binds with the found user's DN and the provided password to authenticate
4. If `ldap_group_base_dn` is set, the user's groups are fetched using `ldap_group_filter` and synced to the platform (group → role mappings apply)
5. A JWT is issued and the user is redirected

### OpenLDAP example

```
ldap_server_url       = ldap://openldap.infra.svc.cluster.local:389
ldap_bind_dn          = cn=admin,dc=company,dc=com
ldap_bind_password    = adminpassword
ldap_user_base_dn     = ou=users,dc=company,dc=com
ldap_user_filter      = (mail=%s)
ldap_group_base_dn    = ou=groups,dc=company,dc=com
ldap_group_filter     = (|(member=%s)(uniqueMember=%s)(memberUid=%s))
```

### Active Directory example

```
ldap_server_url       = ldaps://ad.company.com:636
ldap_bind_dn          = CN=svc-pupitre,OU=Service Accounts,DC=company,DC=com
ldap_bind_password    = secret
ldap_user_base_dn     = OU=Users,DC=company,DC=com
ldap_user_filter      = (userPrincipalName=%s)
ldap_group_base_dn    = OU=Groups,DC=company,DC=com
ldap_group_filter     = (member=%s)
```

### Authentik LDAP outpost

Authentik can expose an LDAP outpost that proxies your Authentik users and groups:

```
ldap_server_url       = ldap://authentik-ldap.authentik.svc.cluster.local:389
ldap_bind_dn          = cn=ldapservice,ou=serviceaccounts,dc=ldap,dc=goauthentik,dc=io
ldap_bind_password    = <token from Authentik outpost>
ldap_user_base_dn     = dc=ldap,dc=goauthentik,dc=io
ldap_user_filter      = (mail=%s)
ldap_group_base_dn    = ou=groups,dc=ldap,dc=goauthentik,dc=io
ldap_group_filter     = (|(member=%s)(uniqueMember=%s))
```

Create the outpost in Authentik: **Directory → Federation & Social login → LDAP outpost** (or via the Outposts menu). The bind DN and password come from the outpost's service account token.

### Group → role mapping

LDAP groups are synced as platform groups on every login. Use `/admin/groups` to assign a role to a group:

| LDAP group CN | Platform role |
|---|---|
| `pupitre-admins` | `admin` |
| `pupitre-students` | `student` |

If a user belongs to multiple groups, the highest role wins (`admin` > `student`).
