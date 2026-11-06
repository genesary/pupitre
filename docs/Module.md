A module is an element of a course. It is made of a name, a type and its content — either markdown stored directly on the module (`content`), or an external source (`src`, `ref`, `path`). It represents a resource that makes up a course: text (markdown), video, image, quiz or list of modules.

Inline markdown is the default editing path: a module is created by pasting markdown into the admin interface, or by importing a whole document (see `docs/Course.md`, "Markdown import and export"). The git source remains available for content that already lives in a repository.

- **video / image**: the URL of the resource hosted on the server is returned directly
- **text**: the markdown stored on the module (`content`), or fetched from a git repository if `src`, `ref` and `path` are set — the git source wins when both are present
- **quiz**: inline questions in the course definition, or a YAML file fetched from git
- **modules**: special entry pointing to a YAML index file in git — expanded in place into a flat list of modules at request time (see below)

### `modules` type — index file

Groups several modules in an external YAML file rather than declaring them one by one in the course definition. The API fetches and parses the index at request time, through the existing `GitCache`.

**Entry in `spec.modules`:**

```yaml
- name: "Linux Introduction"
  type: modules
  src: "https://github.com/org/repo"
  ref: "main"
  path: "courses/linux-intro/index.yaml"
```

**Format of the index file (`index.yaml`):**

```yaml
- name: "What is Linux?"
  type: text
  path: courses/linux-intro/01-what-is-linux.md

- name: "Linux basics quiz"
  type: quiz
  path: courses/linux-intro/quizzes/linux-basics-quiz.yaml
  prerequisites:
    - what-is-linux

- name: "Module from another repository"
  type: text
  src: "https://github.com/org/other-repo"   # override src
  ref: "main"                                 # override ref
  path: courses/other/lesson.md
```

Fields available in an index entry:

| Field | Required | Description |
|---|---|---|
| `name` | yes | Name of the module |
| `type` | no | Type of the module (`text`, `quiz`, `video`, `image`). Default: `text` |
| `path` | yes | Path of the file in the repository |
| `src` | no | URL of the git repository — inherited from the parent entry if absent |
| `ref` | no | Git branch — inherited from the parent entry if absent |
| `hidden` | no | Hides the module from non-admin users |
| `prerequisites` | no | List of module slugs to complete before reaching this one |

**Behaviour:**
- If `src` or `ref` are absent from an index entry, they are inherited from the parent entry
- If the index file is missing or invalid, the entry is ignored and a warning is logged
- Expansion is transparent: the `ListModules`, `GetModule` and `SubmitModule` handlers see a flat list of normal modules

### Endpoint

```
GET /api/courses/{slug}/modules/{index}
```

Response:

```json
{
    "index": 0,
    "slug": "module-slug",
    "name": "Module Title",
    "type": "text",
    "content": "## Markdown content\n\nHello world",
    "viewed": false
}
```
