Un module est un élément d'un cours. Il est composé d'un nom, d'un type et de son contenu — soit du markdown stocké directement sur le module (`content`), soit une source externe (`src`, `ref`, `path`). Il représente une ressource qui compose un cours : texte (markdown), vidéo, image, quiz ou liste de modules.

Le markdown inline est la voie d'édition par défaut : un module se crée en collant du markdown dans l'interface d'administration, ou en important un document entier (voir `docs/Course.md`, « Import / export markdown »). La source git reste disponible pour du contenu qui vit déjà dans un dépôt.

- **video / image** : l'URL de la ressource hébergée sur le serveur est renvoyée directement
- **text** : le markdown stocké sur le module (`content`), ou fetché depuis un dépôt git si `src`, `ref` et `path` sont renseignés — la source git l'emporte quand les deux sont présents
- **quiz** : questions inline dans la définition du cours ou fichier YAML fetché depuis git
- **modules** : entrée spéciale qui pointe vers un fichier YAML d'index dans git — expansée en place en une liste plate de modules au moment de la requête (voir ci-dessous)

### Type `modules` — index file

Permet de regrouper plusieurs modules dans un fichier YAML externe plutôt que de les déclarer un par un dans la définition du cours. L'API fetche et parse l'index au moment de la requête, via le `GitCache` existant.

**Entrée dans `spec.modules` :**

```yaml
- name: "Linux Introduction"
  type: modules
  src: "https://github.com/org/repo"
  ref: "main"
  path: "courses/linux-intro/index.yaml"
```

**Format du fichier index (`index.yaml`) :**

```yaml
- name: "Qu'est-ce que Linux ?"
  type: text
  path: courses/linux-intro/01-what-is-linux.md

- name: "Quiz Bases Linux"
  type: quiz
  path: courses/linux-intro/quizzes/quiz-bases-linux.yaml
  prerequisites:
    - qu-est-ce-que-linux

- name: "Module depuis un autre dépôt"
  type: text
  src: "https://github.com/org/autre-repo"   # override src
  ref: "main"                                 # override ref
  path: courses/autre/lecon.md
```

Champs disponibles dans une entrée d'index :

| Champ | Requis | Description |
|---|---|---|
| `name` | oui | Nom du module |
| `type` | non | Type de module (`text`, `quiz`, `video`, `image`). Défaut : `text` |
| `path` | oui | Chemin du fichier dans le dépôt |
| `src` | non | URL du dépôt git — hérité de l'entrée parente si absent |
| `ref` | non | Branche git — hérité de l'entrée parente si absent |
| `hidden` | non | Cache le module aux utilisateurs non-admin |
| `prerequisites` | non | Liste de slugs de modules à compléter avant d'accéder à celui-ci |

**Comportement :**
- Si `src` ou `ref` sont absents d'une entrée index, ils sont hérités de l'entrée parente
- Si le fichier index est introuvable ou invalide, l'entrée est ignorée et un warning est loggué
- L'expansion est transparente : les handlers `ListModules`, `GetModule`, `SubmitModule` voient une liste plate de modules normaux

### Endpoint

```
GET /api/courses/{slug}/modules/{index}
```

Réponse :

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
