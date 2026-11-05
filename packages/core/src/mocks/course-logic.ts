import type {
  Course,
  InlineQuiz,
  ModuleDetail,
  ModuleSummary,
  MyCourse,
  QuestionResult,
  QuizAnswer,
  QuizQuestion,
  UserPublic,
} from '../lib/api-types';

import { bucket, db, type QuizProgress } from './db';
import { DEMO_COURSES, DEMO_PATHS } from './fixtures/demo-courses';
import { SEED_COURSES, SEED_PATHS } from './fixtures/seed-courses';
import type { CourseDefinition, ModuleDefinition, PathDefinition, QuestionDefinition } from './model';

// The course-service and user-service rules the mock mode needs, written after
// the Go handlers they mirror (course-service/internal/handlers/modules.go,
// courses.go, content/scoring.go; user-service enrollments.go).

/** The fixtures with the definitions an admin created, changed or deleted. */
function withOverrides<T extends { slug: string; spec: object }>(fixtures: T[], overrides: Record<string, T['spec'] | null>): T[] {
  const list = fixtures.flatMap((item) => {
    const override = overrides[item.slug];
    if (override === null) return [];
    return [override ? ({ ...item, spec: override } as T) : item];
  });
  for (const [slug, spec] of Object.entries(overrides)) {
    if (spec && !fixtures.some((item) => item.slug === slug)) list.push({ slug, spec } as T);
  }
  return list;
}

export function courses(): CourseDefinition[] {
  return withOverrides([...DEMO_COURSES, ...SEED_COURSES], db().courseDefs);
}

export function paths(): PathDefinition[] {
  return withOverrides([...SEED_PATHS, ...DEMO_PATHS], db().pathDefs);
}

/** repository/xp.go amounts. */
export const XP_AMOUNTS = { lesson: 10, module: 25, course: 200 } as const;

/** xp.go — awardXP: idempotent per (user, source, sourceSlug). */
export function awardXp(userId: string, source: keyof typeof XP_AMOUNTS, sourceSlug: string) {
  const events = (db().xp[userId] ??= []);
  if (events.some((e) => e.source === source && e.sourceSlug === sourceSlug)) return;
  events.push({ source, sourceSlug, amount: XP_AMOUNTS[source], earnedAt: new Date().toISOString() });
}

export function totalXp(userId: string): number {
  return (db().xp[userId] ?? []).reduce((sum, e) => sum + e.amount, 0);
}

/** Sessions as a trainer left them (sessions_admin.go), else those of the definition. */
export function sessionsOf(course: CourseDefinition) {
  return db().sessions[course.slug] ?? course.spec.sessions;
}
export function findCourse(slug: string): CourseDefinition | undefined {
  return courses().find((course) => course.slug === slug);
}

/** content.SlugifyModuleName: letters and digits kept, the rest becomes "-". */
export function moduleSlug(name: string): string {
  return name
    .replace(/[^\p{L}\p{N}_-]/gu, '-')
    .replace(/^-+|-+$/g, '')
    .toLowerCase();
}

export function isEnrolled(userId: string | undefined, slug: string): boolean {
  return Boolean(userId && db().enrollments[userId]?.[slug]);
}

/** Hidden modules are only returned to admins. */
export function visibleModules(course: CourseDefinition, isAdmin: boolean): ModuleDefinition[] {
  return (course.spec.modules ?? []).filter((module) => isAdmin || !module.hidden);
}

function courseSkills(course: CourseDefinition): string[] {
  return [...new Set((course.spec.modules ?? []).flatMap((module) => module.skills ?? []))];
}

function enrollmentCount(slug: string): number {
  return Object.values(db().enrollments).filter((courses) => courses[slug]).length;
}

/** courses.go — courseResponse */
export function toCourseResponse(course: CourseDefinition): Course {
  const { spec } = course;
  const moduleCount = (spec.modules ?? []).length;
  return {
    slug: course.slug,
    id: course.slug,
    title: spec.title,
    description: spec.description ?? '',
    category: spec.category ?? '',
    difficulty: spec.difficulty ?? '',
    isPublic: spec.public ?? false,
    moduleCount,
    labCount: moduleCount,
    enrollmentCount: enrollmentCount(course.slug),
    prerequisites: spec.prerequisites,
    skills: courseSkills(course),
    badge: spec.badge,
    inPerson: spec.inPerson,
    sessions: sessionsOf(course),
    xpRequired: spec.xpRequired,
  };
}

