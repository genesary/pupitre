A course is an object made of a list of modules, a description, a title, a publication state, a category and a difficulty.

### Definition

A course is stored in the database, and is created or edited through the admin API:

- `POST   /api/admin/courses` — creation, body `{ "slug": "...", "spec": { ... } }`
- `GET    /api/admin/courses/{slug}/definition` — full definition
- `PUT    /api/admin/courses/{slug}/definition` — replaces the definition
- `DELETE /api/admin/courses/{slug}/definition` — deletion
- `POST   /api/admin/courses/import` — creation or update from a markdown document
- `POST   /api/admin/courses/import/preview` — same resolution, without writing anything
- `GET    /api/admin/courses/{slug}/export/markdown` — exports the course as markdown

The `spec` field has the following shape (shown as YAML for readability; the API expects JSON):

```yaml
slug: kubernetes-basics
spec:
  title: "Kubernetes Basics"
  description: "Learn the fundamentals of Kubernetes"
  public: true
  category: "kubernetes"
  difficulty: "beginner"
  modules:
    - name: "What is Kubernetes"
      type: "text"
      content: |
        Kubernetes is a container orchestrator.
    - name: "From a repository"
      type: "text"
      src: "https://github.com/user/repo"
      ref: "main"
      path: "lessons/intro.md"
    - name: "Architecture Overview"
      type: "video"
      src: "/uploads/architecture.mp4"
    - name: "Kubernetes Basics Quiz"
      type: "quiz"
      passingScore: 80
      maxAttemptsPerQuestion: 3
      lockOnMaxAttempts: true
      cooldown:
        strategy: "exponential"
        baseSeconds: 30
        multiplier: 2.0
        maxSeconds: 600
      questions:
        - id: "q1"
          type: "single"
          points: 1
          question: "What is a Pod?"
          answers:
            - id: "a"
              text: "Smallest deployable unit"
              correct: true
            - id: "b"
              text: "A physical node"
              correct: false
```

Module types: `text` (markdown from git), `video` / `image` (URL hosted on the server), `quiz` (inline questions or YAML from git), `modules` (index file — expands several modules from git in place).

The `skills` field of a module lists the **skills** this module teaches (free kebab-case tags). Skills are aggregated automatically at course level (union of all the modules) and exposed in `/api/courses`.

```yaml
modules:
  - name: "Introduction to containers"
    type: text
    src: "https://github.com/org/repo"
    ref: "main"
    path: "docker/intro.md"
    skills: [containers, docker]
  - name: "Docker quiz"
    type: quiz
    passingScore: 80
    skills: [docker]
    questions: [...]
```

- `slug`: slug of the course (used in URLs)
- `spec.title`: title of the course
- `spec.description`: description of the course
- `spec.public`: boolean — `true` = visible in the catalogue, with auto-enrollment on the first visit to a module; `false` = private course, reachable only by users already enrolled (not visible in the public catalogue)
- `spec.category`: category of the course (e.g. kubernetes, linux)
- `spec.difficulty`: difficulty level (beginner, intermediate, advanced)
- `spec.modules[].name`: name of the module
- `spec.modules[].type`: type of the module (`text`, `video`, `image`, `quiz`, `modules`)
- `spec.modules[].src`: URL of the git repository or of the media resource
- `spec.modules[].ref`: git branch
- `spec.modules[].path`: path of the file in the repository
- `spec.modules[].content`: the module's markdown, stored in the database — an alternative to `src`/`ref`/`path`. If both are set, the git source wins.
- `spec.modules[].replication`: boolean (optional) — if `true`, the server downloads the remote resource (video/image) and serves it locally through `/uploads/`

#### `modules` type — inclusion through an index file

For courses with many modules hosted in git, the `modules` type avoids declaring each module individually in the course definition. A single entry points to a YAML index file in git; the API expands it in place on each request.

