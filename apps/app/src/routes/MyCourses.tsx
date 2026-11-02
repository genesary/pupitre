import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router';

import { myCourses } from '@pupitre/core/lib/api';
import type { MyCourse } from '@pupitre/core/lib/api-types';
import { courseIcon } from '@pupitre/core/lib/course-meta';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /my-courses (docs/specs/020-learner-space.md), markup of pupitre's my-courses.astro.

type Status = 'not-started' | 'in-progress' | 'completed';

/** Capped at 100: a course can lose modules after the learner completed them. */
function percent(course: MyCourse): number {
  if (!course.labCount || course.labCount <= 0) return 0;
  return Math.min(100, Math.round((course.completedLabs / course.labCount) * 100));
}

function statusOf(course: MyCourse): Status {
  if (course.completed) return 'completed';
  return percent(course) > 0 ? 'in-progress' : 'not-started';
}

const STATUS_BADGES: Record<Status, { className: string; label: string }> = {
  completed: { className: 'badge-green', label: '✓ Terminé' },
  'in-progress': { className: 'badge-blue', label: 'En cours' },
  'not-started': { className: 'badge-gray', label: 'Non commencé' },
};

const CTA: Record<Status, string> = {
  'not-started': 'Commencer ▸',
  'in-progress': 'Continuer ▸',
  completed: 'Revoir ▸',
};

function MyCourseCard({ course }: { course: MyCourse }) {
  const pct = percent(course);
  const status = statusOf(course);
  const badge = STATUS_BADGES[status];
  return (
    <div className="glass-card flex flex-col transition-[border-color,box-shadow,transform] duration-150 hover:-translate-y-[3px]">
      <div className="course-card-thumb w-full overflow-hidden border-b-[1.5px] border-line shrink-0">
        {courseIcon(course)}
      </div>
      <div className="p-4 flex flex-col gap-2.5 flex-1">
        <div className="flex items-start justify-between gap-2">
          <div className="font-semibold text-[.95rem] leading-snug line-clamp-2">{course.title}</div>
          <span className={`badge ${badge.className} text-[.7rem]`}>{badge.label}</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="progress-bar flex-1">
            <div className="progress-bar-fill" style={{ width: `${pct}%` }} />
          </div>
          <span className="text-[.75rem] font-semibold text-blue shrink-0 min-w-[32px] text-right">{pct}%</span>
        </div>
        <div className="text-[.78rem] text-ink-soft">
          {Math.min(course.completedLabs ?? 0, course.labCount ?? 0)}/{course.labCount ?? 0} modules terminés
        </div>
        <div className="mt-auto flex gap-2">
          <Link to={`/courses/${course.slug}`} className="btn btn-primary btn-sm flex-1 justify-center">
            {CTA[status]}
          </Link>
          <Link to={`/courses/${course.slug}`} className="btn btn-secondary btn-sm">
            Détails
          </Link>
        </div>
      </div>
    </div>
  );
}

export function MyCourses() {
  const mine = useQuery({ queryKey: ['my', 'courses'], queryFn: myCourses });
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState('');
  const [progress, setProgress] = useState<'all' | Status>('all');

  // The search applies after 250 ms, like the Astro page.
  useEffect(() => {
    const timer = setTimeout(() => setQuery(search), 250);
    return () => clearTimeout(timer);
  }, [search]);

  const courses = useMemo(() => mine.data?.courses ?? [], [mine.data]);
  const visible = useMemo(() => {
    const q = query.toLowerCase().trim();
    return courses
      .filter((c) => !q || c.title.toLowerCase().includes(q) || (c.description || '').toLowerCase().includes(q))
      .filter((c) => progress === 'all' || statusOf(c) === progress);
  }, [courses, query, progress]);

  const totalModules = courses.reduce((sum, c) => sum + (c.labCount || 0), 0);
  const doneModules = courses.reduce((sum, c) => sum + Math.min(c.completedLabs || 0, c.labCount || 0), 0);
  const completedCourses = courses.filter((c) => c.completed).length;
  const overall = totalModules > 0 ? Math.min(100, Math.round((doneModules / totalModules) * 100)) : 0;

  return (
    <div className="page-outer">
      <div className="page-inner">
        <div className="flex items-baseline justify-between gap-4 mb-7 flex-wrap">
          <h1 className="page-title">Mes cours</h1>
          <Link to="/courses" className="btn btn-secondary btn-sm">
            Parcourir le catalogue
          </Link>
        </div>

        {mine.isPending ? (
          <LoadingState />
        ) : mine.isError ? (
          <LoadError error={mine.error} onRetry={() => mine.refetch()} />
        ) : courses.length === 0 ? (
          <div className="text-center py-20 px-6">
            <div className="text-[3.5rem] mb-4">📚</div>
            <h2 className="text-[1.2rem] mb-2">Vous n'êtes inscrit à aucun cours</h2>
            <p className="text-ink-soft text-[.9rem] mb-5">
              Commencez par parcourir le catalogue et inscrivez-vous à vos premiers cours.
            </p>
            <Link to="/courses" className="btn btn-primary">
              Découvrir le catalogue
            </Link>
          </div>
        ) : (
          <div>
            <div className="flex gap-4 flex-wrap mb-7">
              <div className="stat-card">
                <span className="stat-val">{courses.length}</span>
                <span className="stat-label">cours inscrits</span>
              </div>
              <div className="stat-card">
                <span className="stat-val">{completedCourses}</span>
                <span className="stat-label">cours terminés</span>
              </div>
              <div className="stat-card">
                <span className="stat-val">{doneModules}</span>
                <span className="stat-label">modules terminés</span>
              </div>
              <div className="stat-card">
                <span className="stat-val">{overall}%</span>
                <span className="stat-label">progression globale</span>
              </div>
            </div>

            <div className="flex items-center gap-3 mb-5 flex-wrap">
              <div className="relative flex-1 min-w-[180px] sm:min-w-0">
                <svg className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft pointer-events-none" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="11" cy="11" r="8" />
                  <path d="m21 21-4.35-4.35" />
                </svg>
                <input
                  className="input pl-9"
                  type="search"
                  placeholder="Filtrer mes cours…"
                  aria-label="Filtrer mes cours"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
              </div>
              <select
                className="input w-auto shrink-0"
                aria-label="Progression"
                value={progress}
                onChange={(event) => setProgress(event.target.value as 'all' | Status)}
              >
                <option value="all">Tous</option>
                <option value="in-progress">En cours</option>
                <option value="completed">Terminés</option>
                <option value="not-started">Non commencés</option>
              </select>
              <span className="text-[.875rem] text-ink-soft whitespace-nowrap">{visible.length} cours</span>
            </div>

            {visible.length === 0 ? (
              <div className="text-center py-10 text-ink-soft">
                <div className="text-[2rem] mb-2.5">🔍</div>
                <p>Aucun cours ne correspond à votre recherche.</p>
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-[18px]">
                {visible.map((course) => (
                  <MyCourseCard key={course.slug} course={course} />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
