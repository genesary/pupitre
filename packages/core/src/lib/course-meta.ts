// Course presentation rules of the pupitre Astro frontend (pages/courses/index.astro,
// pages/courses/[slug]/index.astro), shared by every screen that shows a course
// (docs/specs/010-catalogue.md).

type CourseLike = { category?: string; skills?: string[] };

const CATEGORY_META: Record<string, { label: string; icon: string }> = {
  linux: { label: 'Linux', icon: '🐧' },
  devops: { label: 'DevOps', icon: '⚙️' },
  kubernetes: { label: 'Kubernetes', icon: '☸️' },
  docker: { label: 'Docker', icon: '🐋' },
  networking: { label: 'Réseaux', icon: '🌐' },
  security: { label: 'Sécurité', icon: '🛡️' },
  programming: { label: 'Programmation', icon: '💻' },
  python: { label: 'Python', icon: '🐍' },
  git: { label: 'Git', icon: '🔀' },
  cloud: { label: 'Cloud', icon: '☁️' },
  database: { label: 'Base de données', icon: '🗄️' },
  'internal-testing': { label: 'Tests internes', icon: '🧪' },
};

export function categoryLabel(slug: string): string {
  return CATEGORY_META[slug]?.label ?? slug.charAt(0).toUpperCase() + slug.slice(1).replace(/-/g, ' ');
}

export function categoryIcon(slug: string): string {
  return CATEGORY_META[slug]?.icon ?? '📖';
}

const ICON_RULES: [RegExp, string][] = [
  [/pentest|hack|exploit|cyber|vuln|ctf/, '🔐'],
  [/security|sécurité|secure/, '🛡️'],
  [/linux|unix|bash|shell/, '🐧'],
  [/git|gitlab|github|ci.?cd|devops|pipeline/, '⚙️'],
  [/docker|container|podman/, '🐋'],
  [/kubernetes|k8s|helm/, '☸️'],
  [/python/, '🐍'],
  [/network|réseau|tcp|ip|wifi/, '🌐'],
  [/cloud|aws|azure|gcp/, '☁️'],
  [/database|sql|postgres|mysql/, '🗄️'],
  [/web|http|html|css|frontend/, '🌍'],
  [/javascript|typescript|node/, '📦'],
  [/go|golang/, '🦫'],
];

/** Thumbnail icon derived from the category and skills (the API has no thumbnail, defect A2). */
export function courseIcon(course: CourseLike, fallback = categoryIcon(course.category ?? '')): string {
  const tags = [course.category ?? '', ...(course.skills ?? [])].join(' ').toLowerCase();
  return ICON_RULES.find(([pattern]) => pattern.test(tags))?.[1] ?? fallback;
}

export const DIFFICULTY_LABELS: Record<string, string> = {
  beginner: 'Débutant',
  intermediate: 'Intermédiaire',
  advanced: 'Avancé',
};

export const DIFFICULTY_BADGES: Record<string, string> = {
  beginner: 'badge-green',
  intermediate: 'badge-orange',
  advanced: 'badge-red',
};

export const LEVEL_ORDER: Record<string, number> = { beginner: 1, intermediate: 2, advanced: 3 };

/** Colour palette per category, assigned in order of appearance. */
export const CATEGORY_PALETTE = [
  { dot: '#1b86f2', thumbBg: '#dbeeff' },
  { dot: '#ff9d18', thumbBg: '#fff0d0' },
  { dot: '#3bb273', thumbBg: '#d2f0e1' },
  { dot: '#7c3aed', thumbBg: '#e5dcff' },
  { dot: '#db2777', thumbBg: '#fad4e8' },
  { dot: '#0891b2', thumbBg: '#cff0f7' },
  { dot: '#e5534b', thumbBg: '#fcd5d3' },
  { dot: '#4f46e5', thumbBg: '#dde2ff' },
] as const;

export type CategoryColor = (typeof CATEGORY_PALETTE)[number];

/** Stable colour per category for one list of categories. */
export function categoryColors(categories: string[]): (category: string) => CategoryColor {
  const order = [...new Set(categories)];
  return (category) => CATEGORY_PALETTE[Math.max(order.indexOf(category), 0) % CATEGORY_PALETTE.length]!;
}

/** XP granted for completing a course; hard-coded in the frontend for now (defect A11). */
export const COURSE_XP = 200;
