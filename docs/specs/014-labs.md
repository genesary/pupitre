# 014 - Labs: GitLab checks and local check

Features: F090 (lab module), F091 (policy check), F092 (GitLab step checks), F093 (local check),
F094 (check history), F096 (desktop app).

## Rendering

A `lab` module is shown in the lesson player ([012](012-lesson-player.md)): the markdown of the
instructions, then the checks. Useful module fields: `hasCheck`, `labUrl`, `checkProvider`
(`gitlab` or `local`), `checkType`, `checkParams`, `steps[] {title, checkType, checkParams}`.

`labUrl`, if present, becomes an « Ouvrir le lab » button (`target="_blank"`,
`rel="noopener noreferrer"`, https URL validated, see [003](003-security.md) S4).

## Lab with steps

- The markdown contains `<!-- check:N -->` markers. At each marker, a step block is inserted:
  « ✔ <step title> », a « Vérifier cette étape » button and a result area. The block is a React
  component placed where the marker is (not HTML injected into the markdown, see
  [003](003-security.md) S2).
- A marker with no matching step is ignored; a step with no marker is shown at the end.
- Checking a step:
  - `checkProvider === 'gitlab'`: `POST /api/courses/{slug}/modules/{index}/steps/{N}/check`;
  - `checkProvider === 'local'`: the local check (below).
- Result `{ allow, violations[] }`:
  - `allow` → « ✓ Étape validée ! »;
  - otherwise → « ✗ Pas encore » and the list of violations, **as text**;
  - error → the message, styled as an error. Over the lab check limit (`429`, [190](190-backend-defects.md)
    B9): « Trop de vérifications d'affilée : réessayez dans N s. », N from the API's message.
- Once every step is validated:
  - « ✓ Lab terminé ! Félicitations. » banner;
  - `POST /api/courses/{slug}/lessons/{moduleSlug}/complete` (an error is ignored: the lab may
    already be completed);
  - after 1.2 s, navigate to the next module, or the stored destination, or the course page.

## Lab with a single check

No steps but `hasCheck`: a bar « Une fois le lab terminé, revenez ici pour valider votre
travail. » and a « Vérifier mon travail » button (« Vérification… » during the call).

- `gitlab`: `POST /api/courses/{slug}/modules/{index}/check`.
- `local`: the local check.

## Local check (desktop app)

The `pupitre-desktop` desktop app (Tauri) embeds the frontend and exposes a Rust command
`local_check`.

- Detection: `'__TAURI__' in window || '__TAURI_INTERNALS__' in window`.
- **In Tauri**: `invoke('local_check', { checkType, params })` through `@tauri-apps/api/core`
  (dynamic import, kept out of the web bundle). Without `checkType`: `allow: false` with the
  violation « checkType manquant dans le module ». The result is then sent to
  `POST /api/courses/{slug}/modules/{index}/record` (without waiting for the response). The server
  stores it as **not verified** (issue genesary/pupitre#86).
- **In a browser**: « Ce lab se vérifie dans Pupitre. » and an « Ouvrir dans Pupitre → » link to
  `pupitre://courses/{slug}/modules/{index}`.

The local check lives in `apps/app/src/lib/local-check.ts`: `resolveCheck(meta, remoteCheck, record)`.
The desktop app only relies on the Tauri command (`local_check`) and the `pupitre://` deep link,
which are unchanged; `record` replaces the URL and token arguments of the Astro version, so the
token stays in the session module ([003](003-security.md) S5).

## Acceptance criteria

- A lab with three GitLab steps shows three blocks where the markers are; validating all three
  shows the banner, then moves to the next module.
- A violation containing `<b>` is shown as is, as text.
- In a browser, a `local` lab shows the `pupitre://` link without calling the API.
- In Tauri, a `local` lab calls `local_check`, then `record`.

## Sources

`frontend/src/pages/courses/[slug]/lessons/[index].astro` (lab part), `lib/local-check.ts`;
`course-service/internal/handlers/check.go`; `pupitre-desktop` repository.
