import { http, HttpResponse } from 'msw';

import type { ExportCategory, PathEnrolledUser, ScopeUser, UserPublic } from '../lib/api-types';

import { findCourse, myCourses, paths } from './course-logic';
import { accounts, findAccount } from './data';
import { bucket, db, save, type MockGroup } from './db';
import { apiError, currentAccount } from './http';
import { groupDepth, pathMembers } from './space-handlers';

// Routes of batch 3 (docs/specs/030, 022 attendance, 047 exports), mirroring
// user-service manager.go, paths.go, sessions.go, exports.go and the lab-check
// exports of course-service. The scope of a manager is the members of the
// groups they belong to, « everyone » excepted (manager.go — managerScopeIDs).

const DEFAULT_GROUP = 'everyone';

type Guarded = { user: UserPublic } | { error: Response };

function requireRole(request: Request, roles: string[]): Guarded {
  const account = currentAccount(request);
  if (!account) return { error: apiError(401, 'Missing Authorization header') };
  if (!roles.includes(account.user.role)) return { error: apiError(403, 'Forbidden') };
  return { user: account.user };
}

function scopeGroups(managerId: string): MockGroup[] {
  return db().groups.filter((g) => g.name !== DEFAULT_GROUP && g.memberIds.includes(managerId));
}

function scopeUserIds(managerId: string): Set<string> {
  return new Set(scopeGroups(managerId).flatMap((g) => g.memberIds));
}

/** repository/user.go — AdminUserRow */
function toScopeUser(id: string): ScopeUser | null {
  const user = findAccount(id)?.user;
  if (!user) return null;
  return {
    id: user.id,
    username: user.username,
    email: user.email,
    role: user.role,
    isActive: user.isActive,
    avatarUrl: user.avatarUrl,
    bio: user.bio,
    authProvider: user.authProvider,
    createdAt: user.createdAt,
    enrolledCourses: Object.keys(db().enrollments[id] ?? {}).length,
  };
}

function ownedGroup(managerId: string, groupId: string): MockGroup | undefined {
  return scopeGroups(managerId).find((g) => g.id === groupId);
}

async function userIdOf(request: Request): Promise<string> {
  try {
    const body = (await request.json()) as { userId?: unknown };
    return typeof body.userId === 'string' ? body.userId : '';
  } catch {
    return '';
  }
}

function participants(slug: string, sessionId: string) {
  return Object.entries(db().bookings).flatMap(([userId, bookings]) => {
    const booking = bookings.find((b) => b.courseSlug === slug && b.sessionId === sessionId);
    const user = findAccount(userId)?.user;
    if (!booking || !user) return [];
    return [
      {
        userId,
        username: user.username,
        email: user.email,
        bookedAt: booking.bookedAt,
        present: Boolean(db().presence[`${slug}|${sessionId}|${userId}`]),
      },
    ];
  });
}

// ── CSV exports ─────────────────────────────────────────────────────────────

type ExportDef = ExportCategory & { rows: () => Record<string, string>[] };

const USER_EXPORTS: ExportDef[] = [
  {
    id: 'users',
    label: 'Utilisateurs',
    fields: [
      { id: 'id', label: 'ID' },
      { id: 'email', label: 'Email' },
      { id: 'username', label: 'Pseudo' },
      { id: 'role', label: 'Rôle' },
      { id: 'isactive', label: 'Actif' },
      { id: 'created_at', label: 'Inscrit le' },
    ],
    defaultFields: ['email', 'username', 'role', 'created_at'],
    filters: [
      { id: 'created_from', label: 'Inscrit depuis', type: 'date' },
      {
        id: 'role',
        label: 'Rôle',
        type: 'select',
        options: [
          { value: 'student', label: 'Étudiant' },
          { value: 'manager', label: 'Manager' },
          { value: 'admin', label: 'Admin' },
        ],
      },
    ],
    rows: () =>
      accounts().map(({ user }) => {
        const current = findAccount(user.id)!.user;
        return {
          id: current.id,
          email: current.email,
          username: current.username,
          role: current.role,
          isactive: String(current.isActive),
          created_at: current.createdAt,
        };
      }),
  },
  {
    id: 'enrollments',
    label: 'Inscriptions aux cours',
    fields: [
      { id: 'userid', label: 'ID utilisateur' },
      { id: 'email', label: 'Email' },
      { id: 'username', label: 'Pseudo' },
      { id: 'courseslug', label: 'Slug cours' },
      { id: 'enrolledat', label: 'Inscrit le' },
    ],
    defaultFields: ['email', 'username', 'courseslug', 'enrolledat'],
    filters: [
      { id: 'enrolled_from', label: 'Inscrit depuis', type: 'date' },
      { id: 'course_slug', label: 'Cours (slug)', type: 'text' },
    ],
    rows: () =>
      Object.entries(db().enrollments).flatMap(([userId, courses]) => {
        const user = findAccount(userId)?.user;
        if (!user) return [];
        return Object.entries(courses).map(([slug, enrolledAt]) => ({
          userid: userId,
          email: user.email,
          username: user.username,
          courseslug: slug,
          enrolledat: enrolledAt,
        }));
      }),
  },
];

