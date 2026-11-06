# Interactive labs — Lab Checker

## Overview

Labs are modules of type `lab` in a course. The learner reads the instructions directly in the platform, does the work in GitLab, then clicks **Vérifier mon travail**. The checker evaluates the work automatically through an OPA/Rego policy.

```
Learner                      pupitre-api                 Checker (same process)
    │                            │                              │
    ├── reads the instructions ──► inline markdown rendering    │
    │   (lesson page)             │                              │
    ├── works in GitLab           │                              │
    │                             │                              │
    ├── clicks "Vérifier" ───────►│                              │
    │                      POST /api/courses/:slug/modules/:i/check
    │                             ├── reads check.yaml + check.rego from git
    │                             ├── calls the checker ───────►│
    │                             │                    ├── fetches the GitLab state
    │                             │                    │   (MRs, commits, pipeline, files)
    │                             │                    └── evaluates the Rego policy
    │                             │◄── {allow, violations} ─────┤
    │                             ├── stores in DB (lab_checks)  │
    │◄── result displayed ────────┤                              │
```

## File structure

For each lab, two files must sit next to `content.md` in the git repository:

```
modules/lab1/
  ├── content.md     # Lab instructions (markdown)
  ├── check.yaml     # Checker configuration
  └── check.rego     # OPA/Rego policy
```

### check.yaml

```yaml
provider: gitlab
project: "e-learning/{{ .Username }}"   # template resolved with the learner's username
files:
  - "lab.py"                            # files to check on the default branch
policy: "check.rego"                    # relative path to the Rego policy
```

### check.rego — Lab 1 example

The violation messages are shown to learners, so they stay in French.

```rego
package checker.lab
import rego.v1

branch_re := `^(feature|hotfix|chore|docs|fix|refactor|test|perf|ci|build|revert)/[a-zA-Z0-9][a-zA-Z0-9._/-]*$`
commit_re := `^(feat|fix|chore|docs|style|refactor|test|perf|ci|build|revert)(\([^)]+\))?: .+`

default allow := false
allow if { count(violations) == 0 }

violations contains "Aucune Merge Request ouverte trouvée..." if { count(input.openMrs) == 0 }
violations contains "Nom de branche non conventionnel..." if {
    count(input.openMrs) > 0
    count(mrs_with_conventional_branch) == 0
}
violations contains msg if {
    some mr in mrs_with_conventional_branch
    mr.pipelineStatus != "success"
    msg := sprintf("Pipeline CI/CD non validée sur '%v' (statut : '%v').", [mr.sourceBranch, mr.pipelineStatus])
}
violations contains msg if {
    some mr in mrs_with_conventional_branch
    not mr_has_conventional_commit(mr)
    msg := sprintf("Aucun commit conventionnel sur '%v'.", [mr.sourceBranch])
}
violations contains "Fichier lab.py introuvable sur main." if { not input.files["lab.py"] }

mrs_with_conventional_branch contains mr if {
    some mr in input.openMrs
    regex.match(branch_re, mr.sourceBranch)
}
mr_has_conventional_commit(mr) if {
    some commit in mr.commits
    regex.match(commit_re, split(commit.message, "\n")[0])
}
```

### OPA input provided by the checker

```json
{
  "project": { "id": "1", "name": "bellinil", "path": "e-learning/bellinil", "defaultBranch": "main" },
  "openMrs": [
    {
      "iid": 3,
      "title": "feat: add user greeting",
      "sourceBranch": "feature/bellinil-greeting",
      "pipelineStatus": "success",
      "commits": [{ "message": "feat: add user greeting\n" }]
    }
  ],
  "mergedMrCount": 0,
  "files": { "lab.py": "print(\"Hello, bellinil!\")\n" }
}
```

## Module configuration in the course definition

```yaml
modules:
  - name: "Lab 1: GitLab Branching & CI/CD with Conventional Commits"
    type: "lab"
    src: "http://10.89.0.1:8880/e-learning/devops-course.git"   # internal URL (pod → host)
    ref: "main"
    path: "modules/lab1/content.md"
    labUrl: "http://localhost:8880/e-learning/devops-course"    # browser URL (optional)
```

> **Note:** `src` uses the host IP reachable from the pods (`10.89.0.1:8880`).
> `labUrl` is the URL shown in the browser when a direct link is needed.

## Git credentials for private repositories

The API reads the credentials from the `course-repo-secret` secret:

```yaml
# git-credentials.yaml
credentials:
  - url: "10.89.0.1:8880/e-learning/*"
    token: "glpat-xxx"
```

> **Important:** the pattern uses `path.Match` — `*` does not cross `/`.
> Use `10.89.0.1:8880/e-learning/*`, not `10.89.0.1:8880/*`.

## GitLab Runner (local Kind environment)

A shell runner is deployed in the cluster to run the CI/CD pipelines:

```yaml
# /tmp/gitlab-runner.yaml
[[runners]]
  url = "http://10.89.0.1:8880"
  token = "glrt-xxx"
  executor = "shell"
  clone_url = "http://gitlab-http.gitlab.svc.cluster.local"  # Kind internal DNS
```

The `clone_url` is needed because `gitlab.local` does not resolve from inside the cluster.

## Trainer tracking — /admin/labs

Each check is recorded in the database:

```sql
CREATE TABLE lab_checks (
    id           BIGSERIAL PRIMARY KEY,
    username     TEXT        NOT NULL,   -- the learner's email
    courseSlug  TEXT        NOT NULL,
    moduleIndex INT         NOT NULL,
    moduleName  TEXT        NOT NULL,
    allow        BOOLEAN     NOT NULL,
    violations   TEXT[]      NOT NULL DEFAULT '{}',
    checkedAt   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified     BOOLEAN     NOT NULL DEFAULT FALSE   -- verified by the server, or reported by the client
);
```

The `/admin/labs` page shows:

- Global stats (total checks, passed, failed, active learners)
- A table filterable by course, with detailed violations
- Endpoint: `GET /api/admin/lab-checks?course=<slug>`

## Tested failure scenarios

| Condition | Returned message |
|---|---|
| No open MR | "Aucune Merge Request ouverte trouvée. Créez une MR sans la fusionner." |
| Non-conventional branch | "Nom de branche non conventionnel. Format attendu : `<type>/<description>`" |
| Non-conventional commit | "Aucun commit conventionnel sur la branche '...'. Format attendu : `feat: ...`" |
| Failed or missing pipeline | "Pipeline CI/CD non validée sur '...' (statut : '...')." |
| `lab.py` missing on main | "Fichier lab.py introuvable sur main." |
