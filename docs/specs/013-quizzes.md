# 013 - Quizzes: quiz page and inline quiz

Features: F075-F081 (definition, question types, scoring, review, progress, cooldowns, locking),
F082 (quiz page), F083 (inline quiz), F084 (anti-copy), F085 (answer confidentiality).

## Routes

- `/courses/:slug/quiz/:index` (signed in): a `quiz` module on its own page.
- The **inline quiz** is the same component, shown below a lesson's content
  ([012](012-lesson-player.md)) from the host module's `inlineQuiz` field.

Redirects of the quiz page: locked module or 403 → `/courses/:slug`; inline quiz (`inline`) → the
host lesson, or `/courses/:slug` when there is none.

## Data

`GET /api/courses/{slug}/modules/{index}` returns, for a quiz:

- `questions[]`: `id`, `type` (`single`, `multiple`, `boolean`, `order`), `question`, `points`,
  `difficulty`, `answers[] {id, text}` (single, multiple), `items[] {id, text}` (order). Correct
  answers are **never** sent to a learner (`correct` exists only for an admin);
- `quizConfig`: `passingScore` (in %), `cooldown {strategy, baseSeconds, multiplier, maxSeconds}`,
  `maxAttemptsPerQuestion`, `lockOnMaxAttempts`;
- `cooldowns`: per question, `{ remainingSeconds, attempts, locked }`;
- `bestScore`, `maxScore`, `passed`, `attempts`.

## Answers

| Type | Interaction | Initial value | Body sent |
| --- | --- | --- | --- |
| `single` | Single choice (radio) | none | `{ "single": "<answerId>" }` |
| `multiple` | Checkboxes | `[]` | `{ "multiple": ["<id>", …] }` |
| `boolean` | Two buttons « Vrai » / « Faux » | none | `{ "boolean": true }` |
| `order` | Reorderable list (drag and drop **and** ↑ ↓ buttons for the keyboard) | order received | `{ "order": ["<itemId>", …] }` |

A question is "answered" when its value is not empty (always for `order`).

Submission: `POST /api/courses/{slug}/modules/{index}/submit` with
`{ "answers": { "<questionId>": <value> } }`.

Response: `{ totalScore, maxScore, passed, questionResults[], cooldowns }`. Each
`questionResults` entry contains `questionId`, `isCorrect`, `pointsEarned`, `pointsMax`,
`feedback` and `sourceRefs`. It never contains the correct answer
([190](190-backend-defects.md) B17).

## Quiz page

- Breadcrumb, title, « N question(s) ».
- One card per question: number, question text, points, options.
- « Navigation » side palette: one chip per question (answered or not), an « X / N » counter and
  a progress bar; clicking a chip scrolls to the question.
- « Soumettre le quiz » button at the top and bottom, disabled while sending (« Envoi… »).

### Result

- Percentage and `totalScore/maxScore`.
- Passed: « Quiz réussi ! », then « Félicitations - X pts sur Y. »
- Failed: « Quiz échoué », then « Score insuffisant (P % < S %). Réessayez ! » when a threshold
  exists, otherwise « Score : X/Y ».
- Actions: « Module suivant ▸ » if passed and a next unlocked module exists; « Recommencer » if
  failed and no question is cooling down or locked; « Retour au cours ».
- **Review** (« Correction »): for each question, ✓ or ✗, the question text, « X/Y pt(s) » and
  `feedback`. The correct answer is not shown: the API no longer sends it, so that a learner
  cannot read it and submit again (audit finding L4).
- `sourceRefs` may produce « Revoir : <module> » links to the cited lesson and anchor.

## Cooldowns and locking

- After a submission, or a **425** whose body contains `cooldowns`, every cooling-down question
  shows « ⏳ Réessayez dans Ns » (counting down every second), and every locked question
  « 🔒 Question verrouillée ». These questions are disabled.
- Cooldowns are kept on the client so they survive a reload. The current frontend uses a cookie
  `pcd_<slug>_<index>`; pupitre2 uses `sessionStorage` with the same key, since the server returns
  `cooldowns` with the module anyway. The server value always wins.

## Anti-copy

Inside the quiz container (page and inline quiz): `copy` and `cut` are blocked, as well as the
context menu (except in input fields), and text is not selectable (`user-select: none`). This is a
deterrent, not a protection.

## Inline quiz

- Same rendering and rules, in a card below the lesson, with a « X/N répondues » counter.
- Passing it (`passed`) calls a callback that updates the lesson's progress and navigation. If the
  quiz was already passed on load, the callback is not called again.

## Text rendering

The question text, answers, items and `feedback` are rendered as sanitized **inline markdown**
(code, bold, links), never as raw HTML.

## Acceptance criteria

- An `order` question can be reordered with the keyboard using ↑ and ↓.
- A submission with a wrong answer and a 30 s cooldown shows the countdown, keeps the question
  disabled after a reload, then enables it again.
- A learner's DOM never contains the correct answer before submission.
- Copying the text of a question copies nothing.

## Sources

`frontend/src/pages/courses/[slug]/quiz/[index].astro`, inline quiz in
`pages/courses/[slug]/lessons/[index].astro`; `course-service/internal/handlers/modules.go`
(`SubmitModule`, `publicAnswer`), `content/scoring.go`, `content/quiz_types.go`.
