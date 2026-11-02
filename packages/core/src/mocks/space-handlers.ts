import { http, HttpResponse } from 'msw';

import type { CourseSession, MyGroup, MyPath, PathMemberStatus, ProgressStatus, UserPublic } from '../lib/api-types';

import {
  completedCourses,
  findCourse,
  isModuleDone,
  paths as pathDefinitions,
  sessionsOf,
  skillModules,
} from './course-logic';
import { accounts, findAccount } from './data';
import { db, save, type MockGroup } from './db';
import { apiError, currentAccount } from './http';
import type { PathDefinition } from './model';
import { badgesOf } from './learner-handlers';

// Routes of the learner space (batch 2, docs/specs/020, 021, 022), mirroring
// user-service (paths.go, skills.go, xp.go, my_groups.go, badges.go, auth.go)
// and course-service (sessions_admin.go).

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** skills.go — a module is completed, available after a completed one, else locked. */
function moduleStatuses(user: UserPublic, skill: string) {
  let previousDone = true;
  return skillModules(skill).map((module) => {
    const done = isModuleDone(user, module.courseSlug, module);
    const status: ProgressStatus = done ? 'completed' : previousDone ? 'available' : 'locked';
    previousDone = done;
    return { ...module, status };
  });
}

/** skills.go — skillIsCompleted: every quiz passed and every lab viewed, at least one of them. */
function skillCompleted(user: UserPublic, skill: string): boolean {
  const assessable = skillModules(skill).filter((m) => m.type === 'quiz' || m.type === 'lab');
  return assessable.length > 0 && assessable.every((m) => isModuleDone(user, m.courseSlug, m));
}

/** paths.go — buildCourseStatuses, used for the skills of a skill path too. */
export function orderedStatuses(slugs: string[], isDone: (slug: string) => boolean): PathMemberStatus[] {
  const out: PathMemberStatus[] = [];
  slugs.forEach((slug, index) => {
    const status: ProgressStatus = isDone(slug)
      ? 'completed'
      : index === 0 || out[index - 1]!.status === 'completed'
        ? 'available'
        : 'locked';
    out.push({ slug, status });
  });
  return out;
}

/** paths.go — the course (or skill) statuses of one path for one user. */
export function pathMembers(user: UserPublic, path: PathDefinition, done = completedCourses(user)): PathMemberStatus[] {
  return path.spec.kind === 'skill'
    ? orderedStatuses(path.spec.skills ?? [], (skill) => skillCompleted(user, skill))
    : orderedStatuses(path.spec.courses ?? [], (course) => done.has(course));
}

export function groupDepth(groups: MockGroup[], group: MockGroup): number {
  const parent = groups.find((g) => g.id === group.parentId);
  return parent ? groupDepth(groups, parent) + 1 : 0;
}

function isTrainer(user: UserPublic) {
  return user.role === 'admin' || user.role === 'manager';
}

type SessionBody = { title?: string; date?: string; location?: string; capacity?: number };

function sessionFromBody(id: string, body: SessionBody): CourseSession {
  return { id, title: body.title ?? '', date: body.date ?? '', location: body.location ?? '', capacity: body.capacity ?? 0 };
}

