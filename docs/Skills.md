# Skills

Skills are tags attached to the modules of a course. They filter courses by skill and define paths built around acquiring skills.

## Defining skills

Skills are declared in the course definition, on each module. The union of the skills of all the modules is denormalized on the course row, so the catalogue can filter by skill without reading the modules.

```yaml
slug: docker-basics
spec:
  title: "Docker fundamentals"
  difficulty: "beginner"
  modules:
    - name: "Introduction to containers"
      type: text
      src: "https://github.com/org/courses"
      ref: "main"
      path: "docker/intro.md"
      skills: [containers, docker]
    - name: "Docker quiz"
      type: quiz
      passingScore: 80
      skills: [docker]
      questions: [...]
```

The skills of a course are the **union of the `skills` of its modules**. The API computes them automatically and exposes them in the `/api/courses` response.

## Endpoints

### GET /api/skills/{slug}/modules

Returns every module of every course teaching this skill, with the completion status of the signed-in user.

```json
{
  "skill": "docker",
  "modules": [
    {
      "name": "Introduction to containers",
      "slug": "introduction-to-containers",
      "index": 0,
      "type": "text",
      "courseSlug": "docker-basics",
      "courseTitle": "Docker fundamentals",
      "status": "completed"
    },
    {
      "name": "Docker quiz",
      "slug": "docker-quiz",
      "index": 1,
      "type": "quiz",
      "courseSlug": "docker-basics",
      "courseTitle": "Docker fundamentals",
      "status": "available"
    }
  ]
}
```

Possible statuses:
- `completed` — quiz or lab passed, or text or video lesson viewed
- `available` — reachable (the previous module is completed, or it is the first of the list)
- `locked` — the previous modules are not all completed

### GET /api/my/skills/{slug}

Same response as above, with the status personalized for the signed-in user (authentication required).

## Frontend page `/skills/[slug]`

The page of a skill shows:
- The courses teaching this skill, **grouped by difficulty level** (Débutant / Intermédiaire / Avancé)
- For each course: a difficulty badge and the number of modules linked to the skill
- The "Retour" button goes back to the previous page (`history.back()`)

Courses are filtered on the client from `/api/courses`, on the `skills` field of each course.

## Skill paths (`kind: skill`)

A path can be of type `skill`, to follow the sequential acquisition of skills:

```yaml
slug: devops-path
spec:
  title: "DevOps path"
  kind: skill
  level: intermediate
  skills: [linux, docker, kubernetes, ci-cd]
```

### Progress in a skill path

- Skills unlock **sequentially** — the first is available, the next ones are locked until the previous one is completed.
- A skill is **completed** when all its assessable modules (quizzes and labs) are passed.
- The `GET /api/my/paths` endpoint returns the user's paths with the status of each skill (`completed` / `available` / `locked`).

### `level` field on paths

| Value            | Meaning      |
|------------------|--------------|
| `beginner`       | Beginner     |
| `intermediate`   | Intermediate |
| `advanced`       | Advanced     |

The level is shown as a badge on the `/my-paths` page.

## Defining a path

In the admin interface, the **Parcours** tab of `/admin/paths` creates, edits, reorders
and deletes paths; the **Inscriptions** tab manages enrolled users.

The same operations through the API: created with `POST /api/admin/courses/paths`, read with
`GET /api/admin/courses/paths/{slug}/definition`, replaced with
`PUT /api/admin/courses/paths/{slug}/definition`, deleted with
`DELETE /api/admin/courses/paths/{slug}/definition`.

`GET /api/paths/{slug}` also serves a path, but replaces its skills with the aggregated
skills of its member courses: to read a definition back as it was written, use the admin
endpoint.

```yaml
slug: security-path
spec:
  title: "Security path"
  description: "Master the basics of computer security"
  kind: skill          # "course" (default) or "skill"
  level: advanced      # beginner | intermediate | advanced (optional)
  skills:              # for kind=skill: ordered list of skills
    - linux
    - networking
    - cryptography
    - pentest
```

For a `kind: course` path, use `courses` instead of `skills`:

```yaml
spec:
  kind: course
  courses:
    - linux-intro
    - networking-basics
    - cyber-essentials
```