/** A private course is shown to its enrolled users and to admins only. */
export function canSeeCourse(course: CourseDefinition, user: UserPublic | null): boolean {
  if (course.spec.hidden && user?.role !== 'admin') return false;
  return Boolean(course.spec.public || user?.role === 'admin' || isEnrolled(user?.id, course.slug));
}

function quizProgress(userId: string, slug: string): Record<number, QuizProgress> {
  return bucket(bucket(db().quizzes, userId), slug) as unknown as Record<number, QuizProgress>;
}

function viewedSet(userId: string, slug: string): Set<string> {
  return new Set(db().viewed[userId]?.[slug] ?? []);
}

/** modules.go — completedSlugs: viewed lessons plus passed quizzes. */
export function completedSlugs(userId: string, course: CourseDefinition, modules: ModuleDefinition[]) {
  const done = viewedSet(userId, course.slug);
  const progress = quizProgress(userId, course.slug);
  modules.forEach((module, index) => {
    if (progress[index]?.passed) done.add(moduleSlug(module.name));
  });
  return done;
}

function questionPoints(question: QuestionDefinition) {
  return question.points ?? 1;
}

/** modules.go — sanitizeQuestions: correct answers for admins only. */
function publicQuestions(questions: QuestionDefinition[], isAdmin: boolean): QuizQuestion[] {
  return questions.map((question) => ({
    id: question.id,
    type: question.type,
    difficulty: question.difficulty,
    points: questionPoints(question),
    question: question.question,
    answers: question.answers?.map((answer) =>
      isAdmin ? { id: answer.id, text: answer.text, correct: answer.correct } : { id: answer.id, text: answer.text },
    ),
    items: question.items,
    sourceRefs: question.feedback?.sourceRefs,
  }));
}

/** modules.go — ListModules, with the sequential and prerequisite locking. */
export function listModules(course: CourseDefinition, user: UserPublic): ModuleSummary[] {
  const isAdmin = user.role === 'admin';
  const modules = visibleModules(course, isAdmin);
  const done = completedSlugs(user.id, course, modules);
  const viewed = viewedSet(user.id, course.slug);
  const progress = quizProgress(user.id, course.slug);
  let previousDone = true;

  return modules.map((module, index) => {
    const slug = moduleSlug(module.name);
    const explicitLocked = (module.prerequisites ?? []).some((prereq) => !done.has(prereq));
    const sequentialLocked = !module.inline && !previousDone;
    const locked = !isAdmin && (explicitLocked || sequentialLocked);
    if (!module.inline) previousDone = done.has(slug);

    const quiz = progress[index];
    const summary: ModuleSummary = {
      index,
      name: module.name,
      slug,
      type: module.type,
      viewed: viewed.has(slug),
      hidden: module.hidden ?? false,
      locked,
      inline: module.inline || undefined,
      prerequisites: module.prerequisites,
      bestScore: quiz?.bestScore,
      maxScore: quiz?.maxScore,
      passed: quiz?.passed,
      attempts: quiz?.attempts,
      skills: module.skills,
    };
    if (module.type === 'lab' && !locked) summary.labUrl = module.labUrl;
    if (module.checkProvider) {
      summary.hasCheck = true;
      summary.checkProvider = module.checkProvider;
    }
    return summary;
  });
}

function cooldownsFor(user: UserPublic, slug: string, index: number) {
  const states = db().attempts[`${user.id}|${slug}|${index}`] ?? {};
  const now = Date.now();
  const out: Record<string, { remainingSeconds: number; attempts: number; locked: boolean }> = {};
  for (const [id, state] of Object.entries(states)) {
    const remaining = Math.max(0, Math.ceil((state.until - now) / 1000));
    if (remaining > 0 || state.locked) out[id] = { remainingSeconds: remaining, attempts: state.attempts, locked: state.locked };
  }
  return out;
}

function quizConfig(module: ModuleDefinition) {
  return {
    passingScore: module.passingScore ?? 0,
    cooldown: module.cooldown,
    maxAttemptsPerQuestion: module.maxAttemptsPerQuestion,
    lockOnMaxAttempts: module.lockOnMaxAttempts ?? false,
  };
}

/** modules.go — buildInlineQuizResponse: the quiz right after this module, if inline. */
function inlineQuiz(course: CourseDefinition, modules: ModuleDefinition[], index: number, user: UserPublic): InlineQuiz | undefined {
  const next = modules[index + 1];
  if (!next || next.type !== 'quiz' || !next.inline || !next.questions?.length) return undefined;
  const progress = quizProgress(user.id, course.slug)[index + 1];
  return {
    index: index + 1,
    name: next.name,
    slug: moduleSlug(next.name),
    questions: publicQuestions(next.questions, user.role === 'admin'),
    questionCount: next.questions.length,
    passingScore: next.passingScore ?? 0,
    maxScore: next.questions.reduce((sum, question) => sum + questionPoints(question), 0),
    bestScore: progress?.bestScore,
    attempts: progress?.attempts,
    passed: progress?.passed ?? false,
    quizConfig: quizConfig(next),
  };
}

