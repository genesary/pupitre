# 012 - Lesson player, markdown rendering, media and progress

Features: F053 (module list and sequential locking), F054 (module prerequisites), F055 (hidden
modules), F056 (lesson player), F057 (reading mode), F058 (markdown), F059 (admonitions), F060
(mermaid), F061 (patterns), F062 (video and image), F100 (lesson progress).

## Route

`/courses/:slug/lessons/:index`, signed in. `:index` is the numeric module index.

## Loading

1. `GET /api/courses/{slug}/modules` and `GET /api/courses/{slug}/modules/{index}` in parallel.
2. Redirects:
   - non-numeric index → `/courses/:slug`;
   - 403 (not enrolled, prerequisites) or locked module → `/courses/:slug`;
   - `quiz` module → `/courses/:slug/quiz/:index` (replacing the history entry);
   - inline quiz (`inline`) → the host lesson (`/courses/:slug/lessons/<host index>`);
   - any other error → « Module introuvable », with a link to the catalogue.

## Layout

Two modes, chosen with a floating button and stored in `localStorage`
(`pupitre.readingMode` = `docs` or `focus`):

- **Docs mode** (default), three columns:
  - left: « ← Retour au cours » link, « X/Y complétés » progress bar, module tree (icon, name, ✓
    when done, active state, locked modules not clickable). Inline quizzes are not listed;
  - centre: breadcrumb (Catalogue › course title › module), title, content, navigation;
  - right: table of contents of the `h2` and `h3` headings of the content, with anchors.
- **Focus mode**: only the centre column remains, with a small floating progress bar (« X/Y »).
- Mobile: the module tree opens in a `Sheet` and closes when a module is chosen.

The breadcrumb shows the course slug today: it shows the course title.

## Content per type

| Type | Rendering |
| --- | --- |
| `text` | Markdown (pipeline below). The first `h1` or `h2` of the content is removed, because it repeats the title |
| `video` | `<video controls>` with `content` as the source (`/uploads/…` or an https URL) |
| `image` | `<img>` with `content` as the source and the module name as `alt` |
| `lab` | Markdown, plus the checks of [014](014-labs.md) |
| no content | « Aucun contenu disponible. » |

## Markdown pipeline

A pure function `renderMarkdown(md, patterns) → html`, unit tested, in this order:

1. **Normalization** of line endings (`\r\n` and `\r` → `\n`).
2. **Docusaurus-style admonitions**: `:::type`, `:::type Title` or `:::type[Title]`, closed by
   `:::`, not nestable. Types and icons: `note` 📝, `tip` 💡, `info` ℹ️, `warning` ⚠️,
   `danger` 🚨, `caution` ⚡. The default title is the type with a capital letter. Markers inside a
   code block (``` or ~~~) are ignored. The body stays markdown.
3. **Patterns** ([045](045-admin-patterns.md)):
   - block: `|||name` on its own line, the content, then `|||`; the content is rendered as inline
     markdown and replaces `{{content}}` in the pattern HTML;
   - inline: `|||name text|||`; the text replaces `{{content}}`.
4. **marked** (GFM: tables, task lists, autolinks).
5. **DOMPurify** ([003](003-security.md) S2).

Once the HTML is in the DOM, React components take the place of some elements:

- **Code blocks**: a « Copier » button, which turns into « ✓ Copié » for 2 s.
- **Mermaid**: each ` ```mermaid ` block becomes a diagram, rendered by `mermaid` loaded on demand
  (`theme: 'dark'` in dark mode, `'default'` otherwise), with `suppressErrors`. The diagram is
  redrawn when the theme changes.
- **Anchors**: every `h2` and `h3` gets a stable `id` (slug of its text) for the table of
  contents.

Patterns come from `GET /api/patterns`, once per session (TanStack Query cache). Their CSS is
injected in a single `<style>` element. This route requires a signed-in user: the current
frontend calls it without a token, so patterns never apply in lessons
([090](090-gaps-and-defects.md)).

## Navigation and progress

- Previous and next modules: the neighbours in the list, **excluding** inline quizzes. Previous is
  « ◂ Précédent », or « ◂ Cours » when there is none.
- Right button:
  - module not yet viewed, or last module: a button that calls
    `POST /api/courses/{slug}/lessons/{moduleSlug}/complete`, then navigates. Labels: « Passer au
    Lab ▸ » if the next module is a lab, « Suivant ▸ », « Terminer la leçon ▸ » if an inline quiz
    is still pending, « Terminer le cours ▸ » otherwise;
  - module already viewed and not the last one: a plain link to the next module.
- The last module always calls `complete`, even if already viewed: this is what records the end
  of the course on the server.
- After the last module: go back to the destination stored in `sessionStorage`
  (`pupitre.courseReturnTo`, set by a learning path page) if there is one, otherwise
  `/courses/:slug`.
- If an inline quiz is still pending on the last module: « Validez le quiz ci-dessous pour terminer
  le cours. »
- Progress updates without a reload (invalidation of the module list).
- If `complete` fails: toast « Erreur lors de la mise à jour ».

## Inline quiz

If the response contains `inlineQuiz` with questions, the inline quiz of [013](013-quizzes.md) is
shown below the content. Passing it updates the progress and the navigation.

## Acceptance criteria

- A `text` module with a `:::warning Attention` block shows an admonition titled « Attention ».
- A ` ```mermaid ` block shows a diagram that follows the dark mode.
- A pattern `|||note Hello|||` is shown with the HTML and CSS of the `note` pattern.
- Switching to focus mode then reloading keeps the mode.
- The content of [003](003-security.md) (S2 criteria) runs nothing.

## Sources

`frontend/src/pages/courses/[slug]/lessons/[index].astro`, `styles/admonitions.css`,
`styles/learning.css`; `course-service/internal/handlers/modules.go` (`GetModule`), `lessons.go`.
