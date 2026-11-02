# 007 - Guided tour

Features: F186 (guided tour), F187 (restart the tour), F188 (role-dependent steps), F189 (first
lesson tip).

These features are **new**: neither the Astro frontend of `genesary/pupitre` nor the
`ebuildy/pupitre2` reference has a tour. They are additions of pupitre2
([090](090-gaps-and-defects.md)).

## Goal

A signed-in user who discovers the platform is guided through it by **the bird of the logo**
(`apps/app/public/pioupitre.png`, [005](005-ui-theme-navigation.md)), acting as a coach. The bird
has no name on screen: the file is called `pioupitre`, the platform is « pupitre ».
The bird flies from one element to the next, across the pages of the learner app, and explains
in a speech bubble what each part is for: where to find a course, what a course page holds, where
the courses, paths, groups, progress, badges, XP, skills and agenda are.

The tour shows the interface; it does not replace the course content and it does not change any
data: it never enrolls the user, never starts a lesson, never answers a quiz. How a lesson works
is explained by a short tip the first time the user opens one (see "First lesson tip").

## Scope

- The **learner app** only (`apps/app`). The admin app (`/admin`, `/manager`) has no tour: its
  entry points are presented by the last steps of the learner tour (see "Role-dependent steps").
- Signed-in users only. A visitor on the public catalogue is not offered the tour.
- **No backend change** (AGENTS.md, "Constraints"): no new route, no new column. The tour state
  is kept in the browser (see "Persistence").
- No new dependency: the tour is a few React components and a small stylesheet in pupitre's tokens.
  Libraries such as `react-joyride`, `driver.js` or `shepherd.js` are not used: they bring their
  own theme, which would go against the pupitre design ([005](005-ui-theme-navigation.md),
  "Visual design"), and add a dependency for little code ([003](003-security.md) S7).

## Starting the tour

### First visit

After sign-in, on the first page of the learner app that has a `Layout` (not `/login`,
`/auth/callback`, a lesson or a quiz), if the tour has never been offered to this user in this
browser (see "Persistence"), the bird appears at the bottom right with a welcome bubble:

> **Bonjour {username} !** Bienvenue sur pupitre ! Je vais vous faire visiter la plateforme en
> quelques étapes.

Buttons: « C'est parti ! » (primary), « Plus tard » (ghost). A close button (`aria-label`
« Fermer la visite ») does the same as « Plus tard ».

- « C'est parti ! » starts the tour at step 1.
- « Plus tard » closes the bubble and shows the toast « Vous pourrez relancer la visite depuis le
  menu. ».
- In both cases the offer is recorded and is not shown automatically again.
- `{username}` is rendered as JSX text, never as HTML ([003](003-security.md) S1).

The offer is not shown on a lesson or quiz page, so that it never covers content the user came to
read. It waits for the next eligible page.

### Restarting (F187)

- The side menu gets an entry « 🐤 Visite guidée », in the last block, before the theme toggle
  ([005](005-ui-theme-navigation.md), "Side menu"). It closes the menu and starts the tour at
  step 1, from any page. Its label is written in `SideMenu`: `@pupitre/core` cannot import the
  app's messages.
- The entry is shown to signed-in users only.

## Steps

Each step has an id, a page, an optional target and a text. The target is an element marked with a
`data-tour="<id>"` attribute; selectors on classes or text are not used, so that a restyling does
not break the tour.

