# Adding courses through the admin API

How an administrator creates and edits courses by calling the API directly, from PowerShell
(Windows) or a Linux shell, instead of the « Administration » pages. The routes and their rules
are those of [042](../specs/042-admin-courses.md), [106](../specs/106-backend-courses-and-content.md)
and the route inventory of [103](../specs/103-backend-http-api.md); this guide only shows how to
call them.

Two ways to write a course:

- **A JSON definition** (`POST /api/admin/courses`): the whole course, every field available.
- **A markdown document** (`POST /api/admin/courses/import`): easier to write, one module per
  heading; what markdown cannot say goes in a `<!--pupitre … -->` comment.

Both examples of this guide are in [`examples/`](examples/) and are accepted as they are.

## 1. Get a token

The API takes the session token of the web app (`Authorization: Bearer <token>`,
[105](../specs/105-backend-authentication.md)). It is valid 8 hours, and « Déconnexion » revokes
it at once.

1. Sign in to pupitre with an **admin** account (locally: `admin.kc`, see `apps/api/compose.yaml`).
2. Open the browser's developer tools (F12), « Console » tab, on a pupitre page.
3. Run:

   ```js
   JSON.parse(sessionStorage.getItem('pupitre.session')).token
   ```

4. Copy the value, without the quotes.

The token gives full admin rights: never paste it in a file of the repository, a ticket or a
chat. Prefer reading it into a variable as below, which keeps it out of the shell history.

## 2. Set up the shell

The API is reached through the site's address: `http://localhost:3000/api` locally (`pnpm dev`),
or `http://localhost:8080/api` when only the API runs (`pnpm api:dev`); `https://<your
domain>/api` in production.

PowerShell (Windows PowerShell 5.1 or PowerShell 7):

```powershell
$Api = 'http://localhost:3000/api'
$Token = [System.Net.NetworkCredential]::new('', (Read-Host 'Token' -AsSecureString)).Password
$Headers = @{ Authorization = "Bearer $Token" }

# Sends JSON and reads the answer as UTF-8. Windows PowerShell 5.1 would read the answer as
# Latin-1 (the API sends no charset), and sending it back would then damage the accents.
function Send-Json($Method, $Path, $Json) {
  $Params = @{ Method = $Method; Uri = "$Api$Path"; Headers = $Headers; UseBasicParsing = $true }
  if ($Json) {
    $Params.ContentType = 'application/json; charset=utf-8'
    $Params.Body = [System.Text.Encoding]::UTF8.GetBytes($Json)
  }
  $Text = [System.Text.Encoding]::UTF8.GetString((Invoke-WebRequest @Params).RawContentStream.ToArray())
  if ($Text) { $Text | ConvertFrom-Json }
}
function Get-Json($Path) { Send-Json Get $Path }

Get-Json '/auth/me'   # shows your account: role must be admin
```

`Read-Host -AsSecureString` hides the token as you paste it.

Linux (bash, with `curl`, and `jq` for the markdown import and the edits: `sudo apt install jq`
on Debian or Ubuntu):

```sh
API=http://localhost:3000/api
read -rsp 'Token: ' TOKEN; echo
AUTH="Authorization: Bearer $TOKEN"

curl -fsS "$API/auth/me" -H "$AUTH"   # shows your account: role must be admin
```

## 3. Create a course from a JSON definition

[`examples/docker-intro.json`](examples/docker-intro.json) creates a public course with a text
module and a quiz.

PowerShell:

```powershell
Send-Json Post '/admin/courses' (Get-Content -Raw -Encoding UTF8 docs/guides/examples/docker-intro.json)
```

Linux:

```sh
curl -fsS -X POST "$API/admin/courses" -H "$AUTH" -H 'Content-Type: application/json' \
  --data-binary @docs/guides/examples/docker-intro.json
```

Answer: `201 {"slug":"docker-intro"}`. The course is in the catalogue at once (`/courses/docker-intro`).

The body is `{ "slug": "...", "spec": { ... } }`. The slug is lowercase letters, digits and dashes;
it is the course's address and cannot be changed later. The main `spec` fields:

| Field | Value |
| --- | --- |
| `title`, `description` | Text shown in the catalogue |
| `public` | `true`: listed in the catalogue, learners enrol on their own. `false`: only enrolled learners see it |
| `category` | `linux`, `devops`, `kubernetes`, `docker`, `networking`, `security`, `programming`, `python`, `git`, `cloud`, `database` (sets the icon and the catalogue filter) |
| `difficulty` | `beginner`, `intermediate`, `advanced` |
| `badge` | `{ "name": "...", "icon": "🐳" }`, earned when the course is completed |
| `prerequisites` | `[{ "course": "<slug>", "minScore": 50 }]`: courses to finish first |
| `modules` | The modules, in order |

A module has a `name`, a `type` and its body:

| `type` | Body |
| --- | --- |
| `text` | `content` (markdown), **or** `src` / `ref` / `path` to read a file from a git repository |
| `video`, `image` | `src`: a URL (`replication: true` downloads it once and serves it locally) |
| `quiz` | `questions` (below), `passingScore` (percentage needed to pass) |
| `lab` | `steps` checked against GitLab ([107](../specs/107-backend-labs-and-checker.md)) |

Optional on every module: `skills` (tags such as `docker`), `hidden`, `prerequisites` (module
slugs). A quiz question has an `id`, a `type`, `points` and the `question`:

| Question `type` | Answer |
| --- | --- |
| `single` | `answers: [{ "id", "text", "correct": true }]`, one correct |
| `multiple` | the same, several correct |
| `boolean` | `correctAnswer: true` or `false` |
| `order` | `items: [{ "id", "text" }]` and `correctOrder: ["id", …]` |

`feedback: { "correct": "...", "wrong": "..." }` is shown after the answer.

## 4. Create a course from markdown

[`examples/git-basics.md`](examples/git-basics.md): the front matter holds the course fields
(the same keys as `spec`, plus `slug` and `split`), `split: h2` makes one module per `##`
heading, and the `<!--pupitre … -->` comment turns the last section into a quiz.

Preview first: nothing is written, the answer lists the modules and the warnings.

PowerShell:

```powershell
$Body = @{ markdown = [string](Get-Content -Raw -Encoding UTF8 docs/guides/examples/git-basics.md) } | ConvertTo-Json
Send-Json Post '/admin/courses/import/preview' $Body
Send-Json Post '/admin/courses/import' $Body
```

The `[string]` matters: without it, Windows PowerShell 5.1 sends the file as an object
(`{"value": …, "PSPath": …}`) and the API answers `400 Invalid import request`.

Linux:

```sh
BODY_FILE=$(mktemp)
jq -Rs '{markdown: .}' docs/guides/examples/git-basics.md > "$BODY_FILE"
curl -fsS -X POST "$API/admin/courses/import/preview" -H "$AUTH" -H 'Content-Type: application/json' \
  --data-binary @"$BODY_FILE"
curl -fsS -X POST "$API/admin/courses/import" -H "$AUTH" -H 'Content-Type: application/json' \
  --data-binary @"$BODY_FILE"
rm "$BODY_FILE"
```

The body goes through a file: passed as `-d "$BODY"`, a document over 128 KiB would exceed
Linux's limit on one argument.

Answer: `201 {"slug":"git-basics","mode":"create","moduleCount":4}`.

Besides `markdown`, the body takes:

| Field | Value |
| --- | --- |
| `mode` | `create` (default; `409` if the slug exists), `replace` (replaces the modules of an existing course), `append` (adds modules to it) |
| `slug` | The target course; for `create`, overrides the front matter's |
| `split` | `none`, `h1` … `h6`; overrides the front matter's |

To get a course back as markdown, edit it and import it again with `mode: replace`:

```powershell
Invoke-RestMethod -Uri "$Api/admin/courses/git-basics/export/markdown" -Headers $Headers -OutFile git-basics.md
```

```sh
curl -fsS "$API/admin/courses/git-basics/export/markdown" -H "$AUTH" -o git-basics.md
```

## 5. Edit or delete a course

A course is always written whole: read its definition, change it, send it back. Read it again
right before writing, so you do not overwrite another admin's change
([042](../specs/042-admin-courses.md), "Concurrency rule").

PowerShell (rename the course):

