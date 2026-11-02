import { http, HttpResponse } from 'msw';

import type { MarkdownPattern, Role, UserPublic } from '../lib/api-types';

import { courses, findCourse, myCourses, paths, toCourseResponse, visibleModules } from './course-logic';
import { accounts, findAccount, MOCK_PUBLIC_SETTINGS } from './data';
import { bucket, db, save } from './db';
import { MOCK_PATTERNS } from './fixtures/patterns';
import { apiError, currentAccount } from './http';
import { pathMembers } from './space-handlers';
import type { CourseDefinition, ModuleDefinition, PathDefinition } from './model';

// Routes of the administration (batch 4, docs/specs/040-047), mirroring
// user-service admin.go, groups.go, settings.go, patterns.go, paths.go and
// course-service admin_definition.go, paths_admin.go, courses_import.go,
// lab_results.go, state.go (cache). Admin role required everywhere.

const ROLES: Role[] = ['student', 'manager', 'admin'];
const SLUG = /^[a-z0-9][a-z0-9-]*$/;

function admin(request: Request): { user: UserPublic } | { error: Response } {
  const account = currentAccount(request);
  if (!account) return { error: apiError(401, 'Missing Authorization header') };
  if (account.user.role !== 'admin') return { error: apiError(403, 'Admin access required') };
  return { user: account.user };
}

async function json<T>(request: Request): Promise<T | null> {
  try {
    return (await request.json()) as T;
  } catch {
    return null;
  }
}

// ── Settings (settings.go) ──────────────────────────────────────────────────

const SECRET_KEYS = ['oidc_client_secret', 'ldap_bind_password'];

/** The rows of a fresh mock instance: OIDC on, local login and registration off, username change still on. */
const DEFAULT_SETTINGS: Record<string, string> = {
  registration_enabled: 'false',
  registration_email_whitelist: '',
  password_min_length: '8',
  password_require_uppercase: 'false',
  password_require_number: 'false',
  profile_allow_username_change: 'true',
  sso_local_login_enabled: 'false',
  gitlab_url: 'https://gitlab.com',
  oidc_enabled: 'true',
  oidc_provider_url: 'http://keycloak:8080/realms/pupitre',
  oidc_issuer_url: '',
  oidc_redirect_base: 'http://localhost:3000',
  oidc_browser_base_url: '',
  oidc_client_id: 'pupitre',
  oidc_client_secret: 'mock-secret',
  oidc_scopes: 'openid email profile groups',
  oidc_group_claim: 'groups',
  oidc_group_admins: 'pupitre-admins',
  oidc_insecure_skip_verify: 'false',
  ldap_enabled: 'false',
  ldap_server_url: '',
  ldap_bind_dn: '',
  ldap_bind_password: '',
  ldap_user_base_dn: '',
  ldap_user_filter: '(mail=%s)',
  ldap_group_base_dn: '',
  ldap_group_filter: '',
};

function settingValues(): Record<string, string> {
  return { ...DEFAULT_SETTINGS, ...db().settings };
}

function settingRows() {
  return Object.entries(settingValues()).map(([key, value]) => ({
    key,
    value: SECRET_KEYS.includes(key) && value !== '' ? '********' : value,
    description: null,
  }));
}

/** GET /api/settings/public follows what the admin saved. */
export function publicSettings() {
  const values = settingValues();
  return Object.fromEntries(Object.keys(MOCK_PUBLIC_SETTINGS).map((key) => [key, values[key] ?? MOCK_PUBLIC_SETTINGS[key as keyof typeof MOCK_PUBLIC_SETTINGS]]));
}

// ── Users (admin.go) ────────────────────────────────────────────────────────

function adminUserRow(user: UserPublic) {
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
    enrolledCourses: Object.keys(db().enrollments[user.id] ?? {}).length,
  };
}

// ── Groups (groups.go) ──────────────────────────────────────────────────────

function groupDepth(id: string): number {
  const group = db().groups.find((g) => g.id === id);
  return group?.parentId ? groupDepth(group.parentId) + 1 : 0;
}

