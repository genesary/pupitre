# Labs interactifs — Lab Checker

## Vue d'ensemble

Les labs sont des modules de type `lab` d'un cours. L'apprenant lit l'énoncé directement dans la plateforme, réalise le travail dans GitLab, puis clique **Vérifier mon travail**. Le checker évalue automatiquement le travail via une policy OPA/Rego.

```
Apprenant                    pupitre-api                 Checker (même processus)
    │                            │                              │
    ├── lit l'énoncé ────────────► rendu markdown inline        │
    │   (lesson page)             │                              │
    ├── travaille dans GitLab     │                              │
    │                             │                              │
    ├── clique "Vérifier" ───────►│                              │
    │                      POST /api/courses/:slug/modules/:i/check
    │                             ├── lit check.yaml + check.rego depuis git
    │                             ├── appelle le checker ──────►│
    │                             │                    ├── fetch GitLab state
    │                             │                    │   (MRs, commits, pipeline, fichiers)
    │                             │                    └── évalue Rego policy
    │                             │◄── {allow, violations} ─────┤
    │                             ├── stocke en DB (lab_checks)  │
    │◄── résultat affiché ────────┤                              │
```

## Structure des fichiers

Pour chaque lab, deux fichiers doivent être co-localisés avec `content.md` dans le repo git :

```
modules/lab1/
  ├── content.md     # Énoncé du lab (markdown)
  ├── check.yaml     # Configuration du checker
  └── check.rego     # Policy OPA/Rego
```

### check.yaml

```yaml
provider: gitlab
project: "e-learning/{{ .Username }}"   # template résolu avec le username de l'apprenant
files:
  - "lab.py"                            # fichiers à vérifier sur la branche par défaut
policy: "check.rego"                    # chemin relatif vers la policy Rego
```

### check.rego — exemple Lab 1

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

### Input OPA fourni par le checker

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

## Configuration du module dans la définition du cours

```yaml
modules:
  - name: "Lab 1 : GitLab Branching & CI/CD with Conventional Commits"
    type: "lab"
    src: "http://10.89.0.1:8880/e-learning/devops-course.git"   # URL interne (pod → host)
    ref: "main"
    path: "modules/lab1/content.md"
    labUrl: "http://localhost:8880/e-learning/devops-course"    # URL navigateur (optionnel)
```

> **Note :** `src` utilise l'IP hôte accessible depuis les pods (`10.89.0.1:8880`).
> `labUrl` est l'URL affichée dans le navigateur si besoin de lien direct.

## Credentials git pour les repos privés

L'API lit les credentials depuis le secret `course-repo-secret` :

```yaml
# git-credentials.yaml
credentials:
  - url: "10.89.0.1:8880/e-learning/*"
    token: "glpat-xxx"
```

> **Important :** le pattern utilise `path.Match` — `*` ne traverse pas les `/`.
> Utiliser `10.89.0.1:8880/e-learning/*` et non `10.89.0.1:8880/*`.

## GitLab Runner (environnement local Kind)

Un runner shell est déployé dans le cluster pour exécuter les pipelines CI/CD :

```yaml
# /tmp/gitlab-runner.yaml
[[runners]]
  url = "http://10.89.0.1:8880"
  token = "glrt-xxx"
  executor = "shell"
  clone_url = "http://gitlab-http.gitlab.svc.cluster.local"  # DNS interne Kind
```

Le `clone_url` est nécessaire car `gitlab.local` ne résout pas depuis l'intérieur du cluster.

## Traçabilité formateur — /admin/labs

Chaque vérification est enregistrée en base de données :

```sql
CREATE TABLE lab_checks (
    id           BIGSERIAL PRIMARY KEY,
    username     TEXT        NOT NULL,   -- email de l'apprenant
    courseSlug  TEXT        NOT NULL,
    moduleIndex INT         NOT NULL,
    moduleName  TEXT        NOT NULL,
    allow        BOOLEAN     NOT NULL,
    violations   TEXT[]      NOT NULL DEFAULT '{}',
    checkedAt   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified     BOOLEAN     NOT NULL DEFAULT FALSE   -- vérifié par le serveur, ou signalé par le client
);
```

La page `/admin/labs` affiche :

- Stats globales (total vérifications, validés, échoués, apprenants actifs)
- Tableau filtrable par cours avec violations détaillées
- Endpoint : `GET /api/admin/lab-checks?course=<slug>`

## Scénarios d'échec testés

| Condition | Message retourné |
|---|---|
| Aucune MR ouverte | "Aucune Merge Request ouverte trouvée. Créez une MR sans la fusionner." |
| Branche non conventionnelle | "Nom de branche non conventionnel. Format attendu : `<type>/<description>`" |
| Commit non conventionnel | "Aucun commit conventionnel sur la branche '...'. Format attendu : `feat: ...`" |
| Pipeline échouée ou absente | "Pipeline CI/CD non validée sur '...' (statut : '...')." |
| `lab.py` absent sur main | "Fichier lab.py introuvable sur main." |