const LAB_EXPORTS: ExportDef[] = [
  {
    id: 'lab_checks',
    label: 'Lab Checks',
    fields: [
      { id: 'username', label: "Nom d'utilisateur" },
      { id: 'course_slug', label: 'Slug cours' },
      { id: 'module_name', label: 'Nom module' },
      { id: 'allow', label: 'Résultat' },
      { id: 'checked_at', label: 'Vérifié le' },
    ],
    defaultFields: ['username', 'course_slug', 'module_name', 'allow', 'checked_at'],
    filters: [
      { id: 'course_slug', label: 'Cours (slug)', type: 'text' },
      {
        id: 'allow',
        label: 'Résultat',
        type: 'select',
        options: [
          { value: 'true', label: 'Réussi' },
          { value: 'false', label: 'Échoué' },
        ],
      },
    ],
    rows: () => [
      { username: 'alice', course_slug: 'pupitre-demo', module_name: 'Lab GitLab', allow: 'false', checked_at: '2026-10-18T10:02:00Z' },
      { username: 'alice', course_slug: 'pupitre-demo', module_name: 'Lab GitLab', allow: 'true', checked_at: '2026-10-18T10:09:00Z' },
    ],
  },
];

/** The filters the mock applies: equality on a field of the same name, and « …_from » dates. */
function filtered(def: ExportDef, filters: Record<string, string>) {
  return def.rows().filter((row) =>
    Object.entries(filters).every(([id, value]) => {
      if (!value) return true;
      if (id.endsWith('_from')) {
        const field = Object.keys(row).find((key) => key.includes('at'));
        return !field || (row[field] ?? '') >= value;
      }
      const key = id === 'course_slug' && 'courseslug' in row ? 'courseslug' : id;
      return !(key in row) || row[key] === value;
    }),
  );
}

type ExportBody = { category?: string; fields?: string[]; filters?: Record<string, string> };

