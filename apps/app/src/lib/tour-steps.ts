import type { Role } from '@pupitre/core/lib/api-types';

import { TOUR_STEP_TEXTS } from '@/lib/messages/tour';

// Steps of the guided tour and of the first lesson tip
// (docs/specs/007-guided-tour.md, "Steps" and "First lesson tip").

/** Raised only when the tour changes enough to be offered again. */
export const TOUR_VERSION = 1;

export type TourStep = {
  id: string;
  /** Page of the step; absent: the current page. `:slug` is the course of the `course-card` step. */
  path?: string;
  /** `data-tour` values, the first visible one is highlighted; absent: a centred bubble. */
  targets?: string[];
  /** Skipped when its target does not show up. */
  optional?: boolean;
  /** `data-tour` the page sets once its data loaded: an optional step without its target is skipped then. */
  loaded?: string;
  /** Needs the course of the `course-card` step: skipped without it. */
  needsCourse?: boolean;
  /** Roles that see the step; absent: every role. */
  roles?: Role[];
  title: string;
  text: string;
};

type StepDef = Omit<TourStep, 'title' | 'text'>;

const COURSE_PATH = '/courses/:slug';

const TOUR: StepDef[] = [
  { id: 'catalogue', path: '/courses', targets: ['nav-catalogue'] },
  { id: 'search', path: '/courses', targets: ['catalogue-search'] },
  { id: 'course-card', path: '/courses', targets: ['course-card'], optional: true, loaded: 'catalogue-loaded' },
  { id: 'continue', path: '/courses', targets: ['continue-hero'], optional: true, loaded: 'catalogue-loaded' },
  { id: 'course-page', path: COURSE_PATH, targets: ['page-title'], needsCourse: true },
  { id: 'course-modules', path: COURSE_PATH, targets: ['course-modules'], needsCourse: true },
  { id: 'course-action', path: COURSE_PATH, targets: ['course-action'], needsCourse: true },
  { id: 'my-courses', path: '/my-courses', targets: ['nav-my-courses'] },
  { id: 'menu', targets: ['nav-menu'] },
  { id: 'paths', path: '/my-paths', targets: ['page-title'] },
  { id: 'groups', path: '/my-groups', targets: ['page-title'] },
  { id: 'badges', path: '/my-badges', targets: ['page-title'] },
  { id: 'xp', path: '/my-xp', targets: ['page-title'] },
  { id: 'leaderboard', path: '/leaderboard', targets: ['page-title'] },
  { id: 'skills', path: '/my-skills', targets: ['page-title'] },
  { id: 'agenda', path: '/agenda', targets: ['page-title'] },
  { id: 'profile', targets: ['nav-profile'] },
  { id: 'theme', targets: ['nav-theme'] },
  { id: 'manager', targets: ['nav-manager'], roles: ['manager'] },
  { id: 'admin', targets: ['nav-admin'], roles: ['admin'] },
  { id: 'end' },
];

const LESSON_TIP: StepDef[] = [
  { id: 'lesson-modules', targets: ['lesson-modules', 'lesson-modules-mobile'] },
  { id: 'lesson-content', targets: ['page-title'] },
  { id: 'lesson-nav', targets: ['lesson-nav'] },
];

function withTexts(steps: StepDef[]): TourStep[] {
  return steps.map((step) => ({ ...step, ...TOUR_STEP_TEXTS[step.id]! }));
}

/** The tour steps a user of `role` sees. */
export function tourSteps(role: Role): TourStep[] {
  return withTexts(TOUR.filter((step) => !step.roles || step.roles.includes(role)));
}

export const LESSON_TIP_STEPS: TourStep[] = withTexts(LESSON_TIP);

/** The page of a step, or null to stay on the current page. */
export function stepPath(step: TourStep, courseSlug: string | null): string | null {
  if (!step.path) return null;
  return step.path === COURSE_PATH ? `/courses/${courseSlug}` : step.path;
}

/** Lessons and quizzes: the tour is never offered over the content a user came to read. */
export function isReadingPage(pathname: string): boolean {
  return /^\/courses\/[^/]+\/(lessons|quiz)\//.test(pathname);
}
