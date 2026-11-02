import { request, requestBlob } from './api-client';
import type {
  AdminGroup,
  GroupMapping,
  GroupMember,
  AdminLeaderboardEntry,
  AdminStats,
  AuthorizeResponse,
  AuthResponse,
  CheckResult,
  Course,
  EarnedBadge,
  EnrollmentRow,
  ExportCategory,
  ExportPreview,
  ExportRequest,
  MarkdownPattern,
  ModuleDetail,
  ModuleSummary,
  LeaderboardEntry,
  LearningPath,
  ManagerGroup,
  MyCourse,
  MyGroup,
  MyPath,
  MyXp,
  PublicSettings,
  PathEnrolledUser,
  PathEnrollmentRow,
  PublicUser,
  ScopeGroup,
  ScopeUser,
  SessionParticipant,
  QuizAnswer,
  QuizResult,
  Role,
  SessionBooking,
  SessionInput,
  SkillLevel,
  SkillModuleStatus,
  UserPublic,
} from './api-types';

// One typed function per Go route used by the frontend (docs/specs/004-api-and-routing.md),
// on top of the HTTP client of api-client.ts.

const enc = encodeURIComponent;

// ── Authentication (user-service, Keycloak only: docs/specs/001) ────────────

export const getPublicSettings = () => request<PublicSettings>('/settings/public', { anonymous: true });

export const oidcAuthorize = () => request<AuthorizeResponse>('/auth/oidc/authorize', { anonymous: true });

export const oidcCallback = (code: string, state: string) =>
  request<AuthResponse>('/auth/oidc/callback', { method: 'POST', body: { code, state }, anonymous: true });

export const getMe = () => request<UserPublic>('/auth/me');

/** Ends every session of the signed-in user on the server (docs/specs/190-backend-defects.md B7). */
export const logout = () => request<{ message: string }>('/auth/logout', { method: 'POST' });

/** Bio or emoji avatar only: username and avatar URLs are not offered (docs/specs/021). */
export const updateProfile = (body: { bio: string } | { avatarUrl: string }) =>
  request<UserPublic>('/auth/profile', { method: 'PUT', body });

// ── course-service ──────────────────────────────────────────────────────────

export const listCourses = () => request<{ courses: Course[]; total: number }>('/courses');

export const getCourse = (slug: string) => request<Course>(`/courses/${enc(slug)}`);

export const listModules = (slug: string) =>
  request<{ modules: ModuleSummary[] }>(`/courses/${enc(slug)}/modules`);

export const getModule = (slug: string, index: number) =>
  request<ModuleDetail>(`/courses/${enc(slug)}/modules/${index}`);

export const submitQuiz = (slug: string, index: number, answers: Record<string, QuizAnswer>) =>
  request<QuizResult>(`/courses/${enc(slug)}/modules/${index}/submit`, {
    method: 'POST',
    body: { answers },
  });

export const completeLesson = (slug: string, moduleSlug: string) =>
  request<{ message: string }>(`/courses/${enc(slug)}/lessons/${enc(moduleSlug)}/complete`, {
    method: 'POST',
    body: {},
  });

export const checkModule = (slug: string, index: number) =>
  request<CheckResult>(`/courses/${enc(slug)}/modules/${index}/check`, { method: 'POST', body: {} });

export const checkStep = (slug: string, index: number, step: number) =>
  request<CheckResult>(`/courses/${enc(slug)}/modules/${index}/steps/${step}/check`, {
    method: 'POST',
    body: {},
  });

/** Stores a check run by the desktop app; the server keeps it as not verified. */
export const recordLocalCheck = (slug: string, index: number, result: CheckResult) =>
  request<CheckResult>(`/courses/${enc(slug)}/modules/${index}/record`, {
    method: 'POST',
    body: result,
  });

// ── user-service ────────────────────────────────────────────────────────────

export const myCourses = () => request<{ courses: MyCourse[] }>('/my/courses');

export const mySkills = () => request<{ skills: SkillLevel[] }>('/my/skills');

export const mySkillModules = (skill: string) =>
  request<{ skill: string; modules: SkillModuleStatus[] }>(`/my/skills/${enc(skill)}`);

export const myPaths = () => request<{ paths: MyPath[] }>('/my/paths');