```yaml
modules:
  - name: "Linux Introduction"
    type: modules
    src: "https://github.com/org/repo"
    ref: "main"
    path: "courses/linux-intro/index.yaml"
```

The `index.yaml` file lists the modules in display order. The `src` and `ref` fields are inherited from the parent entry when absent. See `docs/Module.md` for the complete format of the index.

### Markdown import and export

A whole course is written and read back as **a single markdown document**. This is the
editing path the admin interface puts forward: it needs no git repository.

```markdown
---
slug: linux-introduction
title: Introduction to Linux
category: linux
difficulty: beginner
public: true
split: h2
---

## What is Linux?

Linux is a **kernel**.

## Quiz

<!--pupitre
type: quiz
passingScore: 80
questions:
  - id: q1
    type: single
    points: 1
    question: "What does chmod do?"
    answers:
      - {id: a, text: "Changes the permissions", correct: true}
      - {id: b, text: "Changes the owner", correct: false}
-->
```

**Header (`---`)** — optional. It is the course definition, with exactly the same keys
as the API's `spec`, plus:

| Key | Description |
|---|---|
| `slug` | Slug of the course. Otherwise: the `slug` of the request, or else derived from the `title` (accents folded, `[a-z0-9-]`). |
| `split` | Heading level that starts a new module: `none`, `h1`…`h6`. The request can override it. |

**Splitting** — `split: none` makes the whole document one module. Otherwise, each
heading of the chosen level starts a module, named after this heading. The text before
the first heading becomes a leading module; if it only holds the document's title, it
is ignored. Headings inside a code block (``` or ~~~) split nothing.

**`<!--pupitre …-->` directive** — optional YAML block placed right under a heading,
holding everything markdown cannot express: `type`, `src`/`ref`/`path`, `questions`,
`skills`, `prerequisites`, `hidden`, `passingScore`, `steps`… The keys are those of the
module in the `spec`. No renderer displays it, so the document stays readable as it is.
A text module with inline content does not need it.

When a module takes its content from elsewhere (git source, inline `questions`, `video`,
`image`, `modules`), the markdown written under its heading is ignored, and a warning is
returned in `warnings`.

#### Import modes

| Mode | Effect |
|---|---|
| `create` (default) | Creates the course. `409` if the slug is already taken. |
| `replace` | Replaces the course's modules. If the document has a header, it also replaces the course's fields; otherwise the stored metadata is kept. |
| `append` | Adds the document's modules after the existing ones. The header is ignored. |

`replace` and `append` return `404` if the slug matches no course.

`POST /api/admin/courses/import/preview` resolves the document exactly as the import
does (conflicts and 404 included) but writes nothing: it returns the target slug, the
course's fields, the list of modules (with the body size of each one and an `existing`
marker for those already in place) and the `warnings`.

#### Export

`GET /api/admin/courses/{slug}/export/markdown` renders the stored course as the
document the import reads back identically. The heading level is chosen among those no
module body already uses — then written in `split` — so that the export → import cycle
does not cut a module in two. The `?split=h2` parameter forces a level.

Admin request bodies are capped at 10 MB (against 1 MB elsewhere), since a whole course
fits in a single document.

### Endpoints

#### GET /api/courses/{slug}/modules

Returns the list of modules, with the `viewed` status per user:

```json
{
    "modules": [
        {
            "index": 0,
            "slug": "what-is-kubernetes",
            "name": "What is Kubernetes?",
            "type": "text",
            "viewed": false
        },
        {
            "index": 1,
            "slug": "core-concepts",
            "name": "Core concepts",
            "type": "text",
            "viewed": false
        }
    ]
}
```

#### GET /api/courses/{slug}/modules/{index}

Returns a module with its content:

```json
{
    "index": 0,
    "slug": "what-is-kubernetes",
    "name": "What is Kubernetes?",
    "type": "text",
    "content": "# Markdown content...",
    "viewed": false
}
```

The `viewed` field comes from the personal data stored in the database.
The `slug` field is the DNS-compliant version of the module's name.
