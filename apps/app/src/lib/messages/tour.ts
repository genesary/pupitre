// French strings of the guided tour and the first lesson tip
// (docs/specs/007-guided-tour.md).

export const TOUR_MESSAGES = {
  welcomeTitle: (username: string) => `Bonjour ${username} !`,
  welcomeText: 'Bienvenue sur pupitre ! Je vais vous faire visiter la plateforme en quelques étapes.',
  start: "C'est parti !",
  later: 'Plus tard',
  close: 'Fermer la visite',
  laterToast: 'Vous pourrez relancer la visite depuis le menu.',
  skippedToast: 'Visite interrompue. Relancez-la depuis le menu quand vous voulez.',
  previous: 'Précédent',
  next: 'Suivant',
  finish: 'Terminer',
  skip: 'Passer la visite',
  skipTip: 'Passer',
  waiting: '…',
  announce: (n: number, total: number, title: string) => `Étape ${n} sur ${total} : ${title}`,
} as const;

type StepText = { title: string; text: string };

export const TOUR_STEP_TEXTS: Record<string, StepText> = {
  catalogue: {
    title: 'Le catalogue',
    text: "Tous les cours de la plateforme sont ici. C'est votre point de départ.",
  },
  search: { title: 'Trouver un cours', text: 'Cherchez par titre, ou filtrez par catégorie et par niveau.' },
  'course-card': { title: 'Un cours', text: 'Chaque carte est un cours. Ouvrons celui-ci ensemble.' },
  continue: {
    title: 'Reprendre',
    text: 'pupitre garde votre place : reprenez la dernière leçon en un clic.',
  },
  'course-page': {
    title: 'La page du cours',
    text: 'Sa description, son niveau, ses compétences et le badge à gagner.',
  },
  'course-modules': {
    title: 'Le programme',
    text: 'Les modules du cours, à suivre dans l’ordre. Certains se débloquent quand le précédent est terminé.',
  },
  'course-action': {
    title: "S'inscrire",
    text: "Inscrivez-vous ici pour commencer. Ensuite, ce bloc affiche votre progression et l'XP du cours.",
  },
  'my-courses': { title: 'Mes cours', text: 'Les cours auxquels vous êtes inscrit, avec votre progression.' },
  menu: {
    title: 'Le menu',
    text: 'Tout le reste est rangé ici : parcours, groupes, badges, XP, compétences, agenda.',
  },
  paths: { title: 'Mes parcours', text: 'Des cours enchaînés dans un ordre pensé pour vous.' },
  groups: {
    title: 'Mes groupes',
    text: 'Les groupes dont vous faites partie, et les cours et parcours qu’ils vous proposent.',
  },
  badges: {
    title: 'Badges',
    text: 'Terminez un cours pour gagner son badge. Ils débloquent aussi des avatars.',
  },
  xp: { title: 'Mon XP', text: "Chaque leçon et chaque quiz réussi vous rapportent de l'XP." },
  leaderboard: { title: 'Classement', text: 'Comparez votre XP à celle des autres apprenants.' },
  skills: { title: 'Compétences', text: 'Les compétences que vos cours vous font acquérir.' },
  agenda: { title: 'Agenda', text: 'Les sessions en présentiel : inscrivez-vous ici.' },
  profile: { title: 'Votre profil', text: 'Votre avatar et votre bio.' },
  theme: { title: 'Mode sombre', text: 'Pour les yeux fatigués.' },
  manager: { title: 'Mon espace', text: 'Suivez vos membres, vos groupes et leurs parcours.' },
  admin: {
    title: 'Administration',
    text: 'Gérez les utilisateurs, les cours et les paramètres de la plateforme.',
  },
  end: {
    title: "C'est tout !",
    text: 'Bon apprentissage ! Vous pouvez relancer cette visite depuis le menu (« Visite guidée »).',
  },
  'lesson-modules': {
    title: 'Les modules',
    text: 'Le programme du cours. La coche indique les modules terminés.',
  },
  'lesson-content': { title: 'La leçon', text: 'Lisez à votre rythme : votre progression est enregistrée.' },
  'lesson-nav': {
    title: 'Avancer',
    text: 'Passez au module suivant ici. Quand un quiz est demandé, validez-le pour continuer.',
  },
};
