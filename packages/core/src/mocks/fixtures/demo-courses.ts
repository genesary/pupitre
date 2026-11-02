import type { CourseDefinition, PathDefinition } from '../model';

// Hand-written courses that cover what the Go seed catalogue does not: every
// markdown feature, media, every question type, in-person sessions, an XP gate
// a private course and a skill path. Mock mode only (docs/specs/006-mock-mode.md).

const inDays = (days: number, hour: number) => {
  const date = new Date();
  date.setDate(date.getDate() + days);
  date.setHours(hour, 0, 0, 0);
  return date.toISOString();
};

export const DEMO_COURSES: CourseDefinition[] = [
  {
    slug: 'pupitre-demo',
    spec: {
      title: 'Découvrir pupitre',
      description:
        "Un cours de démonstration qui montre tout ce qu'une leçon pupitre sait afficher : admonitions, diagrammes, patterns, médias, quiz et labs.",
      public: true,
      category: 'programming',
      difficulty: 'beginner',
      badge: { name: 'Explorateur pupitre', icon: '🧭' },
      modules: [
        {
          name: 'Bienvenue',
          type: 'text',
          skills: ['pupitre'],
          content: `## Bienvenue

pupitre affiche le **markdown** des cours, avec quelques extensions.

:::tip Astuce
Le bouton en bas à droite bascule entre le mode docs et le mode lecture focus.
:::

:::warning[Attention]
Les admonitions sont des blocs \`:::type\` fermés par \`:::\`.
:::

:::danger
Un bloc de code n'est jamais interprété :

\`\`\`text
:::note
ceci reste du texte
:::
\`\`\`
:::

### Un tableau

| Commande | Effet |
|---|---|
| \`ls\` | Liste les fichiers |
| \`cd\` | Change de dossier |

### Une liste de tâches

- [x] Lire la leçon
- [ ] Faire le quiz

> Une citation, avec un [lien externe](https://kubernetes.io/docs/).

|||note Ceci est rendu par le pattern « note ».|||

|||callout
Un pattern **bloc** : son contenu est du markdown inline.
|||
`,
        },
        {
          name: 'Diagrammes et code',
          type: 'text',
          skills: ['pupitre'],
          content: `## Un diagramme mermaid

\`\`\`mermaid
graph LR
  A[Apprenant] --> B(Leçon)
  B --> C{Quiz}
  C -->|réussi| D[Badge]
  C -->|échoué| B
\`\`\`

## Du code

\`\`\`bash
kubectl get pods -n default
kubectl logs mon-pod -f
\`\`\`

### Contenu piégé

Le HTML ci-dessous ne doit **rien** exécuter (spec 003) :

<img src="x" onerror="alert('xss-markdown')">
<a href="javascript:alert('xss-link')">un lien piégé</a>
<script>alert('xss-script')</script>
<iframe src="https://example.com"></iframe>
`,
        },
        {
          name: 'Vérifier ses acquis',
          type: 'quiz',
          inline: true,
          passingScore: 50,
          questions: [
            {
              id: 'demo-q1',
              type: 'single',
              points: 1,
              question: 'Quel bloc ferme une admonition ?',
              answers: [
                { id: 'a', text: '`:::`', correct: true },
                { id: 'b', text: '`---`' },
                { id: 'c', text: '`|||`' },
              ],
              feedback: { correct: 'Exact.', wrong: 'Une admonition se ferme par `:::`.' },
            },
            {
              id: 'demo-q2',
              type: 'boolean',
              points: 1,
              question: 'Un bloc de code peut contenir `:::note` sans créer d’admonition.',
              correctAnswer: true,
            },
          ],
        },
        {
          name: 'Une vidéo',
          type: 'video',
          content: '/uploads/demo.mp4',
        },
        {
          name: 'Une image',
          type: 'image',
          content: '/pioupitre.png',
        },
        {
          name: 'Quiz complet',
          type: 'quiz',
          passingScore: 75,
          cooldown: { strategy: 'exponential', baseSeconds: 20, multiplier: 2, maxSeconds: 120 },
          maxAttemptsPerQuestion: 5,
          lockOnMaxAttempts: true,
          questions: [
            {
              id: 'quiz-single',
              type: 'single',
              points: 1,
              question: 'Quelle commande liste les pods ?',
              answers: [
                { id: 'a', text: '`kubectl get pods`', correct: true },
                { id: 'b', text: '`docker ps`' },
                { id: 'c', text: '`kubectl logs`' },
              ],
            },
            {
              id: 'quiz-multiple',
              type: 'multiple',
              points: 2,
              question: 'Lesquels sont des objets Kubernetes ?',
              answers: [
                { id: 'a', text: 'Pod', correct: true },
                { id: 'b', text: 'Deployment', correct: true },
                { id: 'c', text: 'Dockerfile' },
              ],
            },
            {
              id: 'quiz-boolean',
              type: 'boolean',
              points: 1,
              question: 'Un Service donne une adresse stable devant des Pods.',
              correctAnswer: true,
            },
            {
              id: 'quiz-order',
              type: 'order',
              points: 2,
              question: 'Remettez les étapes dans l’ordre :',
              items: [
                { id: 'build', text: 'Construire l’image' },
                { id: 'push', text: 'Pousser l’image' },
                { id: 'apply', text: 'Appliquer le manifeste' },
              ],
              correctOrder: ['build', 'push', 'apply'],
            },
          ],
        },
        {
          name: 'Lab GitLab',
          type: 'lab',
          checkProvider: 'gitlab',
          labUrl: 'https://gitlab.com/',
          steps: [
            { title: 'Projet créé', checkType: 'project_exists', checkParams: { project: 'e-learning/{{ .Username }}' } },
            { title: 'Merge request ouverte', checkType: 'open_mr', checkParams: { project: 'e-learning/{{ .Username }}' } },
          ],
          content: `## Objectifs

Créez votre projet, puis ouvrez une merge request.

### Étape 1

Créez le projet \`e-learning/<votre-identifiant>\`.

<!-- check:0 -->

### Étape 2

Ouvrez une merge request vers \`main\`.

<!-- check:1 -->
`,
        },
      ],
    },
  },
  {
    slug: 'atelier-kubernetes',
    spec: {
      title: 'Atelier Kubernetes en présentiel',
      description: 'Une journée en salle pour déployer une application de bout en bout.',
      public: true,
      inPerson: true,
      category: 'kubernetes',
      difficulty: 'intermediate',
      xpRequired: 200,
      sessions: [
        { id: 'paris-1', title: 'Session de Paris', date: inDays(5, 9), location: 'Paris — salle Turing', capacity: 12 },
        { id: 'lyon-1', title: 'Session de Lyon', date: inDays(19, 14), location: 'Lyon — salle Lovelace', capacity: 8 },
        { id: 'past-1', title: 'Session passée', date: inDays(-10, 9), location: 'Paris — salle Turing', capacity: 12 },
      ],
      modules: [
        {
          name: 'Préparer l’atelier',
          type: 'text',
          skills: ['kubernetes'],
          content: '## Préparer l’atelier\n\nInstallez `kubectl` et `kind` avant de venir.',
        },
      ],
    },
  },
  {
    slug: 'cours-prive',
    spec: {
      title: 'Cours réservé',
      description: 'Un cours privé : il faut être inscrit par un administrateur.',
      public: false,
      category: 'security',
      difficulty: 'advanced',
      modules: [{ name: 'Contenu réservé', type: 'text', content: '## Réservé\n\nContenu privé.' }],
    },
  },
];

/** A skill path: the Go seed only has course paths. */
export const DEMO_PATHS: PathDefinition[] = [
  {
    slug: 'parcours-competences-conteneurs',
    spec: {
      title: 'Compétences conteneurs',
      description: 'Les compétences à acquérir avant de passer en production.',
      kind: 'skill',
      level: 'beginner',
      skills: ['linux', 'docker', 'kubernetes'],
    },
  },
];