export const myXp = () => request<MyXp>('/my/xp');

export const myGroups = () => request<{ groups: MyGroup[]; total: number }>('/my/groups');

export const myBadges = () => request<{ badges: EarnedBadge[] }>('/my/badges');

export const mySessionBookings = () => request<{ bookings: SessionBooking[] }>('/my/session-bookings');

// Public routes: sent without the token.

export const leaderboard = () => request<{ leaderboard: LeaderboardEntry[] }>('/leaderboard', { anonymous: true });

export const getPublicUser = (id: string) => request<PublicUser>(`/users/${enc(id)}`, { anonymous: true });

export const userBadges = (id: string) =>
  request<{ badges: EarnedBadge[] }>(`/users/${enc(id)}/badges`, { anonymous: true });

export const enroll = (slug: string) =>
  request<{ message: string }>(`/enrollments/${enc(slug)}`, { method: 'POST', body: {} });

/** 204 No Content; 409 when the session is full. */
export const bookSession = (slug: string, sessionId: string) =>
  request<void>(`/session-bookings/${enc(slug)}/${enc(sessionId)}`, { method: 'POST' });

export const cancelBooking = (slug: string, sessionId: string) =>
  request<void>(`/session-bookings/${enc(slug)}/${enc(sessionId)}`, { method: 'DELETE' });

export const listPaths = () => request<{ paths: LearningPath[] }>('/paths');

// Course sessions: admin or manager (docs/specs/022).

export const createSession = (slug: string, body: SessionInput) =>
  request<unknown>(`/admin/courses/${enc(slug)}/sessions`, { method: 'POST', body });

export const updateSession = (slug: string, sessionId: string, body: SessionInput) =>
  request<unknown>(`/admin/courses/${enc(slug)}/sessions/${enc(sessionId)}`, { method: 'PUT', body });

export const deleteSession = (slug: string, sessionId: string) =>
  request<unknown>(`/admin/courses/${enc(slug)}/sessions/${enc(sessionId)}`, { method: 'DELETE' });

/** Requires a signed-in user: the Astro frontend called it anonymously (defect A1). */
export const listPatterns = () => request<{ patterns: MarkdownPattern[] }>('/patterns');

// ── Manager space (docs/specs/030) ──────────────────────────────────────────

export const managerUsers = () =>
  request<{ users: ScopeUser[] | null; total: number; groups: ScopeGroup[] }>('/manager/users');

export const managerUserEnrollments = (userId: string) =>
  request<{ enrollments: EnrollmentRow[] | null }>(`/manager/users/${enc(userId)}/enrollments`);

export const managerUserPathEnrollments = (userId: string) =>
  request<{ pathEnrollments: PathEnrollmentRow[] }>(`/manager/users/${enc(userId)}/path-enrollments`);

export const managerEnroll = (slug: string, userId: string) =>
  request<{ message: string }>(`/manager/enrollments/${enc(slug)}`, { method: 'POST', body: { userId } });

export const managerUnenroll = (slug: string, userId: string) =>
  request<{ message: string }>(`/manager/enrollments/${enc(slug)}/users/${enc(userId)}`, { method: 'DELETE' });

export const managerGroups = () => request<{ groups: ManagerGroup[] }>('/manager/groups');

export const managerCreateGroup = (name: string) =>
  request<{ id: string }>('/manager/groups', { method: 'POST', body: { name } });

export const managerCreateSubgroup = (groupId: string, name: string) =>
  request<{ id: string }>(`/manager/groups/${enc(groupId)}/subgroups`, { method: 'POST', body: { name } });

export const managerDeleteGroup = (groupId: string) =>
  request<{ message: string }>(`/manager/groups/${enc(groupId)}`, { method: 'DELETE' });

export const managerGroupMembers = (groupId: string) =>
  request<{ members: ScopeUser[] | null }>(`/manager/groups/${enc(groupId)}/members`);

export const managerAddGroupMember = (groupId: string, userId: string) =>
  request<{ message: string }>(`/manager/groups/${enc(groupId)}/members`, { method: 'POST', body: { userId } });