function gitPlaceholder(module: ModuleDefinition) {
  return `## ${module.name}\n\n:::info Contenu git\nEn production, ce module est lu depuis \`${module.src}\` (\`${module.path}\`, branche \`${module.ref ?? 'main'}\`). Le mode mock ne clone pas de dépôt.\n:::\n`;
}

/** modules.go — GetModule */
export function moduleDetail(course: CourseDefinition, index: number, user: UserPublic): ModuleDetail | undefined {
  const modules = visibleModules(course, user.role === 'admin');
  const module = modules[index];
  if (!module) return undefined;
  const summary = listModules(course, user)[index]!;
  const detail: ModuleDetail = { ...summary };

  if (module.type === 'quiz') {
    detail.questions = publicQuestions(module.questions ?? [], user.role === 'admin');
    detail.questionCount = module.questions?.length ?? 0;
    detail.quizConfig = quizConfig(module);
    detail.cooldowns = cooldownsFor(user, course.slug, index);
  } else if (module.src && !module.content) {
    detail.content = gitPlaceholder(module);
  } else {
    detail.content = module.content;
  }
  if (module.checkProvider) {
    detail.checkType = module.checkType;
    detail.checkParams = module.checkParams;
    detail.steps = module.steps;
  }
  detail.inlineQuiz = inlineQuiz(course, modules, index, user);
  return detail;
}

// ── Scoring (content/scoring.go, simplified: no partial scoring) ────────────

function isCorrect(question: QuestionDefinition, answer: QuizAnswer | undefined): boolean {
  if (!answer) return false;
  const correctIds = (question.answers ?? []).filter((a) => a.correct).map((a) => a.id);
  switch (question.type) {
    case 'single':
      return 'single' in answer && answer.single !== null && correctIds.includes(answer.single);
    case 'multiple':
      return (
        'multiple' in answer &&
        answer.multiple.length === correctIds.length &&
        answer.multiple.every((id) => correctIds.includes(id))
      );
    case 'boolean':
      return 'boolean' in answer && answer.boolean === question.correctAnswer;
    case 'order': {
      const expected = question.correctOrder ?? question.items?.map((item) => item.id) ?? [];
      return 'order' in answer && answer.order.join('|') === expected.join('|');
    }
  }
}

/** modules.go — finalizeSubmission, with recordQuestionCooldowns. */
export function scoreQuiz(
  course: CourseDefinition,
  index: number,
  user: UserPublic,
  answers: Record<string, QuizAnswer>,
): { status: 200; body: unknown } | { status: 425; body: unknown } {
  const module = visibleModules(course, user.role === 'admin')[index]!;
  const questions = module.questions ?? [];
  const pending = Object.fromEntries(
    Object.entries(cooldownsFor(user, course.slug, index)).filter(([, cd]) => cd.remainingSeconds > 0),
  );
  if (Object.keys(pending).length > 0) {
    return { status: 425, body: { error: 'Cooldown active for some questions', cooldowns: pending } };
  }

  const results: QuestionResult[] = questions.map((question) => {
    const correct = isCorrect(question, answers[question.id]);
    const points = questionPoints(question);
    return {
      questionId: question.id,
      type: question.type,
      isCorrect: correct,
      pointsEarned: correct ? points : 0,
      pointsMax: points,
      feedback: (correct ? question.feedback?.correct : question.feedback?.wrong) || undefined,
      sourceRefs: question.feedback?.sourceRefs,
    };
  });
  const totalScore = results.reduce((sum, r) => sum + r.pointsEarned, 0);
  const maxScore = results.reduce((sum, r) => sum + r.pointsMax, 0);
  const passed = maxScore > 0 && (totalScore / maxScore) * 100 >= (module.passingScore ?? 0);

  // Cooldowns: a wrong answer waits base × multiplier^(attempts-1), capped.
  const key = `${user.id}|${course.slug}|${index}`;
  const states = bucket(db().attempts, key);
  for (const result of results) {
    if (result.isCorrect) {
      delete states[result.questionId];
      continue;
    }
    const attempts = (states[result.questionId]?.attempts ?? 0) + 1;
    const spec = module.cooldown;
    const seconds = spec
      ? Math.min(spec.baseSeconds * spec.multiplier ** (attempts - 1), spec.maxSeconds)
      : 0;
    const locked =
      Boolean(module.lockOnMaxAttempts) &&
      module.maxAttemptsPerQuestion !== undefined &&
      attempts >= module.maxAttemptsPerQuestion;
    states[result.questionId] = { attempts, until: Date.now() + seconds * 1000, locked };
  }

  const progress = quizProgress(user.id, course.slug);
  const previous = progress[index];
  if (passed) awardXp(user.id, 'module', `${course.slug}/${moduleSlug(module.name)}`);
  progress[index] = {
    bestScore: Math.max(previous?.bestScore ?? 0, totalScore),
    maxScore,
    passed: Boolean(previous?.passed) || passed,
    attempts: (previous?.attempts ?? 0) + 1,
  };
  touch(user.id, course.slug);
  awardBadgeIfComplete(course, user);

  return {
    status: 200,
    body: { totalScore, maxScore, passed, questionResults: results, cooldowns: cooldownsFor(user, course.slug, index) },
  };
}