function exportHandlers(base: string, defs: ExportDef[], roles: string[]) {
  const resolve = async (request: Request) => {
    const guard = requireRole(request, roles);
    if ('error' in guard) return { error: guard.error };
    const body = (await request.json().catch(() => ({}))) as ExportBody;
    const def = defs.find((d) => d.id === body.category);
    if (!def) return { error: apiError(400, body.category ? `unknown category: ${body.category}` : 'Catégorie requise') };
    const fields = (body.fields?.length ? body.fields : def.defaultFields).filter((f) => def.fields.some((d) => d.id === f));
    // A manager exports the members of their groups only (exports.go — exportScope).
    const scope = guard.user.role === 'manager' ? scopeUserIds(guard.user.id) : null;
    const rows = filtered(def, body.filters ?? {}).filter((row) => !scope || scope.has(row.id ?? row.userid ?? ''));
    return { def, fields, rows };
  };
  return [
    http.get(`${base}/categories`, ({ request }) => {
      const guard = requireRole(request, roles);
      if ('error' in guard) return guard.error;
      return HttpResponse.json({ categories: defs.map(({ id, label, fields, defaultFields, filters }) => ({ id, label, fields, defaultFields, filters })) });
    }),
    http.post(`${base}/preview`, async ({ request }) => {
      const result = await resolve(request);
      if ('error' in result) return result.error;
      const rows = result.rows.slice(0, 10).map((row) => Object.fromEntries(result.fields.map((f) => [f, row[f] ?? ''])));
      return HttpResponse.json({ headers: result.fields, rows, totalCount: result.rows.length });
    }),
    http.post(`${base}/download`, async ({ request }) => {
      const result = await resolve(request);
      if ('error' in result) return result.error;
      const cell = (value: string) => (/[;"\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value);
      // A value that starts like a formula is exported as text (platform/csvsafe, defect B8).
      const safe = (value: string) => (/^[=+\-@\t\r]/.test(value) ? `'${value}` : value);
      const lines = [
        result.fields.map((f) => cell(result.def.fields.find((d) => d.id === f)?.label ?? f)).join(';'),
        ...result.rows.map((row) => result.fields.map((f) => cell(safe(row[f] ?? ''))).join(';')),
      ];
      const filename = `${result.def.id}-export-${new Date().toISOString().slice(0, 10)}.csv`;
      const BOM = String.fromCharCode(0xfeff);
      return new HttpResponse(`${BOM}${lines.join('\r\n')}\r\n`, {
        headers: {
          'Content-Type': 'text/csv; charset=utf-8',
          'Content-Disposition': `attachment; filename="${filename}"`,
        },
      });
    }),
  ];
}

export const managerHandlers = [
  // course-service paths.go — ListPaths (public)
  http.get('/api/paths', () =>
    HttpResponse.json({ paths: paths().map((p) => ({ slug: p.slug, ...p.spec, kind: p.spec.kind ?? 'course' })) }),
  ),

  // ── Members ───────────────────────────────────────────────────────────────
  http.get('/api/manager/users', ({ request }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const groups = scopeGroups(guard.user.id);
    const users = [...scopeUserIds(guard.user.id)].flatMap((id) => toScopeUser(id) ?? []);
    return HttpResponse.json({
      users,
      total: users.length,
      groups: groups.map((g) => ({ id: g.id, name: g.name, memberIds: g.memberIds })),
    });
  }),

  http.get('/api/manager/users/:userId/enrollments', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const userId = String(params.userId);
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    const user = findAccount(userId)?.user;
    const enrollments = user
      ? myCourses(user).map((c) => ({
          courseSlug: c.slug,
          completedLabs: c.completedLabs,
          totalScore: c.totalScore,
          lastActivity: c.lastActivity ?? undefined,
        }))
      : [];
    return HttpResponse.json({ enrollments });
  }),

  // PathEnrollmentRow has no json tags: the keys are « Slug » and « EnrolledAt » (defect A22).
  http.get('/api/manager/users/:userId/path-enrollments', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const userId = String(params.userId);
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    const pathEnrollments = Object.entries(db().paths[userId] ?? {}).map(([Slug, EnrolledAt]) => ({ Slug, EnrolledAt }));
    return HttpResponse.json({ pathEnrollments });
  }),

  http.post('/api/manager/enrollments/:slug', async ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const userId = await userIdOf(request);
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    if (!findCourse(String(params.slug))) return apiError(404, 'Course not found');
    bucket(db().enrollments, userId)[String(params.slug)] ??= new Date().toISOString();
    save();
    return HttpResponse.json({ message: 'User enrolled' });
  }),

  http.delete('/api/manager/enrollments/:slug/users/:userId', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const userId = String(params.userId);
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    delete db().enrollments[userId]?.[String(params.slug)];
    save();
    return HttpResponse.json({ message: 'User unenrolled' });
  }),

  // ── Groups ────────────────────────────────────────────────────────────────
  http.get('/api/manager/groups', ({ request }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const all = db().groups;
    const groups = scopeGroups(guard.user.id)
      .map((g) => ({ id: g.id, name: g.name, parentId: g.parentId, depth: groupDepth(all, g), memberCount: g.memberIds.length }))
      .sort((a, b) => a.depth - b.depth || a.name.localeCompare(b.name));
    return HttpResponse.json({ groups });
  }),

  http.post('/api/manager/groups', async ({ request }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const { name } = (await request.json().catch(() => ({}))) as { name?: string };
    if (!name) return apiError(400, 'name required');
    if (db().groups.some((g) => g.name === name)) return apiError(409, 'Group name already exists');
    const id = crypto.randomUUID();
    db().groups.push({ id, name, source: 'local', parentId: '', memberIds: [guard.user.id], courseSlugs: [] });
    save();
    return HttpResponse.json({ id, message: 'Group created' }, { status: 201 });
  }),

  // The manager joins the subgroup, so it stays in their scope (docs/specs/190-backend-defects.md B1).
  http.post('/api/manager/groups/:groupId/subgroups', async ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const parent = ownedGroup(guard.user.id, String(params.groupId));
    if (!parent) return apiError(403, 'Group not in manager scope');
    const { name } = (await request.json().catch(() => ({}))) as { name?: string };
    if (!name) return apiError(400, 'name required');
    if (db().groups.some((g) => g.name === name)) return apiError(409, 'Group name already exists');
    const id = crypto.randomUUID();
    db().groups.push({ id, name, source: 'local', parentId: parent.id, memberIds: [guard.user.id], courseSlugs: [] });
    save();
    return HttpResponse.json({ id, message: 'Group created' }, { status: 201 });
  }),

  http.delete('/api/manager/groups/:groupId', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const group = ownedGroup(guard.user.id, String(params.groupId));
    if (!group) return apiError(403, 'Group not in manager scope');
    db().groups = db().groups.filter((g) => g.id !== group.id && g.parentId !== group.id);
    save();
    return HttpResponse.json({ message: 'Group deleted' });
  }),

  http.get('/api/manager/groups/:groupId/members', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const group = ownedGroup(guard.user.id, String(params.groupId));
    if (!group) return apiError(403, 'Group not in manager scope');
    return HttpResponse.json({ members: group.memberIds.flatMap((id) => toScopeUser(id) ?? []) });
  }),

  // Like the Go handler, only a user already in the manager's scope (manager.go — callerScope).
  http.post('/api/manager/groups/:groupId/members', async ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const group = ownedGroup(guard.user.id, String(params.groupId));
    if (!group) return apiError(403, 'Group not in manager scope');
    const userId = await userIdOf(request);
    if (!userId) return apiError(400, 'userId required');
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    if (!group.memberIds.includes(userId)) group.memberIds.push(userId);
    save();
    return HttpResponse.json({ message: 'Member added' });
  }),

  http.delete('/api/manager/groups/:groupId/members/:userId', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const group = ownedGroup(guard.user.id, String(params.groupId));
    if (!group) return apiError(403, 'Group not in manager scope');
    group.memberIds = group.memberIds.filter((id) => id !== params.userId);
    save();
    return HttpResponse.json({ message: 'Member removed' });
  }),

  // ── Paths ─────────────────────────────────────────────────────────────────
  http.get('/api/manager/paths/:slug/enrollments', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    const scope = scopeUserIds(guard.user.id);
    const users: PathEnrolledUser[] = Object.entries(db().paths).flatMap(([userId, paths]) => {
      const enrolledAt = paths[path.slug];
      const account = findAccount(userId);
      if (!enrolledAt || !scope.has(userId) || !account) return [];
      const courses = pathMembers(account.user, path);
      return [
        {
          userId,
          email: account.user.email,
          role: account.user.role,
          enrolledAt,
          completedCourses: courses.filter((c) => c.status === 'completed').length,
          totalCourses: courses.length,
          courses,
        },
      ];
    });
    return HttpResponse.json({ users });
  }),

  // EnrollWithCourses: the path, and each course of a course path.
  http.post('/api/manager/paths/:slug/enrollments', async ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    const userId = await userIdOf(request);
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    const now = new Date().toISOString();
    bucket(db().paths, userId)[path.slug] ??= now;
    for (const course of path.spec.courses ?? []) bucket(db().enrollments, userId)[course] ??= now;
    save();
    return HttpResponse.json({ message: 'User enrolled' });
  }),

  http.delete('/api/manager/paths/:slug/enrollments/:userId', ({ request, params }) => {
    const guard = requireRole(request, ['manager']);
    if ('error' in guard) return guard.error;
    const userId = String(params.userId);
    if (!scopeUserIds(guard.user.id).has(userId)) return apiError(403, 'User not in manager scope');
    delete db().paths[userId]?.[String(params.slug)];
    save();
    return HttpResponse.json({ message: 'User unenrolled' });
  }),

  // ── Attendance (sessions.go, same handlers under /api/admin and /api/manager) ──
  ...(['admin', 'manager'] as const).flatMap((role) => [
    http.get(`/api/${role}/session-bookings/:slug/:sessionId`, ({ request, params }) => {
      const guard = requireRole(request, [role]);
      if ('error' in guard) return guard.error;
      // A manager sees the participants of their scope only (sessions.go).
      const scope = role === 'manager' ? scopeUserIds(guard.user.id) : null;
      const list = participants(String(params.slug), String(params.sessionId)).filter((p) => !scope || scope.has(p.userId));
      return HttpResponse.json({ participants: list });
    }),
    http.patch(`/api/${role}/session-bookings/:slug/:sessionId/:userId/presence`, async ({ request, params }) => {
      const guard = requireRole(request, [role]);
      if ('error' in guard) return guard.error;
      const body = (await request.json().catch(() => null)) as { present?: unknown } | null;
      if (!body || typeof body.present !== 'boolean') return apiError(400, 'Invalid request body');
      if (role === 'manager' && !scopeUserIds(guard.user.id).has(String(params.userId))) {
        return apiError(403, 'User not in manager scope');
      }
      db().presence[`${params.slug}|${params.sessionId}|${params.userId}`] = body.present;
      save();
      return HttpResponse.json({ ok: true });
    }),
  ]),

  // ── CSV exports: user-service for admins and managers, lab checks for admins ──
  ...exportHandlers('/api/admin/exports/lab-checks', LAB_EXPORTS, ['admin']),
  ...exportHandlers('/api/admin/exports', USER_EXPORTS, ['admin', 'manager']),
];
