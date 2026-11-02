import { http, HttpResponse } from 'msw';

import type { CheckResult, EarnedBadge, QuizAnswer, SkillLevel, UserPublic } from '../lib/api-types';

import {
  canSeeCourse,
  completedCourses,
  courses,
  findCourse,
  isEnrolled,
  listModules,
  markViewed,
  moduleDetail,
  myCourses,
  scoreQuiz,
  sessionsOf,
  toCourseResponse,
  totalXp,
  visibleModules,
} from './course-logic';
import { bucket, db, save } from './db';
import { MOCK_PATTERNS } from './fixtures/patterns';
import { apiError, currentAccount } from './http';
import type { CourseDefinition } from './model';

// Routes of the learner flow (batch 1), mirroring course-service and
// user-service. Every write is saved to the mock state.

/** modules.go — ensureModuleAccess: auto-enrolment in public courses, prerequisites. */
function moduleAccess(course: CourseDefinition, user: UserPublic): Response | null {
  if (user.role === 'admin') return null;
  if (!isEnrolled(user.id, course.slug)) {
    if (!course.spec.public) return apiError(403, 'Enroll in this course to access it');
    bucket(db().enrollments, user.id)[course.slug] = new Date().toISOString();
    save();
  }
  const unmet = (course.spec.prerequisites ?? []).some((prereq) => !db().badges[user.id]?.[prereq.course]);
  return unmet ? apiError(403, 'Complete prerequisite courses first') : null;
}

function bookingCount(slug: string, sessionId: string): number {
  return Object.values(db().bookings).filter((list) => list.some((b) => b.courseSlug === slug && b.sessionId === sessionId)).length;
}

/** badges.go — enrichBadgeRows: one entry per earned badge, with its number of holders. */
export function badgesOf(userId: string): EarnedBadge[] {
  const earned = db().badges[userId] ?? {};
  return Object.entries(earned).flatMap(([slug, earnedAt]) => {
    const course = findCourse(slug);
    if (!course?.spec.badge) return [];
    const earnedBy = Object.values(db().badges).filter((badges) => badges[slug]).length;
    return [{ courseSlug: slug, courseTitle: course.spec.title, name: course.spec.badge.name, icon: course.spec.badge.icon, earnedAt, earnedBy }];
  });
}

type Resolved = { course: CourseDefinition; user: UserPublic } | { error: Response };

/** Signed-in user and visible course of a /courses/:slug/… route. */
function resolve(request: Request, slug: string | readonly string[] | undefined): Resolved {
  const account = currentAccount(request);
  if (!account) return { error: apiError(401, 'Missing Authorization header') };
  const course = findCourse(String(slug));
  if (!course || !canSeeCourse(course, account.user)) {
    return { error: apiError(404, 'Course not found') };
  }
  return { course, user: account.user };
}

// Deterministic lab checks: a step fails on its first check, then passes, so
// both outcomes can be seen.
const stepChecks = new Map<string, number>();

function labCheck(key: string, label: string): CheckResult {
  const count = (stepChecks.get(key) ?? 0) + 1;
  stepChecks.set(key, count);
  return count === 1
    ? { allow: false, violations: [`${label} : pas encore trouvé (le mock réussit au 2e essai).`] }
    : { allow: true, violations: [] };
}

/** lab_results.go — every check is stored, verified unless the desktop app reported it. */
function recordCheck(user: UserPublic, course: CourseDefinition, index: number, result: CheckResult, verified: boolean) {
  const checks = db().labChecks;
  checks.push({
    id: checks.length + 100,
    username: user.username,
    courseSlug: course.slug,
    moduleIndex: index,
    moduleName: visibleModules(course, user.role === 'admin')[index]?.name ?? '',
    allow: result.allow,
    violations: result.violations ?? null,
    verified,
    checkedAt: new Date().toISOString(),
  });
  save();
}

