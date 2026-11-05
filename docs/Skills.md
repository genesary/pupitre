# Compétences (Skills)

Les compétences sont des tags associés aux modules d'un cours. Elles permettent de filtrer les cours par compétence et de définir des parcours orientés acquisition de compétences.

## Définition des compétences

Les compétences se déclarent dans la définition du cours, au niveau de chaque module. L'union des compétences de tous les modules est dénormalisée sur la ligne du cours, ce qui permet au catalogue de filtrer par compétence sans lire les modules.

```yaml
slug: docker-basics
spec:
  title: "Les fondamentaux de Docker"
  difficulty: "beginner"
  modules:
    - name: "Introduction aux conteneurs"
      type: text
      src: "https://github.com/org/courses"
      ref: "main"
      path: "docker/intro.md"
      skills: [conteneurs, docker]
    - name: "Quiz Docker"
      type: quiz
      passingScore: 80
      skills: [docker]
      questions: [...]
```

Les compétences d'un cours sont l'**union de tous les `skills` de ses modules**. Elles sont calculées automatiquement par l'API et exposées dans la réponse `/api/courses`.

## Endpoints

### GET /api/skills/{slug}/modules

Retourne tous les modules de tous les cours qui enseignent cette compétence, avec le statut de complétion de l'utilisateur connecté.

```json
{
  "skill": "docker",
  "modules": [
    {
      "name": "Introduction aux conteneurs",
      "slug": "introduction-aux-conteneurs",
      "index": 0,
      "type": "text",
      "courseSlug": "docker-basics",
      "courseTitle": "Les fondamentaux de Docker",
      "status": "completed"
    },
    {
      "name": "Quiz Docker",
      "slug": "quiz-docker",
      "index": 1,
      "type": "quiz",
      "courseSlug": "docker-basics",
      "courseTitle": "Les fondamentaux de Docker",
      "status": "available"
    }
  ]
}
```

Statuts possibles :
- `completed` — quiz/lab passé, ou leçon texte/vidéo consultée
- `available` — accessible (le module précédent est complété, ou premier de la liste)
- `locked` — les modules précédents ne sont pas tous complétés

### GET /api/my/skills/{slug}

Même réponse que ci-dessus, mais avec le statut personnalisé pour l'utilisateur connecté (authentification requise).

## Page frontend `/skills/[slug]`

La page d'une compétence affiche :
- Les cours qui enseignent cette compétence, **groupés par niveau de difficulté** (Débutant / Intermédiaire / Avancé)
- Pour chaque cours : badge de difficulté, nombre de modules liés à la compétence
- Le bouton "Retour" renvoie à la page précédente (`history.back()`)

Les cours sont filtrés côté client depuis `/api/courses` sur le champ `skills` de chaque cours.

## Parcours de type skill (`kind: skill`)

Un parcours peut être de type `skill` pour suivre l'acquisition séquentielle de compétences :

```yaml
slug: devops-path
spec:
  title: "Parcours DevOps"
  kind: skill
  level: intermediate
  skills: [linux, docker, kubernetes, ci-cd]
```

### Progression dans un parcours skill

- Les compétences sont débloquées **séquentiellement** — la première est disponible, les suivantes verrouillées jusqu'à complétion de la précédente.
- Une compétence est **complétée** quand tous ses modules évaluables (quiz et lab) sont passés.
- L'endpoint `GET /api/my/paths` retourne les parcours de l'utilisateur avec le statut de chaque compétence (`completed` / `available` / `locked`).

### Champ `level` sur les parcours

| Valeur           | Signification |
|------------------|---------------|
| `beginner`       | Débutant      |
| `intermediate`   | Intermédiaire |
| `advanced`       | Avancé        |

Le niveau est affiché comme badge sur la page `/my-paths`.

## Définition d'un parcours

Depuis l'interface d'administration, l'onglet **Parcours** de `/admin/paths`
crée, modifie, réordonne et supprime les parcours ; l'onglet **Inscriptions**
gère les utilisateurs inscrits.

Les mêmes opérations en API : créé via `POST /api/admin/courses/paths`, lu via
`GET /api/admin/courses/paths/{slug}/definition`, remplacé via
`PUT /api/admin/courses/paths/{slug}/definition`, supprimé via
`DELETE /api/admin/courses/paths/{slug}/definition`.

`GET /api/paths/{slug}` sert aussi un parcours, mais y substitue les
compétences agrégées de ses cours membres : pour relire une définition telle
qu'elle a été écrite, passer par l'endpoint admin.

```yaml
slug: security-path
spec:
  title: "Parcours Sécurité"
  description: "Maîtrisez les bases de la sécurité informatique"
  kind: skill          # "course" (défaut) ou "skill"
  level: advanced      # beginner | intermediate | advanced (optionnel)
  skills:              # pour kind=skill : liste ordonnée de compétences
    - linux
    - networking
    - cryptographie
    - pentest
```

Pour un parcours `kind: course`, utiliser `courses` à la place de `skills` :

```yaml
spec:
  kind: course
  courses:
    - linux-intro
    - networking-basics
    - cyber-essentials
```