```powershell
$Def = Get-Json '/admin/courses/docker-intro/definition'
$Def.spec.title = 'Docker : les bases'
Send-Json Put '/admin/courses/docker-intro/definition' (@{ spec = $Def.spec } | ConvertTo-Json -Depth 50)
```

Linux:

```sh
curl -fsS "$API/admin/courses/docker-intro/definition" -H "$AUTH" \
  | jq '{spec: (.spec | .title = "Docker : les bases")}' \
  | curl -fsS -X PUT "$API/admin/courses/docker-intro/definition" -H "$AUTH" \
      -H 'Content-Type: application/json' --data-binary @-
```

`ConvertTo-Json` needs `-Depth`: without it, Windows PowerShell cuts the modules and the questions
at the second level.

Delete: the course is removed at once, with no confirmation and no way back. Save its definition
first:

```powershell
Invoke-WebRequest -UseBasicParsing -Uri "$Api/admin/courses/docker-intro/definition" -Headers $Headers -OutFile docker-intro.backup.json
Invoke-RestMethod -Method Delete -Uri "$Api/admin/courses/docker-intro/definition" -Headers $Headers
```

```sh
curl -fsS "$API/admin/courses/docker-intro/definition" -H "$AUTH" -o docker-intro.backup.json
curl -fsS -X DELETE "$API/admin/courses/docker-intro/definition" -H "$AUTH"
```

The backup is the `{ "slug", "spec" }` body of step 3: `POST` it to `/admin/courses` to create
the course again.

## 6. Enrol learners

Needed for a course with `public: false`. Find the ids first (`GET /admin/users/search?q=`,
`GET /admin/groups`).

PowerShell:

```powershell
Get-Json '/admin/users/search?q=alice'
Send-Json Post '/admin/enrollments/docker-intro' (@{ userId = '<user id>' } | ConvertTo-Json)
Send-Json Post '/admin/enrollments/docker-intro/groups' (@{ groupId = '<group id>' } | ConvertTo-Json)
```

Linux:

```sh
curl -fsS "$API/admin/users/search?q=alice" -H "$AUTH"
curl -fsS -X POST "$API/admin/enrollments/docker-intro" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"userId":"<user id>"}'
curl -fsS -X POST "$API/admin/enrollments/docker-intro/groups" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"groupId":"<group id>"}'
```

## 7. Courses read from git

A module with `src` / `ref` / `path` is read from the repository when a learner opens it, and kept
in a cache for 10 minutes. After pushing a change to the repository, clear the course's cache to
see it at once:

```powershell
Invoke-RestMethod -Method Post -Uri "$Api/admin/courses/docker-intro/cache/clear" -Headers $Headers
```

```sh
curl -fsS -X POST "$API/admin/courses/docker-intro/cache/clear" -H "$AUTH"
```

The API must reach the repository: a module whose repository is unreachable shows « Module
introuvable » to the learner. A private repository needs `GIT_TOKEN` or `GIT_CREDENTIALS_PATH`
([101](../specs/101-backend-configuration.md)).

## Errors

| Answer | Meaning |
| --- | --- |
| `401 Missing Authorization header`, `401 Invalid token`, `401 Session expired` | No token, a wrong one, or one revoked by « Déconnexion » or expired: get a new one (step 1) |
| `403` | The account is not admin |
| `400 slug is required`, `400` invalid slug | The body has no `slug`, or it holds other characters than `a-z`, `0-9` and `-` |
| `400 Invalid course spec` | The JSON is malformed, or a field has the wrong type (a string instead of a list…) |
| `404` | Unknown course (edit, delete, `replace` and `append` imports) |
| `409` | The slug is taken: pick another, or edit the course instead |

A misspelt field name is **not** an error: the API drops unknown keys without a word and still
answers `201`. Read the course back (`GET /admin/courses/<slug>/definition`) to check that every
field is there.

PowerShell stops on an error; the API's message is in `$_.ErrorDetails.Message`:

```powershell
try { Send-Json Post '/admin/courses' (Get-Content -Raw -Encoding UTF8 docs/guides/examples/docker-intro.json) } catch { $_.ErrorDetails.Message }
```

With `curl`, drop `-f` to see the message of an error answer.
