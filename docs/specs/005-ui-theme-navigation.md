# 005 - Theme, navigation, toasts, page states and accessibility

Features: F010 (light / dark theme), F011 (navigation shell), F012 (toasts), F018 (language).

## Colour tokens

The tokens of the current frontend (`styles/tokens.css`) become the CSS variables of the shadcn
theme, declared in `packages/ui/src/styles/globals.css` (`:root` and `.dark`).

| Current token | Light | Dark | shadcn variable |
| --- | --- | --- | --- |
| `ink` | `#2b2f36` | `#dde1ea` | `--foreground` |
| `ink-soft` | `#6b7280` | `#8b92a0` | `--muted-foreground` |
| `line` | `#e2e6ef` | `#252830` | `--border`, `--input` |
| `fill` | `#f3f5f9` | `#1a1d24` | `--muted`, `--secondary` |
| `paper` | `#ffffff` | `#1e2128` | `--card`, `--popover` |
| `page` | `#fefefe` | `#16181f` | `--background` |
| `blue` | `#1b86f2` | `#1b86f2` | `--primary`, `--ring` |
| `blue-deep` | `#1466c4` | - | hover of the primary button |
| `blue-soft` | `#e2f0fd` | `#162336` | `--accent` |
| `orange` / `orange-soft` | `#ff9d18` / `#fff3e0` | - / `#271a08` | `--warning` (added) |
| `good` / `good-soft` | `#3bb273` / `#dcf2e6` | - / `#0d2218` | `--success` (added) |
| `bad` / `bad-soft` | `#e5534b` / `#fde8e7` | - / `#270f0e` | `--destructive` |
| `purple` / `purple-soft` | `#7c3aed` / `#f3e8ff` | - / `#1a0e38` | skill badges |

- Radius: `--radius: 10px` (cards: 16 px).
- Base font size: 17 px instead of pupitre's 16 px, for readability (decided change): almost every
  size and spacing is in `rem`, so the whole interface grows by about 6 %. Set on `html` in
  `packages/ui/src/styles/globals.css` as `106.25%`, which still follows the user's own default
  font size, for both apps.
- Fonts: Hanken Grotesk (text), Caveat (the "pupitre" logo), JetBrains Mono (code), served locally
  (see [003](003-security.md) S6).
- The logo is `apps/app/public/pioupitre.png`, next to the word "pupitre" in Caveat.

## Visual design

**pupitre2 keeps the current pupitre design.** The colours above, the fonts, the radii and the
look of every screen (cards, badges, category colours, "Reprendre" hero, level colours, lesson
layout, quiz palette, podium…) are those of the current Astro frontend, not a new design.

- The visual reference of each screen is the current frontend: its page markup and
  `frontend/src/styles/*.css` (`components.css`, `learning.css`, `gamification.css`,
  `admonitions.css`, `admin.css`) at commit `3cdf7b2` of `genesary/pupitre`.
- Those styles are ported to Tailwind classes or to a small CSS layer in `packages/ui/src/styles/`, keeping the
  same values (colours, spacing, sizes, shadows).
- shadcn components are restyled with these tokens; their default look does not replace pupitre's.
- Code reused from the reference rewrite is restyled the same way ([090](090-gaps-and-defects.md),
  rule 6). Its theme is not imported.
- A design change is a separate, explicit decision, never a side effect of the rewrite.
- Decided change: in light mode, the learner app's page background is the drawing of
  `ebuildy/pupitre2` (`apps/app/public/images/pupitre-background.svg`, on `body` in
  `apps/app/src/index.css`), in place of the page wrappers' gradient (`.cat-outer`,
  `.course-outer`, `.page-outer`, `.skill-outer`, `.up-outer`, `.agenda-wrap`). It covers the
  viewport once and stays put while the page scrolls; its embedded fonts are cut down to the
  glyphs drawn. Dark mode keeps
  `--color-page`; the admin app is unchanged.
- Decided change: the catalogue ([010](010-catalogue.md)) has a larger grid of up to five
  columns, its cards rise and fade in each time a filter changes, its difficulty filter is a range
  slider, and in light mode its content sits on a white panel (`.cat-panel`) over the background
  drawing. The rules are in `apps/app/src/index.css` (`.cat-grid`, `.cat-diff-range`,
  `.cat-panel`); the pupitre CSS copies are unchanged.
- Decided change: so that text stays readable over the drawing, the reading columns sit on a
  white panel in light mode: the course page's content column (`.course-main`), the lesson and its
  table of contents (`.lesson-center`, `.lesson-right`) and the quiz (`.quiz-main`). Below 640 px
  the columns are simply white. Dark mode is unchanged.