export const managerRemoveGroupMember = (groupId: string, userId: string) =>
  request<{ message: string }>(`/manager/groups/${enc(groupId)}/members/${enc(userId)}`, { method: 'DELETE' });

export const managerPathEnrollments = (slug: string) =>
  request<{ users: PathEnrolledUser[] | null }>(`/manager/paths/${enc(slug)}/enrollments`);

export const managerEnrollPath = (slug: string, userId: string) =>
  request<{ message: string }>(`/manager/paths/${enc(slug)}/enrollments`, { method: 'POST', body: { userId } });

export const managerUnenrollPath = (slug: string, userId: string) =>
  request<{ message: string }>(`/manager/paths/${enc(slug)}/enrollments/${enc(userId)}`, { method: 'DELETE' });

// ── Attendance (docs/specs/022): /api/admin/… for an admin, /api/manager/… for a manager ──

type TrainerRole = 'admin' | 'manager';

export const sessionParticipants = (role: TrainerRole, slug: string, sessionId: string) =>
  request<{ participants: SessionParticipant[] | null }>(`/${role}/session-bookings/${enc(slug)}/${enc(sessionId)}`);

export const markPresence = (role: TrainerRole, slug: string, sessionId: string, userId: string, present: boolean) =>
  request<{ ok: boolean }>(`/${role}/session-bookings/${enc(slug)}/${enc(sessionId)}/${enc(userId)}/presence`, {
    method: 'PATCH',
    body: { present },
  });

// ── CSV exports (docs/specs/047): user-service and course-service lab checks ──

export type ExportSource = '/admin/exports' | '/admin/exports/lab-checks';

export const exportCategories = (source: ExportSource) =>
  request<{ categories: ExportCategory[] }>(`${source}/categories`);

export const exportPreview = (source: ExportSource, body: ExportRequest) =>
  request<ExportPreview>(`${source}/preview`, { method: 'POST', body });

export const exportDownload = (source: ExportSource, body: ExportRequest) =>
  requestBlob(`${source}/download`, { method: 'POST', body });

// ── Administration (docs/specs/040-047) ─────────────────────────────────────

export const adminStats = () => request<AdminStats>('/admin/stats');

export const adminLeaderboard = () => request<{ leaderboard: AdminLeaderboardEntry[] }>('/admin/leaderboard');

/** Every course, private and hidden ones included. */
export const adminCourses = () => request<{ courses: Course[]; total: number }>('/admin/courses');

export const adminUsers = () => request<{ users: ScopeUser[] | null; total: number }>('/admin/users');

export const authProviders = () => request<{ providers: { provider: string; count: number }[] | null }>('/admin/users/providers');

/** Role and activation only: username, bio and avatar come from Keycloak or the profile (spec 041). */
export const updateUserAdmin = (userId: string, body: { role: Role; isActive: boolean }) =>
  request<UserPublic>(`/admin/users/${enc(userId)}`, { method: 'PUT', body });

export const deleteUser = (userId: string) =>
  request<{ message: string }>(`/admin/users/${enc(userId)}`, { method: 'DELETE' });

export type PlatformSetting = { key: string; value: string; description: string | null };

export const adminSettings = () => request<{ settings: PlatformSetting[] }>('/admin/settings');

/** Every value is a string; an unknown key is refused with a 400. Returns the updated list. */
export const updateSettings = (values: Record<string, string>) =>
  request<{ settings: PlatformSetting[] }>('/admin/settings', { method: 'PUT', body: values });

export type PatternInput = { name: string; label: string; scope: string; html: string; css: string; js: string };

export const createPattern = (body: PatternInput) =>
  request<MarkdownPattern>('/admin/patterns', { method: 'POST', body });

export const updatePattern = (name: string, body: PatternInput) =>
  request<MarkdownPattern>(`/admin/patterns/${enc(name)}`, { method: 'PUT', body });

export const deletePattern = (name: string) =>
  request<{ message: string }>(`/admin/patterns/${enc(name)}`, { method: 'DELETE' });

/** course-service lab_results.go — labCheckRow */
export type LabCheckRow = {
  id: number | string;
  username: string;
  courseSlug: string;
  moduleIndex: number;
  moduleName: string;
  allow: boolean;
  violations: string[] | null;
  /** false for a check the desktop app ran and recorded (« Auto-déclaré »). */
  verified: boolean;
  checkedAt: string;
};