export const learnerHandlers = [
  // ── course-service ────────────────────────────────────────────────────────
  http.get('/api/courses', ({ request }) => {
    const user = currentAccount(request)?.user ?? null;
    const list = courses().filter((course) => canSeeCourse(course, user)).map(toCourseResponse);
    return HttpResponse.json({ courses: list, total: list.length });
  }),

  http.get('/api/courses/:slug', ({ request, params }) => {
    const user = currentAccount(request)?.user ?? null;
    const course = findCourse(String(params.slug));
    if (!course || !canSeeCourse(course, user)) return apiError(404, 'Course not found');
    return HttpResponse.json(toCourseResponse(course));
  }),

  http.get('/api/courses/:slug/modules', ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const denied = moduleAccess(resolved.course, resolved.user);
    if (denied) return denied;
    return HttpResponse.json({ modules: listModules(resolved.course, resolved.user) });
  }),

  http.get('/api/courses/:slug/modules/:index', ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const denied = moduleAccess(resolved.course, resolved.user);
    if (denied) return denied;
    const detail = moduleDetail(resolved.course, Number(params.index), resolved.user);
    return detail ? HttpResponse.json(detail) : apiError(404, 'Module not found');
  }),

  http.post('/api/courses/:slug/modules/:index/submit', async ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const denied = moduleAccess(resolved.course, resolved.user);
    if (denied) return denied;
    const index = Number(params.index);
    const module = visibleModules(resolved.course, resolved.user.role === 'admin')[index];
    if (!module) return apiError(404, 'Module not found');
    if (module.type !== 'quiz') return apiError(400, 'Module is not a quiz');
    const body = (await request.json()) as { answers?: Record<string, QuizAnswer> };
    const result = scoreQuiz(resolved.course, index, resolved.user, body.answers ?? {});
    save();
    return HttpResponse.json(result.body as Record<string, unknown>, { status: result.status });
  }),

  http.post('/api/courses/:slug/lessons/:lessonSlug/complete', ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const denied = moduleAccess(resolved.course, resolved.user);
    if (denied) return denied;
    markViewed(resolved.course, String(params.lessonSlug), resolved.user);
    save();
    return HttpResponse.json({ message: 'Lesson marked as complete' });
  }),

  http.post('/api/courses/:slug/modules/:index/check', ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const result = labCheck(`${resolved.user.id}|${params.slug}|${params.index}`, 'Le travail');
    recordCheck(resolved.user, resolved.course, Number(params.index), result, true);
    return HttpResponse.json(result);
  }),

  http.post('/api/courses/:slug/modules/:index/steps/:step/check', ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const module = visibleModules(resolved.course, false)[Number(params.index)];
    const step = module?.steps?.[Number(params.step)];
    if (!step) return apiError(404, 'Step not found');
    const result = labCheck(`${resolved.user.id}|${params.slug}|${params.index}|${params.step}`, step.title);
    recordCheck(resolved.user, resolved.course, Number(params.index), result, true);
    return HttpResponse.json(result);
  }),

  http.post('/api/courses/:slug/modules/:index/record', async ({ request, params }) => {
    const resolved = resolve(request, params.slug);
    if ('error' in resolved) return resolved.error;
    const result = (await request.json()) as CheckResult;
    recordCheck(resolved.user, resolved.course, Number(params.index), result, false);
    return HttpResponse.json(result);
  }),

  // ── user-service ──────────────────────────────────────────────────────────
  http.get('/api/my/courses', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    return HttpResponse.json({ courses: myCourses(account.user) });
  }),

  // skill_level.go: a skill has a row once a course teaching it is completed;
  // its level is the highest difficulty completed, « maître » once all are.
  http.get('/api/my/skills', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    const done = completedCourses(account.user);
    const order = ['beginner', 'intermediate', 'advanced'];
    const skills = new Map<string, SkillLevel>();
    for (const course of courses().filter((c) => c.spec.public && !c.spec.hidden)) {
      const response = toCourseResponse(course);
      for (const skill of response.skills ?? []) {
        const entry = skills.get(skill) ?? { skill, level: '', completedCourses: 0, totalCourses: 0 };
        entry.totalCourses += 1;
        if (done.has(course.slug)) {
          entry.completedCourses += 1;
          if (order.indexOf(response.difficulty) > order.indexOf(entry.level)) entry.level = response.difficulty;
        }
        skills.set(skill, entry);
      }
    }
    const rows = [...skills.values()]
      .filter((entry) => entry.completedCourses > 0)
      .map((entry) => ({
        ...entry,
        level: entry.completedCourses >= entry.totalCourses ? 'maître' : entry.level || 'beginner',
      }))
      .sort((a, b) => a.skill.localeCompare(b.skill));
    return HttpResponse.json({ skills: rows });
  }),

  http.get('/api/my/badges', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    return HttpResponse.json({ badges: badgesOf(account.user.id) });
  }),

  http.get('/api/my/session-bookings', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    return HttpResponse.json({ bookings: db().bookings[account.user.id] ?? [] });
  }),

  http.post('/api/enrollments/:slug', ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    const course = findCourse(String(params.slug));
    if (!course || !course.spec.public) return apiError(404, 'Course not found');
    const required = course.spec.xpRequired ?? 0;
    if (totalXp(account.user.id) < required) {
      return apiError(403, `Ce cours demande ${required} XP. Terminez d'autres cours pour en gagner.`);
    }
    bucket(db().enrollments, account.user.id)[course.slug] ??= new Date().toISOString();
    save();
    return HttpResponse.json({ message: 'Enrolled' }, { status: 201 });
  }),

  http.delete('/api/enrollments/:slug', ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    delete db().enrollments[account.user.id]?.[String(params.slug)];
    save();
    return HttpResponse.json({ message: 'Unenrolled' });
  }),

  // sessions.go — BookSession: 409 once a limited session is full, then 204.
  http.post('/api/session-bookings/:slug/:sessionId', ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    const course = findCourse(String(params.slug));
    const session = course && sessionsOf(course)?.find((s) => s.id === params.sessionId);
    const list = (db().bookings[account.user.id] ??= []);
    const already = list.some((b) => b.courseSlug === params.slug && b.sessionId === params.sessionId);
    if (!already && (session?.capacity ?? 0) > 0 && bookingCount(String(params.slug), String(params.sessionId)) >= session!.capacity!) {
      return apiError(409, 'Session is full');
    }
    if (!already) {
      list.push({ courseSlug: String(params.slug), sessionId: String(params.sessionId), bookedAt: new Date().toISOString() });
    }
    save();
    return new HttpResponse(null, { status: 204 });
  }),

  http.delete('/api/session-bookings/:slug/:sessionId', ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    db().bookings[account.user.id] = (db().bookings[account.user.id] ?? []).filter(
      (b) => !(b.courseSlug === params.slug && b.sessionId === params.sessionId),
    );
    save();
    return new HttpResponse(null, { status: 204 });
  }),

  http.get('/api/session-bookings/:slug/:sessionId/count', ({ request, params }) => {
    if (!currentAccount(request)) return apiError(401, 'Missing Authorization header');
    return HttpResponse.json({ count: bookingCount(String(params.slug), String(params.sessionId)) });
  }),

  http.get('/api/patterns', ({ request }) => {
    if (!currentAccount(request)) return apiError(401, 'Missing Authorization header');
    return HttpResponse.json({ patterns: db().patterns ?? MOCK_PATTERNS });
  }),
];
