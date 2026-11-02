# 107 - Backend: labs and the checker

A lab is a module of type `lab`: the learner reads the instructions in pupitre, works in GitLab,
then asks pupitre to check the work ([014](014-labs.md)). In `genesary/pupitre` the check ran in
checker-service, called by course-service over HTTP with the internal secret. In the monolith the
checker is the `checker` package, called in-process ([104](104-backend-in-process-calls.md)), with
**the same limits**.

## Flow

1. `POST /api/courses/{slug}/modules/{index}/check` (signed in): the courses domain resolves the
   lab module, reads `check.yaml` and the Rego policy from the course's git repository, and calls
   `Checker.Evaluate`.
2. The checker resolves the GitLab project, fetches its state once (project, open merge requests
   with their commits and pipeline status, merged MR count, the listed files on the default
   branch), and evaluates the policy with that state as input.
3. The result `{allow, violations}` is stored in `lab_checks` (a storage failure is logged, the
   learner still gets the result), recorded as module progress (detached), and returned.

`POST /api/courses/{slug}/modules/{index}/check-step` checks one step of a guided lab through
`Checker.CheckStep`. `POST /api/courses/{slug}/modules/{index}/record` stores the result of a
check that ran in the browser. `GET /api/admin/lab-checks?course=` and the lab results routes feed
`/admin/labs` ([047](047-admin-exports-and-labs.md)).

## Lab files

Next to the lab's `content.md` in git:

- `check.yaml`: `provider: gitlab`, `project` (a template resolved with the learner's username,
  for example `e-learning/{{ .Username }}`), `files` to read on the default branch, `policy` (path
  of the Rego file).
- `check.rego`: package `checker.lab`, `default allow := false`, a `violations` set of French
  messages shown as is to the learner.

OPA input: `project {id, name, path, defaultBranch}`, `openMrs[] {iid, title, sourceBranch,
pipelineStatus, commits[] {message}}`, `mergedMrCount`, `files {name: content}`.

## Step checks

| Step type | Passes when |
| --- | --- |
| `gitlab_branch` | a branch starting with the given pattern exists |
| `gitlab_mr_open` | an MR is open |
| `gitlab_conventional_commit` | a commit of an open MR follows Conventional Commits |
| `gitlab_pipeline_passed` | the pipeline of an open MR succeeded |
| `gitlab_commit_on_branch` | a commit exists on the given branch |
| `gitlab_file_on_branch` | the given file exists on the branch |
| `gitlab_conventional_commit_on_branch` | a conventional commit exists on the branch |

The project's last path segment must be the learner's username, and only MRs authored by the
learner count. That username is the login the identity provider gave at the learner's last OIDC
sign-in (`preferred_username`, stored as `identityname`), never the profile username, which the
learner can change; a learner without one gets `403` ([190](190-backend-defects.md) B11). The same
username fills the `{{ .Username }}` of `check.yaml` and of the step parameters. GitLab lists are paginated by 100 and read to the end. The base branch defaults to
`main`. Messages are the French messages of `step.go`.

## Isolation and limits

Running learner-triggered policy evaluation inside the API process is the main risk of the move
([100](100-backend-scope-and-architecture.md)). Every limit of checker-service is kept:

| Limit | Value |
| --- | --- |
| OPA capabilities | restricted: no `http.send`, no `net.*`, no `opa.runtime` (no SSRF, no environment leak) |
| Evaluation timeout | 10 s per evaluation, on its own context |
| Input size | 5 MB of JSON, refused beyond |
| Rate limit | 20 checks per 60 s for the whole process, `429` relayed as `502` ([101](101-backend-configuration.md)) |
| GitLab access | `GITLAB_BASE_URL`, `GITLAB_TOKEN`; the token never leaves the server |

Policies come only from course repositories, which only admins configure. A panic during
evaluation is recovered by the request's `Recoverer`, as for any handler.

## Git credentials for private repositories

Per-URL tokens in `git-credentials.yaml` (`GIT_CREDENTIALS_PATH`), entries `{url, token}` matched
with `path.Match`: `*` does not cross `/` (write `host/e-learning/*`, not `host/*`).

## Acceptance criteria

- `eval_test.go`, `fetch_test.go`, `step_test.go`, `pagination_test.go` (against the mock GitLab)
  pass in `internal/checker`.
- The check tests of course-service pass with the checker wired in-process against the mock GitLab:
  a passing lab, each failure message of the table in `docs/interactive-labs.md`, a policy using
  `http.send` refused, an input above 5 MB refused, the 21st check in a minute answered `502`.

## Sources

`checker-service/internal/checker/` and `internal/handlers/handler.go`;
`course-service/internal/handlers/check.go`, `lab_results.go`; `docs/interactive-labs.md`.
