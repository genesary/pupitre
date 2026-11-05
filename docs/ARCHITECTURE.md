# Architecture — E-Learning Platform

## Décision : une seule application

Le backend est une seule application Go, `pupitre-api` (`apps/api`), sur le port 8080. Elle regroupe trois domaines dans un même processus : `users`, `courses` et `checker`. Ils s'appellent directement, sans API interne. Le frontend est composé de deux applications React, servies sur la même origine.

## Source de vérité des cours : PostgreSQL

La seule source de vérité est la **base de données**. Les cours sont créés et modifiés via l'API d'administration ; il n'y a ni fichier YAML sur disque, ni ressource Kubernetes, ni cache en mémoire.

Chaque requête lit ce dont elle a besoin :

- le catalogue (`GET /api/courses`) filtre en SQL et ne charge aucune ligne de module ;
- l'affichage d'un cours charge ses modules en une requête indexée sur `(course_slug, position)`.

Aucune réplique ne conserve de contenu en mémoire : une réplique qui vient de démarrer sert exactement le même catalogue qu'une réplique en place depuis une semaine, et un redéploiement ne perd rien.

```jsonc
// POST /api/admin/courses
{
  "slug": "kubernetes-basics",
  "spec": {
    "title": "Kubernetes Basics",
    "description": "Apprenez les bases de Kubernetes",
    "public": true,
    "category": "kubernetes",
    "difficulty": "beginner",
    "modules": [
      { "name": "Introduction", "type": "text",
        "src": "https://github.com/org/courses", "ref": "main", "path": "intro.md" },
      { "name": "Démo vidéo", "type": "video", "src": "/uploads/demo.mp4" }
    ]
  }
}
```

### Schéma

| Table | Rôle |
|---|---|
| `courses` | Métadonnées du cours ; colonnes indexées pour le filtrage catalogue (`category`, `difficulty`, `public`, `title`) et `skills` dénormalisé |
| `course_modules` | Un module = une ligne, ordonnée par `position` ; charges utiles opaques (`questions`, `steps`, `check_params`) en `jsonb` |
| `course_prerequisites` | Conditions d'inscription |
| `course_sessions` | Sessions présentielles, clé `(course_slug, session_id)` — une écriture rejouée écrase la même ligne |
| `paths`, `path_courses`, `path_skills` | Parcours et leurs membres ordonnés |
| `quiz_question_attempts` | Compteur de tentatives et fin de cooldown par question |
| `lab_checks` | Historique des vérifications de labs |

## Architecture

```
                    Navigateur
                        │
                        ▼
              Ingress ou HTTPRoute (chart Helm)
                 │                  │
    /api, /uploads, /metrics,     le reste
    /health      │                  │
                 ▼                  ▼
  ┌─────────────────────────┐  ┌───────────────────────┐
  │       pupitre-api       │  │   Frontend (nginx)    │
  │                         │  │                       │
  │  users                  │  │  apps/app : apprenant │
  │  - auth OIDC, JWT       │  │  apps/admin :         │
  │  - profil, inscriptions │  │    /admin et /manager │
  │  - progression, réglages│  └───────────────────────┘
  │                         │
  │  courses                │
  │  - catalogue (SQL)      │
  │  - modules, git         │
  │  - upload de médias     │
  │  - lab_checks           │
  │                         │
  │  checker (OPA/Rego)     │
  │  - état GitLab via API  │
  │  - évalue la policy     │
  └────────────┬────────────┘
               ▼
          PostgreSQL
```

En développement, le serveur Vite joue le rôle de l'ingress : il envoie `/api` et `/uploads` à l'API.

## Parcours (Paths)

Un parcours est une séquence ordonnée de cours (`kind: course`) ou de compétences (`kind: skill`), stockée dans `paths` + `path_courses` / `path_skills`. Voir `docs/Skills.md` pour le détail.

