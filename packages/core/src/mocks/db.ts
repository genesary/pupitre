import type { CourseSession, MarkdownPattern, Role, SessionBooking, XpEvent } from '../lib/api-types';
import type { CourseDefinition, PathDefinition } from './model';
import { readItem, writeItem } from '@pupitre/ui/lib/storage';

// Learner state of the mock backend: what user-service and course-service keep
// in PostgreSQL. Stored in sessionStorage, so it survives the page reloads of
// the sign-in flow and resets with the tab (docs/specs/006-mock-mode.md).

const STORAGE_KEY = 'pupitre.mock.db';

export type QuizProgress = { bestScore: number; maxScore: number; passed: boolean; attempts: number };
export type QuestionAttempt = { attempts: number; until: number; locked: boolean };

type MockDb = {
  /** userId → course slug → enrolledAt */
  enrollments: Record<string, Record<string, string>>;
  /** userId → course slug → viewed module slugs */
  viewed: Record<string, Record<string, string[]>>;
  /** userId → course slug → module index → quiz progress */
  quizzes: Record<string, Record<string, Record<number, QuizProgress>>>;
  /** "userId|slug|index" → question id → attempt state */
  attempts: Record<string, Record<string, QuestionAttempt>>;
  /** userId → bookings */
  bookings: Record<string, SessionBooking[]>;
  /** userId → course slug → earnedAt */
  badges: Record<string, Record<string, string>>;
  /** userId → course slug → last activity */
  activity: Record<string, Record<string, string>>;
  /** userId → XP events (user_xp_events), oldest first */
  xp: Record<string, XpEvent[]>;
  /** userId → path slug → enrolledAt */
  paths: Record<string, Record<string, string>>;
  /** userId → profile fields changed with PUT /api/auth/profile or by an admin */
  profiles: Record<string, { bio?: string; avatarUrl?: string; role?: Role; isActive?: boolean }>;
  /** users deleted by an admin */
  deletedUsers: string[];
  /** course slug → definition written by an admin, null once deleted */
  courseDefs: Record<string, CourseDefinition['spec'] | null>;
  /** path slug → definition written by an admin, null once deleted */
  pathDefs: Record<string, PathDefinition['spec'] | null>;
  /** markdown patterns once an admin changed them */
  patterns: MarkdownPattern[] | null;
  /** platform_settings rows changed by an admin */
  settings: Record<string, string>;
  /** SSO group name → platform role (group_role_mappings) */
  mappings: Record<string, Role>;
  /** lab checks run by learners (lab_checks), newest last */
  labChecks: LabCheckRecord[];
  /** course slug → sessions, once a trainer changed them */
  sessions: Record<string, CourseSession[]>;
  /** groups and their members (user-service groups, user_groups) */
  groups: MockGroup[];
  /** "slug|sessionId|userId" → present */
  presence: Record<string, boolean>;
};

export type LabCheckRecord = {
  id: number;
  username: string;
  courseSlug: string;
  moduleIndex: number;
  moduleName: string;
  allow: boolean;
  violations: string[] | null;
  verified: boolean;
  checkedAt: string;
};

export type MockGroup = {
  id: string;
  name: string;
  source: string;
  parentId: string;
  /** Platform role mapped to an identity-provider group (group_role_mappings). */
  mappedRole?: string;
  memberIds: string[];
  courseSlugs: string[];
};

const ALICE = '7a1c3e52-5f0b-4c2e-9d61-0b8f1a2c3d41';
const ADMIN = '0f6e2d18-3b7c-4a95-8e24-c1d3b5a7f960';
const TRAP = '9c8b7a65-4d3e-4f21-b0a9-8e7d6c5b4a39';
const MARC = '2b9d4f61-8c3a-4e7b-a152-6d0e9f8a7b32';

/**
 * marc manages « Promo DevOps 2026 » (alice and the trap account) and its
 * subgroup; « everyone » holds every user and is never part of a scope.
 */