export const labChecks = (course = '') =>
  request<LabCheckRow[]>(`/admin/lab-checks${course ? `?course=${enc(course)}` : ''}`);

// Groups (user-service groups.go, admin.go — docs/specs/044)

export const adminGroups = () => request<{ groups: AdminGroup[] | null }>('/admin/groups');

export const createGroup = (name: string) => request<{ id: string }>('/admin/groups', { method: 'POST', body: { name } });

export const renameGroup = (groupId: string, name: string) =>
  request<{ message: string }>(`/admin/groups/${enc(groupId)}`, { method: 'PUT', body: { name } });

export const deleteGroup = (groupId: string) =>
  request<{ message: string }>(`/admin/groups/${enc(groupId)}`, { method: 'DELETE' });

export const createSubgroup = (groupId: string, name: string) =>
  request<{ id: string }>(`/admin/groups/${enc(groupId)}/subgroups`, { method: 'POST', body: { name } });

/** parentId null: the group becomes a root. */
export const moveGroup = (groupId: string, parentId: string | null) =>
  request<{ message: string }>(`/admin/groups/${enc(groupId)}/parent`, { method: 'PATCH', body: { parentId } });

export const groupMembers = (groupId: string, recursive: boolean) =>
  request<{ members: GroupMember[] | null; total: number }>(
    `/admin/groups/${enc(groupId)}/members${recursive ? '?recursive=true' : ''}`,
  );

export const addGroupMember = (groupId: string, userId: string) =>
  request<{ message: string }>(`/admin/groups/${enc(groupId)}/members`, { method: 'POST', body: { userId } });

export const removeGroupMember = (groupId: string, userId: string) =>
  request<{ message: string }>(`/admin/groups/${enc(groupId)}/members/${enc(userId)}`, { method: 'DELETE' });

export const groupMappings = () => request<{ mappings: GroupMapping[] | null }>('/admin/groups/mappings');

/** Creates or updates the mapping of an SSO group name. */
export const saveGroupMapping = (mapping: GroupMapping) =>
  request<{ message: string }>('/admin/groups/mappings', { method: 'POST', body: mapping });

export const deleteGroupMapping = (groupName: string) =>
  request<{ message: string }>(`/admin/groups/mappings/${enc(groupName)}`, { method: 'DELETE' });

// Learning paths (course-service paths_admin.go, user-service paths.go — docs/specs/043)

/** definition.Path */
export type PathSpec = {
  title?: string;
  description?: string;
  kind?: string;
  level?: string;
  courses?: string[];
  skills?: string[];
};

/** Read through the admin route: the public one adds the courses' skills (spec 043). */
export const pathDefinition = (slug: string) =>
  request<{ slug: string; spec: PathSpec }>(`/admin/courses/paths/${enc(slug)}/definition`);

export const createPath = (slug: string, spec: PathSpec) =>
  request<{ slug: string }>('/admin/courses/paths', { method: 'POST', body: { slug, spec } });

export const updatePath = (slug: string, spec: PathSpec) =>
  request<{ slug: string }>(`/admin/courses/paths/${enc(slug)}/definition`, { method: 'PUT', body: { spec } });

export const deletePath = (slug: string) =>
  request<{ message: string }>(`/admin/courses/paths/${enc(slug)}/definition`, { method: 'DELETE' });

export const adminPathEnrollments = (slug: string) =>
  request<{ users: PathEnrolledUser[] | null }>(`/admin/paths/${enc(slug)}/enrollments`);

export const adminEnrollPath = (slug: string, userId: string) =>
  request<{ message: string }>(`/admin/paths/${enc(slug)}/enrollments`, { method: 'POST', body: { userId } });

export const adminUnenrollPath = (slug: string, userId: string) =>
  request<{ message: string }>(`/admin/paths/${enc(slug)}/enrollments/${enc(userId)}`, { method: 'DELETE' });

// Courses (course-service admin_definition.go, courses_import.go, state.go;
// user-service admin.go enrollments — docs/specs/042)