| # | Id | Page | Target (`data-tour`) | Title | Text |
| --- | --- | --- | --- | --- | --- |
| 1 | `catalogue` | `/courses` | `nav-catalogue` (top bar link) | Le catalogue | Tous les cours de la plateforme sont ici. C'est votre point de départ. |
| 2 | `search` | `/courses` | `catalogue-search` | Trouver un cours | Cherchez par titre, ou filtrez par catégorie et par niveau. |
| 3 | `course-card` | `/courses` | `course-card` (first card) | Un cours | Chaque carte est un cours. Ouvrons celui-ci ensemble. |
| 4 | `continue` | `/courses` | `continue-hero` | Reprendre | pupitre garde votre place : reprenez la dernière leçon en un clic. |
| 5 | `course-page` | `/courses/:slug` | page title | La page du cours | Sa description, son niveau, ses compétences et le badge à gagner. |
| 6 | `course-modules` | `/courses/:slug` | `course-modules` (module list) | Le programme | Les modules du cours, à suivre dans l'ordre. Certains se débloquent quand le précédent est terminé. |
| 7 | `course-action` | `/courses/:slug` | `course-action` (enrollment area) | S'inscrire | Inscrivez-vous ici pour commencer. Ensuite, ce bloc affiche votre progression et l'XP du cours. |
| 8 | `my-courses` | `/my-courses` | `nav-my-courses` (top bar link) | Mes cours | Les cours auxquels vous êtes inscrit, avec votre progression. |
| 9 | `menu` | current page | `nav-menu` (menu button) | Le menu | Tout le reste est rangé ici : parcours, groupes, badges, XP, compétences, agenda. |
| 10 | `paths` | `/my-paths` | page title | Mes parcours | Des cours enchaînés dans un ordre pensé pour vous. |
| 11 | `groups` | `/my-groups` | page title | Mes groupes | Les groupes dont vous faites partie, et les cours et parcours qu'ils vous proposent. |
| 12 | `badges` | `/my-badges` | page title | Badges | Terminez un cours pour gagner son badge. Ils débloquent aussi des avatars. |
| 13 | `xp` | `/my-xp` | page title | Mon XP | Chaque leçon et chaque quiz réussi vous rapportent de l'XP. |
| 14 | `leaderboard` | `/leaderboard` | page title | Classement | Comparez votre XP à celle des autres apprenants. |
| 15 | `skills` | `/my-skills` | page title | Compétences | Les compétences que vos cours vous font acquérir. |
| 16 | `agenda` | `/agenda` | page title | Agenda | Les sessions en présentiel : inscrivez-vous ici. |
| 17 | `profile` | current page | `nav-profile` (avatar) | Votre profil | Votre avatar et votre bio. |
| 18 | `theme` | current page | `nav-theme` (theme toggle) | Mode sombre | Pour les yeux fatigués. |
| 19 | `manager` | current page | `nav-manager` | Mon espace | Suivez vos membres, vos groupes et leurs parcours. |
| 20 | `admin` | current page | `nav-admin` | Administration | Gérez les utilisateurs, les cours et les paramètres de la plateforme. |
| 21 | `end` | current page | none | C'est tout ! | Bon apprentissage ! Vous pouvez relancer cette visite depuis le menu (« Visite guidée »). |

- "page title" is the `<h1>` of the page, marked `data-tour="page-title"`.
- Steps 5 to 7 open the course of the card highlighted at step 3: the tour reads that card's link
  (`/courses/:slug`). The enrollment area of step 7 is highlighted as it is (« S'inscrire au
  cours », the progress block, or « Accès restreint »); the tour never clicks it
  ([011](011-course-page-and-enrollment.md)).
- Texts are first drafts: they live in `apps/app/src/lib/messages/tour.ts` ([005](005-ui-theme-navigation.md),
  "Language and formats") and may be reworded without changing this spec.
- The wording matches the rest of the interface (« vous »).

### Optional steps

- **Steps 3, 5, 6 and 7** are skipped when the catalogue has no course.
- **Step 4** is skipped when the « Reprendre » hero is not shown (no lesson started).
- A step whose target is missing for another reason (page in error, element hidden at this
  width) is **not** skipped: its bubble is shown centred on the page, without a highlight. A
  target that is in the page but hidden (the top bar links under 640 px) is shown centred at
  once, without waiting.

### Role-dependent steps (F188)

The steps are filtered by the role of `GET /api/auth/me` ([002](002-roles-and-guards.md)):

| Role | Step 19 `manager` | Step 20 `admin` |
| --- | --- | --- |
| `student` | - | - |
| `manager` | shown | - |
| `admin` | - | shown |

These steps point at the top bar entry; the tour does not open the admin app (it is a page load,
which would end the tour).

The progress counter counts only the steps the user will see (« 3 / 19 » for a student whose
catalogue has courses and who has started a lesson). An optional step is known to be skipped only
when it is reached: the total then drops (by 4 when the catalogue is empty).

## Display

### The bird and the bubble

- The bird is the logo image, 56 px, in the bubble's header, next to the title and the counter.
  Its `alt` is empty (decorative): the bubble carries the text.
- The bubble is a card in pupitre's tokens: `--card` background, `--border`, 16 px radius, the
  card shadow, Hanken Grotesk; title in bold, text in `--foreground`, counter in
  `--muted-foreground`. It stays readable in light and dark mode (AA contrast).
- Buttons: « Précédent » (ghost, hidden on step 1), « Suivant » (primary; « Terminer » on the last
  step), « Passer la visite » (link style). Counter « n / total ».
- Max width 320 px. Below 640 px of viewport width (the width under which the top bar hides its
  links), the bubble is docked at the bottom of the screen, full width minus 16 px on each side;
  the bird stays in its header.
- The bubble is placed by the tour itself: below the target, above it when there is no room
  below, centred on the target and kept 16 px inside the viewport. Without a target it is centred
  on the screen.

### Highlight