const GROUPS: MockGroup[] = [
  { id: 'a0e8f4c2-0000-4000-8000-000000000001', name: 'everyone', source: 'local', parentId: '', memberIds: [ALICE, MARC, ADMIN, TRAP], courseSlugs: [] },
  {
    id: 'b3f1c2d4-0a1b-4c5d-8e9f-101112131415',
    name: 'Promo DevOps 2026',
    source: 'local',
    parentId: '',
    memberIds: [ALICE, TRAP, MARC],
    courseSlugs: ['docker-fundamentals', 'kubernetes-basics', 'gitlab-cicd'],
  },
  {
    id: 'd5b3e4f6-2c3d-4e7f-8a1b-222324252627',
    name: 'Groupe A',
    source: 'local',
    parentId: 'b3f1c2d4-0a1b-4c5d-8e9f-101112131415',
    memberIds: [ALICE, MARC],
    courseSlugs: [],
  },
  {
    id: 'c4a2d3e5-1b2c-4d6e-9f0a-161718192021',
    name: 'equipe-plateforme',
    source: 'keycloak',
    parentId: '',
    mappedRole: 'student',
    memberIds: [ALICE],
    courseSlugs: [],
  },
];
const daysAgo = (days: number) => new Date(Date.now() - days * 86_400_000).toISOString();

const LINUX_LESSONS = [
  'navigation-et-système-de-fichiers',
  'utilisateurs--groupes-et-permissions',
  'processus--services-et-réseau',
];

/**
 * alice has finished linux-intro and started two courses and a path, so every
 * screen has content. admin and the trap account hold badges too: the
 * leaderboard then shows the trap account's payloads (spec 003).
 */
function seed(): MockDb {
  const xp = (source: string, sourceSlug: string, amount: number, days: number): XpEvent => ({
    source,
    sourceSlug,
    amount,
    earnedAt: daysAgo(days),
  });
  return {
    enrollments: {
      [ALICE]: { 'kubernetes-basics': daysAgo(12), 'pupitre-demo': daysAgo(2), 'linux-intro': daysAgo(30) },
      [ADMIN]: { 'linux-intro': daysAgo(40), 'docker-fundamentals': daysAgo(35) },
      [TRAP]: { 'linux-intro': daysAgo(25) },
    },
    viewed: {
      [ALICE]: {
        'kubernetes-basics': ['concepts-fondamentaux'],
        'pupitre-demo': [],
        'linux-intro': LINUX_LESSONS,
      },
    },
    quizzes: {
      [ALICE]: { 'linux-intro': { 3: { bestScore: 5, maxScore: 5, passed: true, attempts: 1 } } },
    },
    attempts: {},
    // Two participants for the attendance page of the next Paris session.
    bookings: {
      [TRAP]: [{ courseSlug: 'atelier-kubernetes', sessionId: 'paris-1', bookedAt: daysAgo(3) }],
      [ADMIN]: [{ courseSlug: 'atelier-kubernetes', sessionId: 'paris-1', bookedAt: daysAgo(4) }],
    },
    badges: {
      [ALICE]: { 'linux-intro': daysAgo(20) },
      [ADMIN]: { 'linux-intro': daysAgo(38), 'docker-fundamentals': daysAgo(30) },
      [TRAP]: { 'linux-intro': daysAgo(22) },
    },
    activity: { [ALICE]: { 'kubernetes-basics': daysAgo(1) } },
    xp: {
      [ALICE]: [
        ...LINUX_LESSONS.map((lesson, i) => xp('lesson', `linux-intro/${lesson}`, 10, 24 - i)),
        xp('module', 'linux-intro/quiz-introduction-à-linux', 25, 20),
        xp('course', 'linux-intro', 200, 20),
        xp('lesson', 'kubernetes-basics/concepts-fondamentaux', 10, 1),
      ],
    },
    paths: { [ALICE]: { 'parcours-devops': daysAgo(10), 'parcours-competences-conteneurs': daysAgo(3) } },
    profiles: {},
    sessions: {},
    groups: structuredClone(GROUPS),
    presence: {},
    deletedUsers: [],
    courseDefs: {},
    pathDefs: {},
    patterns: null,
    settings: {},
    mappings: { 'equipe-plateforme': 'student' },
    labChecks: [],
  };
}

let cache: MockDb | null = null;

export function db(): MockDb {
  if (cache) return cache;
  try {
    const raw = readItem('session', STORAGE_KEY);
    // A state saved by an older version lacks the newer tables: they start seeded.
    cache = raw ? { ...seed(), ...(JSON.parse(raw) as Partial<MockDb>) } : seed();
  } catch {
    cache = seed();
  }
  return cache;
}

export function save() {
  if (cache) writeItem('session', STORAGE_KEY, JSON.stringify(cache));
}

/** Test helper: back to the seeded state. */
export function resetDb() {
  cache = seed();
  save();
}

export function bucket<T>(map: Record<string, Record<string, T>>, key: string): Record<string, T> {
  map[key] ??= {};
  return map[key];
}