```jsonc
// POST /api/admin/courses/paths
{
  "slug": "devops-path",
  "spec": {
    "title": "Parcours DevOps",
    "kind": "skill",
    "level": "intermediate",
    "skills": ["linux", "docker", "kubernetes", "ci-cd"]
  }
}
```

## Endpoints des parcours et compétences

| Méthode | Path | Description |
|---|---|---|
| `GET` | `/api/paths` | Liste tous les parcours |
| `GET` | `/api/paths/{slug}` | Détail d'un parcours |
| `GET` | `/api/skills/{slug}/modules` | Modules enseignant une compétence |
| `GET` | `/api/my/paths` | Parcours de l'utilisateur avec statut par cours/compétence |
| `GET` | `/api/my/skills/{slug}` | Modules d'une compétence avec statut pour l'utilisateur |
| `POST` | `/api/admin/paths/{slug}/enrollments` | Inscrire un utilisateur dans un parcours (admin) |
| `DELETE` | `/api/admin/paths/{slug}/enrollments/{userId}` | Désinscrire (admin) |

## Contenu des modules

L'API lit les cours et modules depuis PostgreSQL, à la demande, puis résout leur contenu :

- `type: video/image` → sert l'URL
- `type: text` avec `src` git → clone et lit le fichier
- `type: quiz` → questions inline ou YAML depuis git, avec score
- `type: modules` → fetche un fichier YAML d'index depuis git et expand les entrées en place (héritage `src`/`ref` depuis le module parent)

## Authentification

L'interface se connecte uniquement en OIDC via Keycloak : voir `docs/SSO.md`. L'API produit puis valide elle-même ses tokens JWT, signés avec `JWT_SECRET` (32 caractères minimum).

## Configuration

L'API charge sa configuration depuis un fichier YAML optionnel, indiqué par `CONFIG_PATH`. Les variables d'environnement **surchargent** les valeurs du fichier.

Le chart Helm monte ce fichier depuis une ConfigMap dans `/etc/pupitre/config.yaml`. Les valeurs sensibles (JWT, base de données) restent dans des Secrets Kubernetes. En local, `pnpm api:dev` lit `apps/api/.env`.

**Derrière un ingress, renseignez `TRUSTED_PROXIES`** avec les adresses de l'ingress, par exemple via `api.extraEnv`. Par défaut, seul le loopback est de confiance : tous les clients partagent alors l'adresse de l'ingress et une seule limite de débit, et les connexions finissent en `429` dès la 11ᵉ tentative dans la minute.

Le frontend est statique : l'image `docker/front` sert les deux applications avec nginx. Ne mettez aucun secret dans les variables `VITE_*`, elles sont livrées au navigateur.

### Personnalisation

```bash
# Surcharger une valeur via Helm
helm upgrade pupitre oci://ghcr.io/genesary/charts/pupitre --reuse-values --set api.env.GIT_CACHE_TTL=30

# Ou modifier les values.yaml puis helm upgrade
```

## Secret git — course-repo-secret

Pour les modules de cours qui référencent un **dépôt git privé**, l'API a besoin d'un token d'accès. Ce token est fourni via un Secret K8s monté dans le pod :

```
course-repo-secret (type: Opaque)
  └── git-credentials.yaml
        └── credentials:
              - url: "github.com/org/*"        # glob pattern
                token: "github_pat_xxx..."
```

Le fichier suit le format YAML avec une liste d'entrées **url → token**. La première entrée qui correspond (via `path.Match`) est utilisée.

**Création :**

```bash
kubectl create secret generic course-repo-secret \
  --from-file=git-credentials.yaml=./git-credentials.yaml
```

**Chemin de montage dans le pod :** `/etc/git-credentials/git-credentials.yaml`

Le chart indique cet emplacement à l'API (`gitCredentialsPath`, ou la variable `GIT_CREDENTIALS_PATH`). Si le secret n'existe pas, le pod démarre, mais le clone de dépôts privés échoue.

Voir `apps/api/infra/examples/course-service/course-secret.yaml` pour un exemple complet.