function descendantIds(id: string): Set<string> {
  const ids = new Set([id]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const group of db().groups) {
      if (group.parentId && ids.has(group.parentId) && !ids.has(group.id)) {
        ids.add(group.id);
        grew = true;
      }
    }
  }
  return ids;
}

// ── Courses (admin_definition.go, courses_import.go) ────────────────────────

/** A course's modules as content.Module, the definition's sessions keyed by id. */
function definitionOf(course: CourseDefinition) {
  const { sessions, ...rest } = course.spec;
  const keyed = sessions?.length ? Object.fromEntries(sessions.map(({ id, ...session }) => [id, session])) : undefined;
  return { ...rest, ...(keyed ? { sessions: keyed } : {}) };
}

function specFromBody(spec: Record<string, unknown>): CourseDefinition['spec'] {
  const { sessions, ...rest } = spec as CourseDefinition['spec'] & { sessions?: unknown };
  const list =
    sessions && typeof sessions === 'object' && !Array.isArray(sessions)
      ? Object.entries(sessions as Record<string, object>).map(([id, session]) => ({ id, ...session }) as NonNullable<CourseDefinition['spec']['sessions']>[number])
      : (sessions as CourseDefinition['spec']['sessions']);
  return { ...rest, ...(list ? { sessions: list } : {}) } as CourseDefinition['spec'];
}

function moduleSlug(name: string) {
  return name.replace(/[^\p{L}\p{N}_-]/gu, '-').replace(/^-+|-+$/g, '').toLowerCase();
}

type ImportBody = { markdown?: string; slug?: string; split?: string; mode?: string };

