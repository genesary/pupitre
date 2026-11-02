import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { listCourses, myCourses, mySkills } from '@pupitre/core/lib/api';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /my-skills (docs/specs/020-learner-space.md), markup of pupitre's my-skills.astro.

const LEVELS: Record<string, { label: string; className: string }> = {
  beginner: { label: 'Débutant', className: 'level-beginner' },
  intermediate: { label: 'Intermédiaire', className: 'level-intermediate' },
  advanced: { label: 'Avancé', className: 'level-advanced' },
  'maître': { label: '★ Maître', className: 'level-maitre' },
};

/**
 * The Astro page recomputed the level from the ratio (defect A10). The server
 * value wins; this is only the fallback when it is missing.
 */
function fallbackLevel(completed: number, total: number): string {
  const ratio = total > 0 ? completed / total : 0;
  if (ratio >= 1) return 'maître';
  if (ratio >= 0.6) return 'advanced';
  if (ratio >= 0.3) return 'intermediate';
  return 'beginner';
}

function Progress({ completed, total, fillClass }: { completed: number; total: number; fillClass?: string }) {
  if (!total) return null;
  const pct = Math.min(100, Math.round((completed / total) * 100));
  return (
    <div className="ms-progress-wrap">
      <div className="ms-progress-bar">
        <div className={`ms-progress-fill${fillClass ? ` ${fillClass}` : ''}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="ms-progress-label">
        {completed}/{total} cours
      </span>
    </div>
  );
}

type SkillCardProps = { skill: string; level: string | null; completed: number; total: number };

function SkillCard({ skill, level, completed, total }: SkillCardProps) {
  const maitre = level === 'maître';
  const meta = level ? (LEVELS[level] ?? { label: level, className: '' }) : null;
  return (
    <Link to={`/skills/${skill}`} className={`ms-card${maitre ? ' card-maitre' : ''} no-underline text-inherit`}>
      <div className="ms-card-skill">{skill}</div>
      {meta ? (
        <div className={`ms-level ${meta.className}`}>{meta.label}</div>
      ) : (
        <div className="ms-level level-inprogress">En cours</div>
      )}
      <Progress
        completed={completed}
        total={total}
        fillClass={maitre ? 'fill-maitre' : meta ? undefined : 'fill-inprogress'}
      />
      <span className="ms-cta">Voir les modules →</span>
    </Link>
  );
}

export function MySkills() {
  const skills = useQuery({ queryKey: ['my', 'skills'], queryFn: mySkills });
  const mine = useQuery({ queryKey: ['my', 'courses'], queryFn: myCourses });
  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });

  const pending = skills.isPending || mine.isPending || courses.isPending;
  const error = skills.error ?? mine.error ?? courses.error;

  const all = courses.data?.courses ?? [];
  const totalBySkill = new Map<string, number>();
  for (const course of all) {
    for (const skill of course.skills ?? []) totalBySkill.set(skill, (totalBySkill.get(skill) ?? 0) + 1);
  }

  // user-service lists a skill once a course teaching it is completed.
  const acquired = [...(skills.data?.skills ?? [])]
    .filter((s) => s.completedCourses > 0)
    .sort((a, b) => a.skill.localeCompare(b.skill));
  const acquiredNames = new Set(acquired.map((s) => s.skill));

  const bySlug = new Map(all.map((c) => [c.slug, c]));
  const learning = [
    ...new Set((mine.data?.courses ?? []).flatMap((c) => bySlug.get(c.slug)?.skills ?? [])),
  ]
    .filter((skill) => !acquiredNames.has(skill))
    .sort((a, b) => a.localeCompare(b));

  return (
    <div className="page-outer">
      <div className="page-inner">
        <div className="flex items-baseline justify-between gap-4 mb-7 flex-wrap">
          <h1 data-tour="page-title" className="page-title">Mes compétences</h1>
        </div>

        {pending ? (
          <LoadingState />
        ) : error ? (
          <LoadError
            error={error}
            onRetry={() => {
              void skills.refetch();
              void mine.refetch();
              void courses.refetch();
            }}
          />
        ) : acquired.length === 0 && learning.length === 0 ? (
          <div className="text-center py-[60px] px-6 text-ink-soft">
            <div className="text-[3rem] mb-3">🎯</div>
            <h2 className="mb-2">Aucune compétence pour l'instant</h2>
            <p className="mb-5">Commence un cours pour voir tes compétences évoluer.</p>
            <Link to="/courses" className="btn btn-primary">
              Découvrir le catalogue
            </Link>
          </div>
        ) : (
          <div>
            {acquired.length > 0 && (
              <div>
                <div className="section-title">Acquises</div>
                <div className="auto-grid sm:grid-cols-2">
                  {acquired.map((s) => {
                    const total = s.totalCourses || totalBySkill.get(s.skill) || 0;
                    return (
                      <SkillCard
                        key={s.skill}
                        skill={s.skill}
                        level={s.level || fallbackLevel(s.completedCourses, total)}
                        completed={s.completedCourses}
                        total={total}
                      />
                    );
                  })}
                </div>
              </div>
            )}
            {learning.length > 0 && (
              <div>
                <div className="section-title">En apprentissage</div>
                <div className="auto-grid sm:grid-cols-2">
                  {learning.map((skill) => (
                    <SkillCard key={skill} skill={skill} level={null} completed={0} total={totalBySkill.get(skill) ?? 0} />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