/** definition.Course. Unknown fields are kept as read, so a save never drops them. */
export type CourseSpec = {
  title?: string;
  description?: string;
  public?: boolean;
  hidden?: boolean;
  category?: string;
  difficulty?: string;
  scope?: string;
  xpRequired?: number;
  inPerson?: boolean;
  badge?: { name: string; icon?: string };
  modules?: ModuleSpec[];
  [key: string]: unknown;
};

/** content.Module, same rule for unknown fields. */
export type ModuleSpec = {
  name: string;
  type: string;
  content?: string;
  src?: string;
  ref?: string;
  path?: string;
  hidden?: boolean;
  inline?: boolean;
  [key: string]: unknown;
};

export type CourseDefinition = { slug: string; spec: CourseSpec };

export const courseDefinition = (slug: string) => request<CourseDefinition>(`/admin/courses/${enc(slug)}/definition`);

export const createCourse = (slug: string, spec: CourseSpec) =>
  request<{ slug: string }>('/admin/courses', { method: 'POST', body: { slug, spec } });

/** Full replacement of the definition. */
export const updateCourse = (slug: string, spec: CourseSpec) =>
  request<{ slug: string }>(`/admin/courses/${enc(slug)}/definition`, { method: 'PUT', body: { spec } });

export const deleteCourse = (slug: string) =>
  request<{ message: string }>(`/admin/courses/${enc(slug)}/definition`, { method: 'DELETE' });

export type ImportRequest = { markdown: string; slug?: string; split?: string; mode: 'create' | 'replace' | 'append' };

export type ImportPreview = {
  slug: string;
  mode: string;
  course: CourseSpec;
  modules: { name: string; slug: string; type: string; bytes: number; hidden?: boolean; src?: string; path?: string; existing?: boolean }[];
  warnings?: string[];
};

export const importPreview = (body: ImportRequest) =>
  request<ImportPreview>('/admin/courses/import/preview', { method: 'POST', body });

export const importCourse = (body: ImportRequest) =>
  request<{ slug: string; mode: string; moduleCount: number; warnings?: string[] }>('/admin/courses/import', {
    method: 'POST',
    body,
  });

export const exportCourseMarkdown = (slug: string) => requestBlob(`/admin/courses/${enc(slug)}/export/markdown`);

export const clearAllCache = () => request<{ status: string; reposCleared?: number }>('/admin/cache/clear', { method: 'POST' });

export const clearCourseCache = (slug: string) =>
  request<{ status: string; reposCleared?: number }>(`/admin/courses/${enc(slug)}/cache/clear`, { method: 'POST' });

export const clearModuleCache = (slug: string, index: number) =>
  request<{ status: string; reposCleared?: number }>(`/admin/courses/${enc(slug)}/modules/${index}/cache/clear`, {
    method: 'POST',
  });

export type CourseEnrollment = { userId: string; username: string; email: string; enrolledAt: string };
export type GroupEnrollment = { id: string; name: string; source: string; memberCount: number; enrolledAt: string };

export const courseEnrollments = (slug: string) =>
  request<{ enrollments: CourseEnrollment[] | null }>(`/admin/enrollments/${enc(slug)}`);

export const groupEnrollments = (slug: string) =>
  request<{ groups: GroupEnrollment[] | null }>(`/admin/enrollments/${enc(slug)}/groups`);

/** The response gives the number of members enrolled. */
export const enrollGroup = (slug: string, groupId: string) =>
  request<{ message: string; enrolled: number }>(`/admin/enrollments/${enc(slug)}/groups`, { method: 'POST', body: { groupId } });

export const unenrollGroup = (slug: string, groupId: string) =>
  request<{ message: string }>(`/admin/enrollments/${enc(slug)}/groups/${enc(groupId)}`, { method: 'DELETE' });

export const searchUsers = (q: string) =>
  request<{ users: { id: string; username: string; email: string }[] | null }>(`/admin/users/search?q=${enc(q)}`);

export const adminEnrollUser = (slug: string, userId: string) =>
  request<{ message: string }>(`/admin/enrollments/${enc(slug)}`, { method: 'POST', body: { userId } });

export const adminUnenrollUser = (slug: string, userId: string) =>
  request<{ message: string }>(`/admin/enrollments/${enc(slug)}/users/${enc(userId)}`, { method: 'DELETE' });
