# 001 - OIDC authentication through Keycloak

Features: F002 (session token), F003 (browser session and guards), F023-F028 (sign-in, SSO-only
mode, landing page, logout, OIDC), F034 (current user).

## Decision

The platform signs in **only with OIDC through Keycloak**. The new frontend implements no
e-mail / password form, no registration page and no GitHub or LDAP button. Roles come from
Keycloak (groups mapped by the backend, see [044](044-admin-groups.md) and the
`oidc_group_admins` setting in [046](046-admin-settings.md)).

The backend keeps local authentication enabled by default. `registration_enabled`,
`sso_local_login_enabled` and `profile_allow_username_change` must be set to `false` (audit
finding H7): hiding the form in the frontend does not close the `POST /api/auth/register` and
`POST /api/auth/login` routes.

## Sign-in flow

1. **`/login`**: the page reads `GET /api/settings/public`.
   - If `oidc_enabled === "true"`: a single button « Se connecter avec Keycloak ».
   - Otherwise: the message « La connexion n'est pas configurée. Contactez un administrateur. »
   - If a valid session already exists, the page redirects to the landing page (see below).
2. **Click on the button**:
   - store the requested destination (the `redirect` parameter, **validated**, see "Redirect
     after sign-in") in `sessionStorage`;
   - call `GET /api/auth/oidc/authorize`, which returns `{ url, state }`;
   - store `state` in `sessionStorage` (`pupitre.oidc.state`);
   - navigate to `url` (the Keycloak sign-in page).
   - During the call, the button is disabled and shows a spinner. On error, show the returned
     message in an alert and enable the button again.
3. **`/auth/callback`**: Keycloak comes back with `code` and `state`, or with `error`.
   - This route **must not change**: the redirect URI registered in Keycloak is
     `${oidc_redirect_base}/auth/callback`.
   - `error=access_denied` → « Accès refusé : vous avez annulé la connexion. »
   - Any other `error` → « Erreur de connexion : <code> ».
   - Missing `code` or `state` → « Paramètres de connexion manquants. Veuillez réessayer. »
   - `state` different from the stored value → refuse, with « Session de connexion invalide,
     recommencez. » (login CSRF protection, audit finding M3).
   - Otherwise: `POST /api/auth/oidc/callback` with `{ code, state }` → `{ token, user }`. Open
     the session, delete the `pupitre.oidc.*` values, then redirect.
   - During the exchange: « Finalisation de la connexion… ». On failure: the message and a
     « Retour à la connexion » button.

The current frontend decodes `state` to choose between `/auth/oauth/callback` and
`/auth/oidc/callback`. With Keycloak only, it always calls `/auth/oidc/callback`.

## Redirect after sign-in

- Default destination: `/admin` for an admin, `/courses` otherwise. A manager also lands on
  `/courses` and reaches their space from the top bar.
- The `redirect` parameter (set by the guards of [002](002-roles-and-guards.md)) is accepted only
  if it is a **relative path**: it starts with a single `/`, with no `//`, no `\` and no scheme.
  Anything else is ignored in favour of the default destination. This fixes issue
  genesary/pupitre#194 (open redirect and `javascript:` URL).
- A valid `redirect` applies to every role, admins included.

```ts
export function safeRedirect(raw: string | null, fallback: string): string {
  if (!raw || !/^\/(?![/\\])/.test(raw)) return fallback;
  const url = new URL(raw, window.location.origin);
  return url.origin === window.location.origin ? url.pathname + url.search + url.hash : fallback;
}
```

## Session

- The backend returns a JWT (HS256, valid 8 hours by default) that every call sends in
  `Authorization: Bearer <token>`.
- The token and the user are handled by **a single module** (`packages/core/src/lib/session.ts`): no page
  reads the storage directly.
- Interim storage: `sessionStorage` (the session ends with the tab and is not shared between
  tabs). The target is an `HttpOnly` cookie set by the backend, which needs a server change
  (audit decision 2). The session module must allow that change without touching the pages.
- On start-up, if a token exists:
  - read `exp` from the token (decoding only, no verification). If `exp` is in the past, end the
    session;
  - call `GET /api/auth/me` to refresh the user (role, avatar, username). A 401 ends the session.
- A timer ends the session when the token expires and redirects to
  `/login?redirect=<current page>`.
- Any **401** on an authenticated call ends the session and redirects to
  `/login?redirect=<current page>`, with the toast « Session expirée, reconnectez-vous. »
  (today some pages redirect to `/` and others to `/login`).

## Logout

- The « Déconnexion » button (top bar and side menu) calls `POST /api/auth/logout`, which ends
  every session of the user on the server ([190](190-backend-defects.md) B7), then ends the local
  session, clears the TanStack Query cache and redirects to `/login`. The browser forgets the
  token whatever the server answers.
- The Keycloak session is not closed: clicking « Se connecter » again signs in without asking for
  the password. A redirect to the Keycloak `end_session_endpoint` **may** be added once the API
  exposes its URL.

## Current user

`GET /api/auth/me` → `UserPublic`:

```ts
type UserPublic = {
  id: string; username: string; email: string;
  role: 'student' | 'manager' | 'admin';
  avatarUrl: string | null; bio: string | null;
  isActive: boolean; authProvider: string; createdAt: string;
};
```

The current frontend type (`role: "admin" | "student"`) is missing `manager`.

## Removed routes

| Current route | Behaviour in pupitre2 |
| --- | --- |
| `/register` | Redirects to `/login` |
| `/dashboard`, `/` | Redirect to `/courses` |
| « Email » and « LDAP » tabs of `/login` | Removed |

## Acceptance criteria

- With `oidc_enabled=true`, `/login` shows a single button. Clicking it leads to Keycloak, then
  to `/courses` (or `/admin` for an admin) once signed in.
- `/login?redirect=https://example.com` then sign-in → the user lands on `/courses`, never on
  `example.com`.
- `/login?redirect=javascript:alert(1)` then sign-in → no script runs.
- `/login?redirect=/courses/kubernetes-basics` → the user lands on that page, admins included.
- A `state` modified in the callback URL is refused without calling the backend.
- A token that is already expired when the application opens triggers no authenticated call and
  leads to `/login`.

## Sources

`frontend/src/pages/login.astro`, `pages/auth/callback.astro`, `pages/register.astro`,
`layouts/App.astro` (logout), `lib/api.ts` (401 handling);
`user-service/internal/handlers/oidc.go`, `auth.go` (`Me`).