- The target is highlighted by a "spotlight": a fixed layer that dims the page (`rgb(0 0 0 / .45)`)
  with a cut-out around the target's box (8 px margin, `--radius`), and a 2 px `--ring` outline.
- The target is scrolled into view (`scrollIntoView({ block: 'center' })`) before the bubble shows.
- The spotlight follows the target on scroll and resize.
- While the tour runs, the page under the layer **cannot be clicked**: a click on the dimmed area
  does nothing. This keeps the tour and the page in step. The tour moves only with its own
  buttons and keys.

### Animation

- Between two steps, the bubble and the bird fly from their position to the next one (600 ms,
  `transform` only, eased), the spotlight follows, the text fades in and the bird flaps its
  wings.
- Under `prefers-reduced-motion: reduce`, there is no flight and no fade: the bird and the bubble
  appear in place.

## Navigation between pages

- A step on another page navigates with the router (`useNavigate`): every page of the tour is in
  the learner app, so there is no page load.
- After the navigation, the tour waits for the target to appear, for at most **3 s** (pages load
  their data). An optional step goes as soon as its page has loaded without its target: the
  catalogue marks its loaded results with `data-tour="catalogue-loaded"`. Meanwhile
  the bird is shown alone with a small "…" bubble. After 3 s, the step is shown centred (see
  "Optional steps").
- « Précédent » navigates back to the page of the previous step when needed.
- Pages behind `RequireAuth` are reachable because the user is signed in. If the session expires
  during the tour, the guard redirects to `/login` and the tour stops. A sign-out, or a sign-in as
  someone else, stops it too: it never resumes for the next user.

## Keyboard and accessibility

- The bubble is a `role="dialog"` with `aria-labelledby` (title) and `aria-describedby` (text),
  `aria-modal` while the tour runs, since the page under it cannot be used. Focus moves to the
  « Suivant » button at each step and is trapped in the bubble while the tour runs.
- The first-visit offer is a non-modal dialog: it does not take focus and the page stays usable.
- Keys: `→` / `Enter` next, `←` previous, `Escape` ends the tour (same as « Passer la visite »).
- Each step is announced through an `aria-live="polite"` region (« Étape 3 sur 19 : Un cours »).
- When the tour ends, focus returns to the element that had it before: the menu button for a
  restart (the side menu hands focus back to it as it closes), the page otherwise.

## Ending the tour

- « Terminer » (last step), « Passer la visite » and `Escape` end the tour: the layer and the bird
  disappear and the user stays on the current page.
- « Passer la visite » and `Escape` before the last step show the toast « Visite interrompue.
  Relancez-la depuis le menu quand vous voulez. ».
- Reloading the page or signing out ends the tour; it does not resume. It can be restarted from
  the menu.

## First lesson tip (F189)

The tour does not enter a lesson: that needs an enrollment, and the tour changes no data. Instead,
the **first time** the user opens a lesson page (`/courses/:slug/lessons/:index`, any course),
the bird shows a short tip of three steps, with the same bubble, spotlight, buttons and keys as
the tour:

| # | Target (`data-tour`) | Title | Text |
| --- | --- | --- | --- |
| 1 | `lesson-modules` (sidebar module list) | Les modules | Le programme du cours. La coche indique les modules terminés. |
| 2 | `page-title` (lesson title) | La leçon | Lisez à votre rythme : votre progression est enregistrée. |
| 3 | `lesson-nav` (previous / next buttons) | Avancer | Passez au module suivant ici. Quand un quiz est demandé, validez-le pour continuer. |

- It starts once the lesson content is shown, never over a loading page.
- Where the sidebar is hidden (mobile drawer), step 1 targets the « Modules » button instead
  (`lesson-modules-mobile`).
- The counter reads « 1 / 3 ». « Passer » and `Escape` close it, without a toast.
- It is shown once per user and browser, whether it was completed or closed, and whether or not
  the main tour was done. It is not in the side menu.
- It is not shown on the quiz page (`/courses/:slug/quiz/:index`): a user who reaches a quiz has
  already opened a lesson.

## Persistence

- Key `pupitre.tour.<userId>` in `localStorage`, read and written through
  `@pupitre/ui/lib/storage` ([003](003-security.md), AGENTS.md "Security rules"). Value: JSON
  `{ "version": 1, "status": "offered" | "started" | "completed" | "dismissed", "lessonTip": "seen" }`.
  `lessonTip` is absent until the first lesson tip has been shown.
- The tour is offered automatically only when there is no value for this user or when the stored
  `version` is lower than the current `TOUR_VERSION`. `TOUR_VERSION` is raised only when the tour
  changes enough to be worth showing again; rewording a text does not raise it.