- Decided change: the other learner pages get the same white panel in light mode (My courses,
  paths, groups, badges, XP, leaderboard, skills, a skill's page, public profile, profile), and the
  course page's left column is solid white instead of a blurred translucent white.
- Decided change: in dark mode, every card is solid, like the module rows of the course page: a
  plain `--color-paper` background, without the transparency and backdrop blur of the pupitre
  cards (catalogue cards and « Reprendre », stats, XP, paths, skills, groups, leaderboard, public
  profile, course sidebar, sessions, members table). Only the background colour changes: the
  category borders, gradients and the « done » / « booked » variants keep theirs. In
  `packages/ui/src/styles/contrast-fixes.css`.
- Decided change: in dark mode, the catalogue cards' thumbnails are tinted with their category
  colour (20 % over `--color-paper`) instead of the light pastel dimmed by a filter, their borders
  use `--color-line` (the category top border stays), and the « Reprendre » and « +200 XP » pills
  and the « Reprendre » progress track get dark variants. In `apps/app/src/index.css`.
- Decided change: earned and locked badges are easier to tell apart, in both modes
  ([020](020-learner-space.md), "My badges"). An earned badge has a solid card, a green border,
  a green ring around its icon and a « ✓ Obtenu » label; a badge to unlock has an opaque card
  with a dashed border instead of a 55 % opacity one, only its icon in grey, and its colours back
  on hover. The badge card of the course page follows: dashed border while the badge is to earn.

## Light / dark mode

- The preference is stored in `localStorage.theme` (`light` or `dark`). Without a preference, the
  app follows `prefers-color-scheme`.
- The `dark` class is set on `<html>` **before the first render** (a small script loaded in the
  head, not inline) to avoid a flash.
- Toggle in the top bar (sun / moon icon) and in the side menu (« Mode sombre » / « Mode clair »).
- Mermaid diagrams are redrawn when the theme changes ([012](012-lesson-player.md)).

## Top bar (learner space)

From left to right:

1. Menu button (mobile and desktop), which opens the side menu.
2. Logo and "pupitre", link to `/courses`.
3. Links « Catalogue » (`/courses`) and « Mes cours » (`/my-courses`), with the active state.
4. « Mon espace » (`/manager/users`), for a manager only.
5. Theme toggle.
6. When signed in: avatar ([003](003-security.md) S3) and username (hidden on mobile), link to
   `/profile`; « Admin » button for an admin; « Déconnexion » button.
   Otherwise: « Se connecter » button to `/login`.

## Side menu

shadcn `Sheet` component, closed by a click on a link or on the backdrop.

| Section | Entries |
| --- | --- |
| « Mon apprentissage » | 📚 Catalogue · 🎓 Mes cours · 🗺️ Mes parcours · 👥 Mes groupes |
| « Progression & Gamification » | 🏅 Mes badges · ⚡ Mon XP · 🏆 Classement |
| « Compétences & Planning » | 🎯 Mes compétences · 📅 Agenda |
| - | 🐤 Visite guidée (signed in, learner app only, [007](007-guided-tour.md)) · Theme toggle · 🚪 Déconnexion (in red) |

The entry of the current page is highlighted (same path or sub-path).

## Admin sidebar

A **single** list, used by every admin page (today `Admin.astro` and the old dashboard do not have
the same entries):

Tableau de bord · Utilisateurs · Cours · Parcours · Groupes · Patterns · Paramètres · Classement ·
Labs · Exports CSV · Monitoring, then « ← Retour au catalogue ».

A manager who opens `/admin/exports` sees only « Exports CSV » and the link back to the catalogue.

## Manager sidebar

👥 Mes membres (`/manager/users`) · 🏷️ Mes groupes (`/manager/groups`) · 🗺️ Parcours
(`/manager/paths`) · ← Retour au catalogue.

## Toasts

- `Toaster` component (sonner) mounted once. API: `toast.success`, `toast.error`, `toast.info`,
  `toast.warning`.
- Duration: 4 s. Position: bottom right on desktop, top on mobile.
- Messages are plain text (never HTML, see [003](003-security.md) S1).
- The `alert()` calls of the current frontend (agenda, session booking) become error toasts.

## Page states

Every page that loads data has three states, with shared components:

- **loading**: skeletons (`Skeleton`) in place of the cards or rows;
- **empty**: an illustration (emoji), a message and, when useful, an action (« Explorer le
  catalogue »);
- **error**: the API message and a « Réessayer » button.

## Language and formats

- The interface is in French. UI strings are grouped per domain in `apps/app/src/lib/messages/<domain>.ts`
  so they can be translated later (English is not required).
- Dates and numbers: `Intl.DateTimeFormat('fr-FR')` and `Intl.NumberFormat('fr-FR')`.
- Levels: `beginner` → « Débutant », `intermediate` → « Intermédiaire », `advanced` → « Avancé ».

## Responsive design and accessibility

- Default Tailwind breakpoints; no page scrolls horizontally at 360 px.
- Every icon button has an `aria-label`.
- Full keyboard navigation, visible focus (`--ring`).
- AA contrast on text, in light and dark mode. The dark overrides the pupitre CSS lacks are in
  `packages/ui/src/styles/contrast-fixes.css` ([090](090-gaps-and-defects.md), A26 and A27), checked
  by `e2e/contrast.spec.ts`.
- Dialogs (`Dialog`, `AlertDialog`) manage focus and close with Escape.
- Every deletion goes through a confirmation `AlertDialog`.

## Sources

`frontend/src/styles/tokens.css`, `styles/*.css`, `layouts/Base.astro`, `layouts/App.astro`,
`layouts/Admin.astro`, `layouts/Manager.astro`, `pages/admin/[...slug].astro`.