function touch(userId: string, slug: string) {
  bucket(db().activity, userId)[slug] = new Date().toISOString();
}

/** lessons.go — MarkLessonComplete */
export function markViewed(course: CourseDefinition, moduleSlugValue: string, user: UserPublic) {
  const viewed = bucket(db().viewed, user.id);
  const list = (viewed[course.slug] ??= []);
  if (!list.includes(moduleSlugValue)) list.push(moduleSlugValue);
  awardXp(user.id, 'lesson', `${course.slug}/${moduleSlugValue}`);
  touch(user.id, course.slug);
  awardBadgeIfComplete(course, user);
}

export function isCourseComplete(course: CourseDefinition, user: UserPublic) {
  const modules = visibleModules(course, false);
  const done = completedSlugs(user.id, course, modules);
  return modules.length > 0 && modules.every((module) => done.has(moduleSlug(module.name)));
}

/** internal.go — course completion: 200 XP, and the badge when the course has one. */
function awardBadgeIfComplete(course: CourseDefinition, user: UserPublic) {
  if (!isCourseComplete(course, user)) return;
  awardXp(user.id, 'course', course.slug);
  if (course.spec.badge) bucket(db().badges, user.id)[course.slug] ??= new Date().toISOString();
}

/** user-service enrollments.go — MyCourses */
export function myCourses(user: UserPublic): MyCourse[] {
  const enrolled = db().enrollments[user.id] ?? {};
  return Object.keys(enrolled)
    .map((slug) => findCourse(slug))
    .filter((course): course is CourseDefinition => Boolean(course))
    .map((course) => {
      const modules = visibleModules(course, false);
      const done = completedSlugs(user.id, course, modules);
      const progress = quizProgress(user.id, course.slug);
      const response = toCourseResponse(course);
      return {
        slug: course.slug,
        id: course.slug,
        title: response.title,
        description: response.description,
        category: response.category,
        difficulty: response.difficulty,
        isPublic: response.isPublic,
        labCount: response.labCount,
        enrollmentCount: response.enrollmentCount,
        completedLabs: modules.filter((m) => done.has(moduleSlug(m.name))).length,
        totalScore: Object.values(progress).reduce((sum, p) => sum + p.bestScore, 0),
        lastActivity: db().activity[user.id]?.[course.slug] ?? null,
        completed: Boolean(db().badges[user.id]?.[course.slug]) || isCourseComplete(course, user),
      };
    });
}

/** Courses the user completed: a badge earned or every module done. */
export function completedCourses(user: UserPublic): Set<string> {
  return new Set(myCourses(user).filter((c) => c.completed).map((c) => c.slug));
}

/** catalog.go — SkillModules: the visible modules tagged with the skill, course by course. */
export function skillModules(skill: string) {
  return courses().filter((course) => !course.spec.hidden && course.spec.public).flatMap((course) =>
    visibleModules(course, false).flatMap((module, index) =>
      (module.skills ?? []).includes(skill)
        ? [{ name: module.name, slug: moduleSlug(module.name), index, type: module.type, courseSlug: course.slug, courseTitle: course.spec.title }]
        : [],
    ),
  );
}

/** skills.go — a module counts as done once its quiz is passed or its lesson viewed. */
export function isModuleDone(user: UserPublic, courseSlug: string, module: { slug: string; index: number; type: string }) {
  if (module.type === 'quiz') return Boolean(quizProgress(user.id, courseSlug)[module.index]?.passed);
  return viewedSet(user.id, courseSlug).has(module.slug);
}
