# 003 - Frontend security requirements

These requirements cover the findings of the 2026-10-20 audit that are fixed in the browser.
They apply to **every** page and take precedence over the functional specs in case of conflict.

## S1 - No HTML built by string concatenation

The current frontend builds almost all its pages with `innerHTML` and string templates. This led
to XSS findings C2 (username in the admin pages) and C4 (avatar on the leaderboard and profiles).

- Every text coming from the API (username, e-mail, bio, titles, descriptions, group names,
  error messages, lab violations…) is rendered through JSX, which escapes it.
- `dangerouslySetInnerHTML` is **forbidden**, except in the `<SafeHtml>` component, which only
  receives HTML that went through `sanitizeHtml()` (S2). An ESLint rule (`react/no-danger`, with
  an exception for this single file) enforces it.
- No inline event attribute (`onclick="…"`, `onerror="…"`).
- Dynamic URLs in `href` and `src` go through `safeUrl()` (S4).

## S2 - Course markdown and HTML

A module's content can come from a third-party git repository (finding C3).

- Pipeline: markdown → admonitions → patterns → `marked` → **`DOMPurify.sanitize`** → DOM.
  Sanitization runs **after** the patterns are inserted, on the final HTML.
- DOMPurify configuration:
  - default tags and attributes (`html` profile), plus `class` and `id`;
  - forbidden: `script`, `style`, `iframe`, `object`, `embed`, `form`, `button`, `textarea`,
    `select`, and every `on*` attribute;
  - `input` is kept only for GFM task-list checkboxes: a hook removes any `input` that is not
    `type="checkbox"` and forces `disabled`;
  - `href` and `src` limited to `http:`, `https:`, `mailto:`, relative paths and anchors;
  - external links: `target="_blank"` and `rel="noopener noreferrer"` added by a hook.
- Interactive blocks (the « Copier » button, the lab steps `<!-- check:N -->`, mermaid diagrams)
  are **React components** mounted at marked places, not HTML injected into the markdown string.
- Pattern HTML and CSS are admin content, but they are sanitized like the rest. The `js` field of
  a pattern is **never executed**.
- Pattern CSS is injected in a single `<style>` element per page.

## S3 - Avatar

Finding C4. The backend accepts any string in `avatarUrl`.

- Rendering, in every component that shows an avatar (top bar, profile, public profile,
  leaderboard, lists):
  - if `avatarUrl` passes `safeUrl()` and uses `https:`: an `<img>` whose `src` is that value,
    with `referrerPolicy="no-referrer"` and a fallback to the initial if loading fails;
  - otherwise, if it is a single emoji
    (`/^\p{Extended_Pictographic}(️|‍\p{Extended_Pictographic})*$/u`): shown as
    **text**;
  - otherwise: the first letter of the username. An arbitrary value is never interpreted as HTML.
- Input (see [021](021-profile.md)): only the emojis of earned badges are offered. Entering an
  external URL is not kept until the backend downloads the image server side, as requested by
  issue genesary/pupitre#187.

## S4 - URLs and redirects

- `safeRedirect()` for every internal redirect driven by a parameter (see
  [001](001-keycloak-authentication.md), issue genesary/pupitre#194).
- `safeUrl()` for every dynamic `href` or `src`: `http:`, `https:` or a relative path; never
  `javascript:`, `data:` or `vbscript:`.
- URLs received from the API to leave the application (Keycloak authorization URL, a lab's
  `labUrl`) are accepted only with `https:` (and `http:` in development).

## S5 - Session

- The token is available only to the session module ([001](001-keycloak-authentication.md)).
- It is never written in a URL, a log or an error message.
- It is sent only to `/api` (same origin). Calls to another origin carry no `Authorization`
  header.

## S6 - Headers and CSP

The production reverse proxy (or static file server) sends:

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';
  img-src 'self' https: data:; font-src 'self'; connect-src 'self';
  frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
```

- `style-src 'unsafe-inline'` is still needed for pattern CSS and mermaid styles. Everything else
  works without inline script.
- Fonts (Hanken Grotesk, Caveat, JetBrains Mono) are served by the application (`@fontsource/*`)
  instead of Google Fonts, so the CSP has no external origin.
- The theme anti-flash script ([005](005-ui-theme-navigation.md)) is a file, not an inline script.

## S7 - Dependencies

- `pnpm audit` runs in CI and passes with no high or critical vulnerability.
- Versions are pinned by `pnpm-lock.yaml`. Renovate may propose updates.

## Reminder: what remains on the backend

These findings are not fixed in the frontend, but the frontend must not work around them:

| Audit | Server-side fix |
| --- | --- |
| C1 | Refuse to start with a default JWT or internal secret |
| C4 | Validate `avatarUrl` server side (emoji of an earned badge or `/uploads/…`) |
| H2 | Restrict managers to their scope (members, exports, attendance) |
| H3 | Token revocation (short-lived token, `token_version`) |
| H4 | Forward only known headers (the Astro proxy goes away with pupitre2, but the reverse proxy must not forward `X-Internal-Secret` nor trust a client-supplied `X-Forwarded-For`) |
| H7 | Disable local registration and local login |
| M1 | Filter private courses out of `/api/batch/courses` |

## Acceptance criteria

- A username `<img src=x onerror=alert(1)>` is shown as is, as text, on every admin and manager
  page.
- An avatar `<img src=x onerror=alert(1)>` or `https://x" onerror="alert(1)` runs nothing on
  `/leaderboard`, `/users/:id`, `/profile` or in the top bar.
- A markdown module containing `<script>`, `<img onerror>`, `<a href="javascript:…">` or
  `<iframe>` is displayed with no execution and no frame.
- A pattern with a non-empty `js` field triggers no execution.
- The CSP above causes no console error on the main pages.
- A Vitest test covers `safeRedirect`, `safeUrl`, `sanitizeHtml` and the avatar rendering with
  these payloads.
