import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router';

import { listCourses, myCourses, mySkillModules } from '@pupitre/core/lib/api';
import type { Course } from '@pupitre/core/lib/api-types';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /skills/:slug (docs/specs/020-learner-space.md), markup of pupitre's skills/[slug].astro.

const DIFFICULTIES: Record<string, { label: string; className: string }> = {
  beginner: { label: 'Débutant', className: 'diff-beginner' },
  intermediate: { label: 'Intermédiaire', className: 'diff-intermediate' },
  advanced: { label: 'Avancé', className: 'diff-advanced' },
};

const LEVEL_SECTIONS = [
  { key: 'beginner', label: 'Débutant' },
  { key: 'intermediate', label: 'Intermédiaire' },
  { key: 'advanced', label: 'Avancé' },
  { key: '', label: 'Autre' },
];

const DIFFICULTY_ORDER: Record<string, number> = { beginner: 0, intermediate: 1, advanced: 2 };

const skillLabel = (slug: string) =>
  slug
    .split('-')
    .map((word) => (word[0]?.toUpperCase() ?? '') + word.slice(1))
    .join(' ');

type CourseCardProps = {
  course: Course;
  modules: number;
  done: number;
  completed: boolean;
  enrolled: boolean;
};

function SkillCourseCard({ course, modules, done, completed, enrolled }: CourseCardProps) {
  const difficulty = DIFFICULTIES[course.difficulty] ?? { label: course.difficulty || '', className: 'diff-default' };
  const state = completed ? ' card-done' : enrolled ? ' card-ongoing' : '';
  const pct = modules > 0 ? Math.round((done / modules) * 100) : 0;
  return (
    <Link to={`/courses/${course.slug}`} className={`sk-card${state}`}>
      <div className="sk-card-title">{course.title}</div>
      {course.description && <div className="sk-card-desc">{course.description}</div>}
      <div className="sk-card-meta">
        {difficulty.label && <span className={`badge-diff ${difficulty.className}`}>{difficulty.label}</span>}
        {completed ? (
          <span className="badge-status badge-done">✓ Terminé</span>
        ) : enrolled ? (
          <span className="badge-status badge-ongoing">En cours</span>
        ) : null}
        {modules > 0 && (
          <span className="sk-modules-hint">
            · {modules} module{modules > 1 ? 's' : ''} sur cette compétence
          </span>
        )}
      </div>
      {!completed && enrolled && modules > 0 && (
        <div className="cc-progress-wrap">
          <div className="cc-progress-bar">
            <div className="cc-progress-fill" style={{ width: `${pct}%` }} />
          </div>
          <span className="cc-progress-label">
            {done}/{modules} modules
          </span>
        </div>
      )}
      {completed ? (
        <span className="cc-cta">Revoir →</span>
      ) : enrolled ? (
        <span className="cc-cta">Continuer →</span>
      ) : null}
    </Link>
  );
}

export function SkillPage() {
  const { slug = '' } = useParams();
  const navigate = useNavigate();
  const skill = useQuery({ queryKey: ['my', 'skills', slug], queryFn: () => mySkillModules(slug) });
  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });
  const mine = useQuery({ queryKey: ['my', 'courses'], queryFn: myCourses });

  const label = skillLabel(slug);
  const pending = skill.isPending || courses.isPending || mine.isPending;
  const error = skill.error ?? courses.error ?? mine.error;

  const modules = skill.data?.modules ?? [];
  const doneModules = modules.filter((m) => m.status === 'completed').length;
  const pct = modules.length > 0 ? Math.round((doneModules / modules.length) * 100) : 0;
  const colour = pct === 100 ? 'bg-good text-good' : 'text-blue';

  // Per course: modules teaching the skill, and how many are completed.
  const perCourse = new Map<string, { total: number; done: number }>();
  for (const module of modules) {
    const entry = perCourse.get(module.courseSlug) ?? { total: 0, done: 0 };
    entry.total += 1;
    if (module.status === 'completed') entry.done += 1;
    perCourse.set(module.courseSlug, entry);
  }

  const enrolled = new Set((mine.data?.courses ?? []).map((c) => c.slug));
  const completed = new Set((mine.data?.courses ?? []).filter((c) => c.completed).map((c) => c.slug));
  const statusOrder = (c: Course) => (completed.has(c.slug) ? 2 : enrolled.has(c.slug) ? 0 : 1);
  const related = (courses.data?.courses ?? [])
    .filter((c) => (c.skills ?? []).includes(slug))
    .sort(
      (a, b) =>
        (DIFFICULTY_ORDER[a.difficulty] ?? 99) - (DIFFICULTY_ORDER[b.difficulty] ?? 99) || statusOrder(a) - statusOrder(b),
    );
  const sectionOf = (c: Course) => (c.difficulty in DIFFICULTY_ORDER ? c.difficulty : '');

  return (
    <div className="skill-outer">
      <div className="page-inner">
        <button onClick={() => navigate(-1)} className="skill-back">
          ◂ Retour
        </button>
        <h1 className="skill-page-title">{label}</h1>

        {pending ? (
          <LoadingState />
        ) : error ? (
          <LoadError
            error={error}
            onRetry={() => {
              void skill.refetch();
              void courses.refetch();
              void mine.refetch();
            }}
          />
        ) : (
          <div>
            {modules.length > 0 && (
              <div className="skill-prog mb-5">
                <div className="progress-bar flex-1">
                  <div className={`progress-bar-fill ${colour}`} style={{ width: `${pct}%` }} />
                </div>
                <span className={`skill-prog-pct ${colour}`}>{pct}%</span>
                <span>
                  {doneModules}/{modules.length} modules complétés
                </span>
              </div>
            )}

            {related.length === 0 ? (
              <div className="skill-empty">Aucun cours trouvé pour cette compétence.</div>
            ) : (
              <>
                <div className="text-[1.1rem] font-bold mb-1 mt-7">
                  {related.length} cours impliqué{related.length > 1 ? 's' : ''}
                </div>
                <div className="text-[.85rem] text-ink-soft mb-2">
                  Ces cours contiennent des modules qui développent la compétence « {label} »
                </div>
                {LEVEL_SECTIONS.map((section) => {
                  const list = related.filter((c) => sectionOf(c) === section.key);
                  if (list.length === 0) return null;
                  return (
                    <div key={section.key} className="level-section">
                      <div className="level-section-title">{section.label}</div>
                      <div className="sk-courses-grid">
                        {list.map((course) => (
                          <SkillCourseCard
                            key={course.slug}
                            course={course}
                            modules={perCourse.get(course.slug)?.total ?? 0}
                            done={perCourse.get(course.slug)?.done ?? 0}
                            completed={completed.has(course.slug)}
                            enrolled={enrolled.has(course.slug)}
                          />
                        ))}
                      </div>
                    </div>
                  );
                })}
                <div className="mt-6">
                  <Link to={`/courses?skill=${encodeURIComponent(slug)}`} className="btn btn-secondary btn-sm">
                    Voir les cours de ce niveau
                  </Link>
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