- The key is per user id, so two accounts on the same browser each get the offer.
- Unreadable or invalid storage (private mode, bad JSON) counts as "never offered": the worst case
  is an offer shown again, never an error.
- Consequence of having no backend route: the tour is offered again in another browser or after
  site data is cleared. This is accepted.

## Implementation

| Path | Content |
| --- | --- |
| `apps/app/src/components/tour/LearnerLayout.tsx` | The learner app's layout: `Layout` of `@pupitre/core` inside `TourProvider`, used by `router.tsx` |
| `apps/app/src/components/tour/TourProvider.tsx` | Whether the tour runs, `startTour()`, the first-visit offer; the context lives in `tour-context.ts` |
| `apps/app/src/components/tour/TourRunner.tsx` | Runs a step list: navigation, waiting for the target, skipping, keyboard, counter; used by the tour and the lesson tip |
| `apps/app/src/components/tour/TourBubble.tsx` | The bird, the bubble and its placement, the focus trap |
| `apps/app/src/components/tour/TourSpotlight.tsx` | The dimming layer with the cut-out |
| `apps/app/src/components/tour/LessonTip.tsx` | The first lesson tip, mounted by `LessonPage` |
| `apps/app/src/lib/tour-steps.ts` | The step lists (tour and lesson tip), the role filter, `TOUR_VERSION` |
| `apps/app/src/lib/tour-storage.ts` | Reading and writing the persisted state |
| `apps/app/src/lib/messages/tour.ts` | The French strings |
| `apps/app/src/components/tour/tour.css` | The bubble, the spotlight and the flight, in pupitre's tokens (a new file, not one of the copied pupitre CSS files) |

- `TopBar`, `SideMenu` (in `@pupitre/core`) and the learner pages get `data-tour` attributes only;
  their markup and classes are otherwise unchanged.
- `SideMenu` is shared with the admin app: the « Visite guidée » entry takes an optional
  `onStartTour` prop and is shown only when it is given, so the admin app does not show it.
- The tour does not touch the session: it reads the user from `useSession()`.

## Mock mode and tests

- The tour works in `pnpm dev:mock` with the test accounts of [006](006-mock-mode.md); no new
  mock route is needed.
- The other end-to-end tests keep the tour out of the way: the `signIn` helpers mark it as seen
  (`skipTour` in `e2e/helpers.ts`); the tour tests pass `{ tour: true }`.
- Unit tests (Vitest): role filter of the steps, optional steps, persistence (absent, invalid,
  older version, current version), keyboard handling.
- E2E (Playwright, `e2e/tour.spec.ts`, mock mode):
  - `alice` signs in for the first time → the offer is shown; « C'est parti ! » → the tour goes
    through every step to « Terminer » and visits the first course's page, `/my-courses`,
    `/my-paths`, `/my-groups`, `/my-badges`, `/my-xp`, `/leaderboard`, `/my-skills`, `/agenda`;
  - on the course page, the user is still not enrolled after the tour (no `POST` sent);
  - the first lesson opened shows the lesson tip; the next lesson does not;
  - after « Plus tard », a reload does not show the offer again;
  - « Visite guidée » in the side menu restarts the tour;
  - `marc.manager` sees the « Mon espace » step, `admin` the « Administration » step, `alice`
    neither;
  - `Escape` ends the tour and focus returns;
  - the XSS account's username is shown as text in the welcome bubble and no payload runs;
  - checked in light and dark mode and at 360 px (no horizontal scroll).

## Acceptance criteria

- A new signed-in user is offered the tour once per browser; « Plus tard » and the end of the tour
  both prevent a new automatic offer.
- The tour visits the pages of the table in order, highlights each target, and the counter matches
  the number of steps shown for the role.
- A missing target never blocks the tour: optional steps are skipped, others are shown centred
  after at most 3 s.
- The tour can be completed with the keyboard only.
- Under `prefers-reduced-motion`, nothing moves.
- No request is sent to the API by the tour itself: it never enrolls the user.
- The first lesson tip is shown on the first lesson opened, once, and never on a quiz page.
- The admin app shows neither the offer nor the side menu entry.

## Open questions

- Should the admin app get its own tour (for admins and managers) later? Out of scope here.
- If the platform one day stores user preferences server-side, the tour state may move there; it
  would need a backend route, which is out of scope until the backend move is done
  ([199](199-backend-migration-plan.md)).

## Sources

New feature, no source in `genesary/pupitre` or `ebuildy/pupitre2`. Shell and targets:
`packages/core/src/components/TopBar.tsx`, `SideMenu.tsx`, `packages/core/src/lib/navigation.ts`,
`apps/app/src/routes/*.tsx`.