export const spaceHandlers = [
  http.get('/api/my/paths', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    const user = account.user;
    const done = completedCourses(user);
    const enrolled = db().paths[user.id] ?? {};
    const paths: MyPath[] = Object.entries(enrolled).flatMap(([slug, enrolledAt]): MyPath[] => {
      const path = pathDefinitions().find((p) => p.slug === slug);
      if (!path) return [{ slug, title: slug, enrolledAt, courses: [] }];
      const { spec } = path;
      const members = pathMembers(user, path, done);
      return [{ slug, title: spec.title, description: spec.description, kind: spec.kind, enrolledAt, courses: members, skills: spec.skills }];
    });
    return HttpResponse.json({ paths });
  }),

  http.get('/api/my/skills/:slug', ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    const skill = String(params.slug);
    if (!/^[a-z0-9][a-z0-9-]*$/.test(skill)) return apiError(400, 'Invalid skill slug');
    return HttpResponse.json({ skill, modules: moduleStatuses(account.user, skill) });
  }),

  http.get('/api/my/xp', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    // XPRepository.History: newest first.
    const history = [...(db().xp[account.user.id] ?? [])].sort((a, b) => b.earnedAt.localeCompare(a.earnedAt));
    const total = history.reduce((sum, e) => sum + e.amount, 0);
    const thresholds = [0, 200, 600, 1500, 3500];
    const names = ['Novice', 'Apprenant', 'Confirmé', 'Expert', 'Maître'];
    const level = thresholds.filter((threshold) => total >= threshold).length;
    return HttpResponse.json({ total, level, levelName: names[level - 1], history });
  }),

  // my_groups.go: every group of the user but « everyone », with its courses.
  http.get('/api/my/groups', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    const all = db().groups;
    const groups: MyGroup[] = all
      .filter((g) => g.name !== 'everyone' && g.memberIds.includes(account.user.id))
      .map((g) => ({
        id: g.id,
        name: g.name,
        source: g.source,
        parentId: g.parentId,
        depth: groupDepth(all, g),
        mappedRole: g.mappedRole,
        courseSlugs: g.courseSlugs,
      }));
    return HttpResponse.json({ groups, total: groups.length });
  }),

  // ── Public routes ─────────────────────────────────────────────────────────
  http.get('/api/leaderboard', () => {
    const rows = accounts().map(({ user }) => ({ user, badges: badgesOf(user.id) }))
      .filter((row) => row.badges.length > 0)
      .sort((a, b) => b.badges.length - a.badges.length || a.user.username.localeCompare(b.user.username));
    const leaderboard = rows.map((row, index) => {
      const user = findAccount(row.user.id)!.user;
      return {
        rank: index + 1,
        userId: user.id,
        username: user.username,
        avatarUrl: user.avatarUrl,
        count: row.badges.length,
        icons: row.badges.map((badge) => badge.icon || '🏅'),
      };
    });
    return HttpResponse.json({ leaderboard });
  }),

  http.get('/api/users/:id', ({ params }) => {
    const id = String(params.id);
    if (!UUID.test(id)) return apiError(400, 'Invalid user ID');
    const account = findAccount(id);
    if (!account) return apiError(404, 'User not found');
    return HttpResponse.json({ id, username: account.user.username, avatarUrl: account.user.avatarUrl });
  }),

  http.get('/api/users/:id/badges', ({ params }) => HttpResponse.json({ badges: badgesOf(String(params.id)) })),

  http.get('/api/badges/:slug', ({ params }) => {
    const slug = String(params.slug);
    const earnedBy = Object.values(db().badges).filter((badges) => badges[slug]).length;
    return HttpResponse.json({ courseSlug: slug, earnedBy });
  }),

  // ── Profile (auth.go — UpdateProfile) ─────────────────────────────────────
  http.put('/api/auth/profile', async ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    let body: { bio?: unknown; avatarUrl?: unknown; username?: unknown };
    try {
      body = (await request.json()) as typeof body;
    } catch {
      return apiError(400, 'Invalid JSON');
    }
    const changes = (db().profiles[account.user.id] ??= {});
    if (typeof body.bio === 'string') changes.bio = body.bio;
    if (typeof body.avatarUrl === 'string') changes.avatarUrl = body.avatarUrl;
    save();
    return HttpResponse.json(findAccount(account.user.id)!.user);
  }),

  // ── Sessions (course-service sessions_admin.go, admin or manager) ────────
  http.post('/api/admin/courses/:slug/sessions', async ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    if (!isTrainer(account.user)) return apiError(403, 'Forbidden');
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    const body = (await request.json()) as SessionBody;
    if (!body.title || !body.date) return apiError(400, 'title and date are required');
    const id = `sess-${crypto.randomUUID().replace(/-/g, '').slice(0, 16)}`;
    db().sessions[course.slug] = [...(sessionsOf(course) ?? []), sessionFromBody(id, body)];
    save();
    return HttpResponse.json({ id }, { status: 201 });
  }),

  http.put('/api/admin/courses/:slug/sessions/:sessionId', async ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    if (!isTrainer(account.user)) return apiError(403, 'Forbidden');
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    const sessions = sessionsOf(course) ?? [];
    const id = String(params.sessionId);
    if (!sessions.some((s) => s.id === id)) return apiError(404, 'Session not found');
    const body = (await request.json()) as SessionBody;
    db().sessions[course.slug] = sessions.map((s) => (s.id === id ? sessionFromBody(id, body) : s));
    save();
    return HttpResponse.json({ id });
  }),

  http.delete('/api/admin/courses/:slug/sessions/:sessionId', ({ request, params }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Missing Authorization header');
    if (!isTrainer(account.user)) return apiError(403, 'Forbidden');
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    const sessions = sessionsOf(course) ?? [];
    const id = String(params.sessionId);
    if (!sessions.some((s) => s.id === id)) return apiError(404, 'Session not found');
    db().sessions[course.slug] = sessions.filter((s) => s.id !== id);
    save();
    return HttpResponse.json({ message: 'Session deleted' });
  }),
];
