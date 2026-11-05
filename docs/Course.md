Un cours est un objet constitué d'une liste de modules, d'une description, d'un titre, d'un état de publication, d'une catégorie et d'une difficulté.

### Définition

Un cours est stocké en base de données et se crée ou se modifie via l'API d'administration :

- `POST   /api/admin/courses` — création, corps `{ "slug": "...", "spec": { ... } }`
- `GET    /api/admin/courses/{slug}/definition` — définition complète
- `PUT    /api/admin/courses/{slug}/definition` — remplacement de la définition
- `DELETE /api/admin/courses/{slug}/definition` — suppression
- `POST   /api/admin/courses/import` — création / mise à jour depuis un document markdown
- `POST   /api/admin/courses/import/preview` — même résolution, sans rien écrire
- `GET    /api/admin/courses/{slug}/export/markdown` — export du cours en markdown

Le champ `spec` a la forme suivante (présenté ici en YAML pour la lisibilité ; l'API attend du JSON) :

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
        Kubernetes est un orchestrateur de conteneurs.
    - name: "Depuis un dépôt"
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

Module types: `text` (markdown depuis git), `video` / `image` (URL hébergée sur le serveur), `quiz` (questions inline ou YAML depuis git), `modules` (index file — expansion en place de plusieurs modules depuis git).

Le champ `skills` sur un module liste les **compétences** que ce module enseigne (tags libres en kebab-case). Les compétences sont agrégées automatiquement au niveau du cours (union de tous les modules) et exposées dans `/api/courses`.

```yaml
modules:
  - name: "Introduction aux conteneurs"
    type: text
    src: "https://github.com/org/repo"
    ref: "main"
    path: "docker/intro.md"
    skills: [conteneurs, docker]
  - name: "Quiz Docker"
    type: quiz
    passingScore: 80
    skills: [docker]
    questions: [...]
```

- `metadata.name` : slug du cours (utilisé dans les URLs)
- `spec.title` : titre du cours
- `spec.description` : description du cours
- `spec.public` : boolean — `true` = visible dans le catalogue et auto-enrôlement à la première visite d'un module ; `false` = cours privé, accessible uniquement aux utilisateurs déjà enrôlés (non visible dans le catalogue public)
- `spec.category` : catégorie du cours (ex: kubernetes, linux)
- `spec.difficulty` : niveau de difficulté (beginner, intermediate, advanced)
- `spec.modules[].name` : nom du module
- `spec.modules[].type` : type de module (`text`, `video`, `image`, `quiz`, `modules`)
- `spec.modules[].src` : URL du dépôt git ou de la ressource média
- `spec.modules[].ref` : branche git
- `spec.modules[].path` : chemin du fichier dans le dépôt
- `spec.modules[].content` : le markdown du module, stocké en base — alternative à `src`/`ref`/`path`. Si les deux sont renseignés, la source git l'emporte.
- `spec.modules[].replication` : boolean (optionnel) — si `true`, le serveur télécharge la ressource distante (video/image) et la sert localement via `/uploads/`

#### Type `modules` — inclusion par index file

Pour les cours à nombreux modules hébergés dans git, le type `modules` évite de déclarer chaque module individuellement dans la définition du cours. Une seule entrée pointe vers un fichier YAML d'index dans git ; l'API l'expand en place à chaque requête.

```yaml
modules:
  - name: "Linux Introduction"
    type: modules
    src: "https://github.com/org/repo"
    ref: "main"
    path: "courses/linux-intro/index.yaml"
```

Le fichier `index.yaml` liste les modules dans l'ordre d'affichage. Les champs `src` et `ref` sont hérités de l'entrée parente si absents. Voir `docs/Module.md` pour le format complet de l'index.

### Import / export markdown

Un cours entier s'écrit et se relit comme **un seul document markdown**. C'est la
voie d'édition mise en avant dans l'interface d'administration : elle ne demande
aucun dépôt git.

```markdown
---
slug: introduction-linux
title: Introduction à Linux
category: linux
difficulty: beginner
public: true
split: h2
---

## Qu'est-ce que Linux ?

Linux est un **noyau**.

## Quiz

<!--pupitre
type: quiz
passingScore: 80
questions:
  - id: q1
    type: single
    points: 1
    question: "Que fait chmod ?"
    answers:
      - {id: a, text: "Change les permissions", correct: true}
      - {id: b, text: "Change le propriétaire", correct: false}
-->
```

**En-tête (`---`)** — optionnel. C'est la définition du cours, avec exactement les
mêmes clés que le `spec` de l'API, plus :

| Clé | Description |
|---|---|
| `slug` | Slug du cours. À défaut : le `slug` de la requête, sinon dérivé du `title` (accents repliés, `[a-z0-9-]`). |
| `split` | Niveau de titre qui commence un nouveau module : `none`, `h1`…`h6`. La requête peut l'outrepasser. |

**Découpage** — `split: none` fait du document entier un seul module. Sinon, chaque
titre du niveau choisi commence un module, nommé d'après ce titre. Le texte situé
avant le premier titre devient un module de tête ; s'il ne contient que le titre du
document, il est ignoré. Les titres à l'intérieur d'un bloc de code (``` ou ~~~)
ne découpent rien.

**Directive `<!--pupitre …-->`** — bloc YAML optionnel placé juste sous un titre,
qui porte tout ce que le markdown ne sait pas exprimer : `type`, `src`/`ref`/`path`,
`questions`, `skills`, `prerequisites`, `hidden`, `passingScore`, `steps`… Les clés
sont celles du module dans le `spec`. Aucun moteur de rendu ne l'affiche, donc le
document reste lisible tel quel. Un module texte avec du contenu inline n'en a pas
besoin.

Quand un module tire son contenu d'ailleurs (source git, `questions` inline, `video`,
`image`, `modules`), le markdown écrit sous son titre est ignoré et un avertissement
est renvoyé dans `warnings`.

#### Modes d'import

| Mode | Effet |
|---|---|
| `create` (défaut) | Crée le cours. `409` si le slug est déjà pris. |
| `replace` | Remplace les modules du cours. Si le document a un en-tête, il remplace aussi les champs du cours ; sinon les métadonnées stockées sont conservées. |
| `append` | Ajoute les modules du document à la suite des modules existants. L'en-tête est ignoré. |

`replace` et `append` renvoient `404` si le slug ne correspond à aucun cours.

`POST /api/admin/courses/import/preview` résout le document exactement comme
l'import (conflits et 404 compris) mais n'écrit rien : il renvoie le slug visé, les
champs du cours, la liste des modules (avec la taille du corps de chacun et un
marqueur `existing` pour ceux déjà en place) et les `warnings`.

#### Export

`GET /api/admin/courses/{slug}/export/markdown` rend le cours stocké sous la forme
du document que l'import relit à l'identique. Le niveau de titre est choisi parmi
ceux qu'aucun corps de module n'utilise déjà — puis inscrit dans `split` — pour que
le cycle export → import ne recoupe pas un module en deux. Le paramètre `?split=h2`
force un niveau.

Les corps de requête admin sont plafonnés à 10 Mo (contre 1 Mo ailleurs), un cours
entier tenant dans un seul document.

### Endpoints

#### GET /api/courses/{slug}/modules

Retourne la liste des modules avec le statut `viewed` par utilisateur :

```json
{
    "modules": [
        {
            "index": 0,
            "slug": "what-is-kubernetes",
            "name": "Qu'est-ce que Kubernetes ?",
            "type": "text",
            "viewed": false
        },
        {
            "index": 1,
            "slug": "core-concepts",
            "name": "Concepts fondamentaux",
            "type": "text",
            "viewed": false
        }
    ]
}
```

#### GET /api/courses/{slug}/modules/{index}

Retourne un module avec son contenu :

```json
{
    "index": 0,
    "slug": "what-is-kubernetes",
    "name": "Qu'est-ce que Kubernetes ?",
    "type": "text",
    "content": "# Markdown content...",
    "viewed": false
}
```

Le champ `viewed` est relatif aux données personnelles stockées en base de données.
Le champ `slug` est la version DNS-compliant du nom du module.