/** A small take on content.ParseCourseMarkdown: frontmatter, then one module per heading of the split level. */
function parseImport(body: ImportBody) {
  const warnings: string[] = [];
  let text = (body.markdown ?? '').replace(/\r\n?/g, '\n');
  const meta: Record<string, string> = {};
  const front = /^---\n([\s\S]*?)\n---\n?/.exec(text);
  if (front) {
    for (const line of front[1]!.split('\n')) {
      const match = /^(\w+):\s*(.*)$/.exec(line);
      if (match) meta[match[1]!] = match[2]!.replace(/^["']|["']$/g, '');
    }
    text = text.slice(front[0].length);
  }
  const split = body.split || meta.split || 'none';
  const level = { h1: 1, h2: 2, h3: 3 }[split];
  const modules: ModuleDefinition[] = [];
  if (!level) {
    modules.push({ name: meta.title || 'Contenu', type: 'text', content: text.trim() });
  } else {
    const heading = new RegExp(`^${'#'.repeat(level)} (.+)$`, 'gm');
    const marks = [...text.matchAll(heading)];
    if (marks.length === 0) warnings.push(`Aucun titre de niveau ${level} : un seul module a été créé.`);
    if (marks.length === 0) modules.push({ name: meta.title || 'Contenu', type: 'text', content: text.trim() });
    marks.forEach((mark, i) => {
      const start = mark.index! + mark[0].length;
      const end = marks[i + 1]?.index ?? text.length;
      modules.push({ name: mark[1]!.trim(), type: 'text', content: text.slice(start, end).trim() });
    });
  }
  const course: CourseDefinition['spec'] = { title: meta.title || '' };
  if (meta.description) course.description = meta.description;
  if (meta.category) course.category = meta.category;
  if (meta.difficulty) course.difficulty = meta.difficulty;
  if (meta.public) course.public = meta.public === 'true';
  return { meta, course, modules, warnings };
}

function exportMarkdown(course: CourseDefinition): string {
  const { spec } = course;
  const front = ['---', `title: ${spec.title}`];
  if (spec.description) front.push(`description: ${spec.description}`);
  if (spec.category) front.push(`category: ${spec.category}`);
  if (spec.difficulty) front.push(`difficulty: ${spec.difficulty}`);
  front.push(`public: ${Boolean(spec.public)}`, 'split: h2', '---', '');
  const body = (spec.modules ?? []).map((m) => `## ${m.name}\n\n${m.content ?? ''}`.trim()).join('\n\n');
  return `${front.join('\n')}\n${body}\n`;
}

// ── Lab checks (lab_results.go) ─────────────────────────────────────────────

const LAB_CHECKS = [
  { id: 3, username: 'alice', courseSlug: 'pupitre-demo', moduleIndex: 7, moduleName: 'Lab GitLab', allow: true, violations: [], verified: true, checkedAt: '2026-10-18T10:09:00Z' },
  {
    id: 2,
    username: 'alice',
    courseSlug: 'pupitre-demo',
    moduleIndex: 7,
    moduleName: 'Lab GitLab',
    allow: false,
    violations: ['Le pipeline doit contenir un job test', 'Le README est absent'],
    verified: true,
    checkedAt: '2026-10-18T10:02:00Z',
  },
  { id: 1, username: '<img src=x onerror=alert("xss-username")>', courseSlug: 'distrobox', moduleIndex: 0, moduleName: 'Distrobox', allow: true, violations: null, verified: false, checkedAt: '2026-10-15T16:40:00Z' },
];

const ok = (body: Record<string, unknown> = { message: 'OK' }, status = 200) => HttpResponse.json(body, { status });

export const adminHandlers = [
  // ── Dashboard (admin.go) ──────────────────────────────────────────────────
  http.get('/api/admin/stats', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const students = accounts().filter((a) => a.user.role === 'student').length;
    const enrollments = Object.values(db().enrollments);
    const totalCourses = new Set(enrollments.flatMap((e) => Object.keys(e))).size;
    return HttpResponse.json({
      total_users: students,
      total_enrollments: enrollments.reduce((sum, e) => sum + Object.keys(e).length, 0),
      totalCourses,
      total_labs: 0,
      total_submissions: 0,
      success_rate: '0',
    });
  }),

  http.get('/api/admin/leaderboard', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const leaderboard = accounts()
      .map(({ user }) => {
        const enrolled = myCourses(user);
        const quizzes = Object.values(db().quizzes[user.id] ?? {}).flatMap((byCourse) => Object.values(byCourse));
        return {
          id: user.id,
          username: user.username,
          email: user.email,
          avatarUrl: user.avatarUrl,
          totalScore: enrolled.reduce((sum, c) => sum + c.totalScore, 0),
          passedModules: quizzes.filter((q) => q.passed).length,
          enrolledCourses: enrolled.length,
        };
      })
      .sort((a, b) => b.totalScore - a.totalScore || b.passedModules - a.passedModules);
    return HttpResponse.json({ leaderboard });
  }),

  // ── Users (admin.go) ──────────────────────────────────────────────────────
  http.get('/api/admin/users/providers', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const counts = new Map<string, number>();
    for (const { user } of accounts()) counts.set(user.authProvider || 'local', (counts.get(user.authProvider || 'local') ?? 0) + 1);
    return HttpResponse.json({ providers: [...counts].map(([provider, count]) => ({ provider, count })) });
  }),

  http.get('/api/admin/users/search', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const q = (new URL(request.url).searchParams.get('q') ?? '').toLowerCase();
    const users = accounts()
      .map(({ user }) => user)
      .filter((u) => q && (u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q)))
      .slice(0, 10)
      .map((u) => ({ id: u.id, username: u.username, email: u.email }));
    return HttpResponse.json({ users });
  }),

  http.get('/api/admin/users', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const provider = new URL(request.url).searchParams.get('provider');
    const users = accounts()
      .map(({ user }) => adminUserRow(user))
      .filter((u) => !provider || u.authProvider === provider);
    return HttpResponse.json({ users, total: users.length });
  }),

  http.put('/api/admin/users/:userId', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const id = String(params.userId);
    if (!findAccount(id)) return apiError(404, 'User not found');
    const body = await json<{ role?: string; isActive?: boolean }>(request);
    if (!body) return apiError(400, 'Invalid JSON');
    if (body.role !== undefined && !ROLES.includes(body.role as Role)) return apiError(400, `Role must be one of: ${ROLES.join(', ')}`);
    const changes = (db().profiles[id] ??= {});
    if (body.role) changes.role = body.role as Role;
    if (typeof body.isActive === 'boolean') changes.isActive = body.isActive;
    save();
    return HttpResponse.json(findAccount(id)!.user);
  }),

  http.delete('/api/admin/users/:userId', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const id = String(params.userId);
    if (!findAccount(id)) return apiError(404, 'User not found');
    db().deletedUsers.push(id);
    save();
    return ok({ message: 'User deleted' });
  }),

  // ── Settings (settings.go) ────────────────────────────────────────────────
  http.get('/api/admin/settings', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    return HttpResponse.json({ settings: settingRows() });
  }),

  http.put('/api/admin/settings', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<Record<string, unknown>>(request);
    if (!body) return apiError(400, 'Invalid JSON');
    if (Object.keys(body).length === 0) return apiError(400, 'No settings provided');
    for (const [key, value] of Object.entries(body)) {
      if (!(key in DEFAULT_SETTINGS) || key === 'oidc_insecure_skip_verify') return apiError(400, `Unknown setting key: '${key}'`);
      if (typeof value !== 'string') return apiError(400, `Setting '${key}' must be a string`);
    }
    Object.assign(db().settings, body);
    save();
    return HttpResponse.json({ settings: settingRows() });
  }),

  // ── Patterns (patterns.go) ────────────────────────────────────────────────
  http.post('/api/admin/patterns', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<Partial<MarkdownPattern>>(request);
    if (!body?.name || !body.label || !body.html) return apiError(400, 'name, label and html are required');
    const list = (db().patterns ??= structuredClone(MOCK_PATTERNS));
    if (list.some((p) => p.name === body.name && p.scope === (body.scope || 'global'))) {
      return apiError(409, 'Pattern with this name already exists in this scope');
    }
    const now = new Date().toISOString();
    const pattern: MarkdownPattern = {
      id: crypto.randomUUID(),
      name: body.name,
      label: body.label,
      description: body.description ?? '',
      parameter: body.parameter ?? '',
      html: body.html,
      css: body.css ?? '',
      js: body.js ?? '',
      scope: body.scope || 'global',
      fromConfig: false,
      createdAt: now,
      updatedAt: now,
    };
    list.push(pattern);
    save();
    return HttpResponse.json(pattern, { status: 201 });
  }),

  http.put('/api/admin/patterns/:name', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<Partial<MarkdownPattern>>(request);
    if (!body?.html) return apiError(400, 'html is required');
    const list = (db().patterns ??= structuredClone(MOCK_PATTERNS));
    const pattern = list.find((p) => p.name === params.name);
    if (!pattern) return apiError(404, 'Pattern not found');
    Object.assign(pattern, {
      label: body.label ?? pattern.label,
      html: body.html,
      css: body.css ?? '',
      js: body.js ?? '',
      scope: body.scope || pattern.scope,
      updatedAt: new Date().toISOString(),
    });
    save();
    return HttpResponse.json(pattern);
  }),

  http.delete('/api/admin/patterns/:name', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const list = (db().patterns ??= structuredClone(MOCK_PATTERNS));
    if (!list.some((p) => p.name === params.name)) return apiError(404, 'Pattern not found');
    db().patterns = list.filter((p) => p.name !== params.name);
    save();
    return ok({ message: 'Pattern deleted' });
  }),

  // ── Groups (groups.go, admin.go) ──────────────────────────────────────────
  http.get('/api/admin/groups/mappings', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    return HttpResponse.json({ mappings: Object.entries(db().mappings).map(([groupName, platformRole]) => ({ groupName, platformRole })) });
  }),

  http.post('/api/admin/groups/mappings', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<{ groupName?: string; platformRole?: string }>(request);
    if (!body?.groupName) return apiError(400, 'groupName required');
    if (!ROLES.includes(body.platformRole as Role)) return apiError(400, `platformRole must be one of: ${ROLES.join(', ')}`);
    db().mappings[body.groupName] = body.platformRole as Role;
    for (const group of db().groups) if (group.name === body.groupName) group.mappedRole = body.platformRole;
    save();
    return ok({ message: 'Mapping saved' });
  }),

  http.delete('/api/admin/groups/mappings/:groupName', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const name = String(params.groupName);
    delete db().mappings[name];
    for (const group of db().groups) if (group.name === name) delete group.mappedRole;
    save();
    return ok({ message: 'Mapping deleted' });
  }),

  http.get('/api/admin/groups', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const groups = db().groups.map((g) => ({
      id: g.id,
      name: g.name,
      source: g.source,
      parentId: g.parentId,
      depth: groupDepth(g.id),
      createdAt: '2026-09-01T09:00:00Z',
      memberCount: g.memberIds.length,
      mappedRole: db().mappings[g.name] ?? g.mappedRole ?? '',
    }));
    return HttpResponse.json({ groups });
  }),

  http.post('/api/admin/groups', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<{ name?: string }>(request);
    if (!body?.name) return apiError(400, 'name required');
    if (db().groups.some((g) => g.name === body.name)) return apiError(409, 'A group with this name already exists');
    const id = crypto.randomUUID();
    db().groups.push({ id, name: body.name, source: 'local', parentId: '', memberIds: [], courseSlugs: [] });
    save();
    return ok({ id, message: 'Group created' }, 201);
  }),

  http.put('/api/admin/groups/:groupId', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const group = db().groups.find((g) => g.id === params.groupId && g.source === 'local');
    if (!group) return apiError(404, 'Group not found');
    const body = await json<{ name?: string }>(request);
    if (!body?.name) return apiError(400, 'name required');
    group.name = body.name;
    save();
    return ok({ message: 'Group updated' });
  }),

  http.delete('/api/admin/groups/:groupId', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const group = db().groups.find((g) => g.id === params.groupId && g.source === 'local');
    if (!group) return apiError(404, 'Group not found');
    if (db().groups.some((g) => g.parentId === group.id)) return apiError(409, 'Cannot delete a group that has subgroups');
    if (group.memberIds.length > 0) return apiError(409, 'Cannot delete a group that has members');
    db().groups = db().groups.filter((g) => g.id !== group.id);
    save();
    return ok({ message: 'Group deleted' });
  }),

  http.post('/api/admin/groups/:groupId/subgroups', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const parent = db().groups.find((g) => g.id === params.groupId);
    if (!parent) return apiError(404, 'Group not found');
    const body = await json<{ name?: string }>(request);
    if (!body?.name) return apiError(400, 'name required');
    if (db().groups.some((g) => g.name === body.name)) return apiError(409, 'A group with this name already exists');
    const id = crypto.randomUUID();
    db().groups.push({ id, name: body.name, source: 'local', parentId: parent.id, memberIds: [], courseSlugs: [] });
    save();
    return ok({ id, message: 'Group created' }, 201);
  }),

  http.patch('/api/admin/groups/:groupId/parent', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const group = db().groups.find((g) => g.id === params.groupId);
    if (!group) return apiError(404, 'Group not found');
    const body = await json<{ parentId?: string | null }>(request);
    const parentId = body?.parentId ?? '';
    if (parentId && descendantIds(group.id).has(parentId)) return apiError(400, 'Cannot move a group under itself or its descendants');
    group.parentId = parentId;
    save();
    return ok({ message: 'Group moved' });
  }),

  http.get('/api/admin/groups/:groupId/members', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const id = String(params.groupId);
    const group = db().groups.find((g) => g.id === id);
    if (!group) return apiError(404, 'Group not found');
    const recursive = new URL(request.url).searchParams.get('recursive') === 'true';
    const groups = recursive ? db().groups.filter((g) => descendantIds(id).has(g.id)) : [group];
    const ids = [...new Set(groups.flatMap((g) => g.memberIds))];
    const members = ids.flatMap((uid) => {
      const user = findAccount(uid)?.user;
      if (!user) return [];
      const row = adminUserRow(user);
      return [recursive ? { ...row, directMembership: group.memberIds.includes(uid) } : row];
    });
    return HttpResponse.json({ members, total: members.length });
  }),

  http.post('/api/admin/groups/:groupId/members', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const group = db().groups.find((g) => g.id === params.groupId);
    if (!group) return apiError(404, 'Group not found');
    const body = await json<{ userId?: string }>(request);
    if (!body?.userId || !findAccount(body.userId)) return apiError(404, 'User not found');
    if (!group.memberIds.includes(body.userId)) group.memberIds.push(body.userId);
    save();
    return ok({ message: 'Member added' });
  }),

  http.delete('/api/admin/groups/:groupId/members/:userId', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const group = db().groups.find((g) => g.id === params.groupId);
    if (!group) return apiError(404, 'Group not found');
    group.memberIds = group.memberIds.filter((id) => id !== params.userId);
    save();
    return ok({ message: 'Member removed' });
  }),

  // ── Course enrollments (admin.go) ─────────────────────────────────────────
  http.get('/api/admin/enrollments/:slug/groups', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const slug = String(params.slug);
    const groups = db()
      .groups.filter((g) => g.courseSlugs.includes(slug))
      .map((g) => ({ id: g.id, name: g.name, source: g.source, memberCount: g.memberIds.length, enrolledAt: '2026-09-15T09:00:00Z' }));
    return HttpResponse.json({ groups });
  }),

  http.post('/api/admin/enrollments/:slug/groups', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const slug = String(params.slug);
    const body = await json<{ groupId?: string }>(request);
    const group = db().groups.find((g) => g.id === body?.groupId);
    if (!group) return apiError(404, 'Group not found');
    if (!group.courseSlugs.includes(slug)) group.courseSlugs.push(slug);
    let enrolled = 0;
    for (const uid of group.memberIds) {
      const mine = bucket(db().enrollments, uid);
      if (!mine[slug]) {
        mine[slug] = new Date().toISOString();
        enrolled += 1;
      }
    }
    save();
    return ok({ message: 'Group enrolled', enrolled });
  }),

  http.delete('/api/admin/enrollments/:slug/groups/:groupId', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const group = db().groups.find((g) => g.id === params.groupId);
    if (!group) return apiError(404, 'Group not found');
    group.courseSlugs = group.courseSlugs.filter((s) => s !== params.slug);
    save();
    return ok({ message: 'Group unenrolled' });
  }),

  http.get('/api/admin/enrollments/:slug', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const slug = String(params.slug);
    const enrollments = Object.entries(db().enrollments).flatMap(([userId, mine]) => {
      const user = findAccount(userId)?.user;
      return mine[slug] && user ? [{ userId, username: user.username, email: user.email, enrolledAt: mine[slug] }] : [];
    });
    return HttpResponse.json({ enrollments });
  }),

  http.post('/api/admin/enrollments/:slug', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<{ userId?: string }>(request);
    if (!body?.userId || !findAccount(body.userId)) return apiError(404, 'User not found');
    bucket(db().enrollments, body.userId)[String(params.slug)] ??= new Date().toISOString();
    save();
    return ok({ message: 'User enrolled' });
  }),

  http.delete('/api/admin/enrollments/:slug/users/:userId', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    delete db().enrollments[String(params.userId)]?.[String(params.slug)];
    save();
    return ok({ message: 'User unenrolled' });
  }),

  // ── Path enrollments (paths.go) ───────────────────────────────────────────
  http.get('/api/admin/paths/:slug/enrollments', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    const users = Object.entries(db().paths).flatMap(([userId, mine]) => {
      const account = findAccount(userId);
      const enrolledAt = mine[path.slug];
      if (!account || !enrolledAt) return [];
      const steps = pathMembers(account.user, path);
      return [
        {
          userId,
          email: account.user.email,
          role: account.user.role,
          enrolledAt,
          completedCourses: steps.filter((s) => s.status === 'completed').length,
          totalCourses: steps.length,
          courses: steps,
        },
      ];
    });
    return HttpResponse.json({ users });
  }),

  http.post('/api/admin/paths/:slug/enrollments', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    const body = await json<{ userId?: string }>(request);
    if (!body?.userId || !findAccount(body.userId)) return apiError(404, 'User not found');
    const now = new Date().toISOString();
    bucket(db().paths, body.userId)[path.slug] ??= now;
    for (const course of path.spec.courses ?? []) bucket(db().enrollments, body.userId)[course] ??= now;
    save();
    return ok({ message: 'User enrolled' });
  }),

  http.delete('/api/admin/paths/:slug/enrollments/:userId', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    delete db().paths[String(params.userId)]?.[String(params.slug)];
    save();
    return ok({ message: 'User unenrolled' });
  }),

  // ── Course-service: courses ───────────────────────────────────────────────
  http.get('/api/admin/courses', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const list = courses().map(toCourseResponse);
    return HttpResponse.json({ courses: list, total: list.length });
  }),

  http.post('/api/admin/courses/import/preview', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<ImportBody>(request);
    if (!body?.markdown?.trim()) return apiError(400, 'markdown is required');
    const mode = body.mode || 'create';
    const parsed = parseImport(body);
    const slug = mode === 'create' ? body.slug || parsed.meta.slug || moduleSlug(parsed.course.title ?? '') : body.slug ?? '';
    const existing = mode === 'append' ? new Set((findCourse(slug)?.spec.modules ?? []).map((m) => moduleSlug(m.name))) : new Set<string>();
    return HttpResponse.json({
      slug,
      mode,
      course: parsed.course,
      modules: parsed.modules.map((m) => ({
        name: m.name,
        slug: moduleSlug(m.name),
        type: m.type,
        bytes: new TextEncoder().encode(m.content ?? '').length,
        existing: existing.has(moduleSlug(m.name)) || undefined,
      })),
      warnings: parsed.warnings,
    });
  }),

  http.post('/api/admin/courses/import', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<ImportBody>(request);
    if (!body?.markdown?.trim()) return apiError(400, 'markdown is required');
    const mode = body.mode || 'create';
    const parsed = parseImport(body);
    if (mode === 'create') {
      const slug = body.slug || parsed.meta.slug || moduleSlug(parsed.course.title ?? '');
      if (!SLUG.test(slug)) return apiError(400, 'invalid slug');
      if (findCourse(slug)) return apiError(409, 'Course already exists');
      db().courseDefs[slug] = { ...parsed.course, title: parsed.course.title || slug, modules: parsed.modules };
      save();
      return HttpResponse.json({ slug, mode, moduleCount: parsed.modules.length, warnings: parsed.warnings }, { status: 201 });
    }
    const course = findCourse(body.slug ?? '');
    if (!course) return apiError(404, 'Course not found');
    const modules = mode === 'replace' ? parsed.modules : [...(course.spec.modules ?? []), ...parsed.modules];
    db().courseDefs[course.slug] = { ...course.spec, modules };
    save();
    return HttpResponse.json({ slug: course.slug, mode, moduleCount: parsed.modules.length, warnings: parsed.warnings });
  }),

  http.post('/api/admin/courses', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<{ slug?: string; spec?: Record<string, unknown> }>(request);
    if (!body?.spec) return apiError(400, 'Invalid course payload');
    if (!body.slug) return apiError(400, 'slug is required');
    if (!SLUG.test(body.slug)) return apiError(400, 'Invalid slug: use lowercase letters, digits and hyphens');
    if (findCourse(body.slug)) return apiError(409, 'Course already exists');
    db().courseDefs[body.slug] = specFromBody(body.spec);
    save();
    return HttpResponse.json({ slug: body.slug }, { status: 201 });
  }),

  http.get('/api/admin/courses/:slug/export/markdown', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    return new HttpResponse(exportMarkdown(course), {
      headers: { 'Content-Type': 'text/markdown; charset=utf-8', 'Content-Disposition': `attachment; filename="${course.slug}.md"` },
    });
  }),

  http.get('/api/admin/courses/:slug/definition', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    return HttpResponse.json({ slug: course.slug, spec: definitionOf(course) });
  }),

  http.put('/api/admin/courses/:slug/definition', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    const body = await json<{ spec?: Record<string, unknown> }>(request);
    if (!body?.spec) return apiError(400, 'Invalid course payload');
    db().courseDefs[course.slug] = specFromBody(body.spec);
    save();
    return HttpResponse.json({ slug: course.slug });
  }),

  http.delete('/api/admin/courses/:slug/definition', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    db().courseDefs[course.slug] = null;
    save();
    return ok({ message: 'Course deleted' });
  }),

  // ── Course-service: paths ─────────────────────────────────────────────────
  http.post('/api/admin/courses/paths', async ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const body = await json<{ slug?: string; spec?: PathDefinition['spec'] }>(request);
    if (!body?.spec) return apiError(400, 'Invalid path payload');
    if (!body.slug) return apiError(400, 'slug is required');
    if (!SLUG.test(body.slug)) return apiError(400, 'Invalid slug: use lowercase letters, digits and hyphens');
    if (paths().some((p) => p.slug === body.slug)) return apiError(409, 'Path already exists');
    db().pathDefs[body.slug] = body.spec;
    save();
    return HttpResponse.json({ slug: body.slug }, { status: 201 });
  }),

  http.get('/api/admin/courses/paths/:slug/definition', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    return HttpResponse.json({ slug: path.slug, spec: path.spec });
  }),

  http.put('/api/admin/courses/paths/:slug/definition', async ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    const body = await json<{ spec?: PathDefinition['spec'] }>(request);
    if (!body?.spec) return apiError(400, 'Invalid path payload');
    db().pathDefs[path.slug] = body.spec;
    save();
    return HttpResponse.json({ slug: path.slug });
  }),

  http.delete('/api/admin/courses/paths/:slug/definition', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const path = paths().find((p) => p.slug === params.slug);
    if (!path) return apiError(404, 'Path not found');
    db().pathDefs[path.slug] = null;
    save();
    return ok({ message: 'Path deleted' });
  }),

  // ── Course-service: labs and cache ────────────────────────────────────────
  http.get('/api/admin/lab-checks', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = new URL(request.url).searchParams.get('course');
    const all = [...LAB_CHECKS, ...db().labChecks];
    return HttpResponse.json(all.filter((c) => !course || c.courseSlug === course));
  }),

  http.post('/api/admin/cache/clear', ({ request }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const repos = new Set(courses().flatMap((c) => (c.spec.modules ?? []).flatMap((m) => (m.src ? [m.src] : []))));
    return HttpResponse.json({ status: 'ok', reposCleared: repos.size });
  }),

  http.post('/api/admin/courses/:slug/cache/clear', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = findCourse(String(params.slug));
    if (!course) return apiError(404, 'Course not found');
    const repos = new Set((course.spec.modules ?? []).flatMap((m) => (m.src ? [m.src] : [])));
    return HttpResponse.json({ status: 'ok', reposCleared: repos.size });
  }),

  http.post('/api/admin/courses/:slug/modules/:index/cache/clear', ({ request, params }) => {
    const guard = admin(request);
    if ('error' in guard) return guard.error;
    const course = findCourse(String(params.slug));
    const module = course && visibleModules(course, true)[Number(params.index)];
    if (!module) return apiError(404, 'Module not found');
    return HttpResponse.json({ status: 'ok', reposCleared: module.src ? 1 : 0 });
  }),
];

