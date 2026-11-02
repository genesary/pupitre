import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { listCourses, myCourses, myXp } from '@pupitre/core/lib/api';
import { COURSE_XP } from '@pupitre/core/lib/course-meta';
import { XP_LEVELS, XP_SOURCE_ICONS, XP_SOURCE_LABELS, xpProgress } from '@/lib/xp';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /my-xp (docs/specs/020-learner-space.md), markup of pupitre's my-xp.astro.

const courseOf = (sourceSlug: string) => sourceSlug.split('/')[0] ?? '';

export function MyXp() {
  const xp = useQuery({ queryKey: ['my', 'xp'], queryFn: myXp });
  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });
  const mine = useQuery({ queryKey: ['my', 'courses'], queryFn: myCourses });

  const total = xp.data?.total ?? 0;
  const level = xp.data?.level ?? 1;
  const levelName = xp.data?.levelName ?? 'Novice';
  const progress = xpProgress(total, level);
  const history = xp.data?.history ?? [];

  const titles = new Map((courses.data?.courses ?? []).map((c) => [c.slug, c.title || c.slug]));
  const titleOf = (slug: string, fallback?: string) => titles.get(slug) || fallback || slug.replace(/-/g, ' ');

  // A course's completion is the only 200 XP event (xp.go award amounts).
  const completed = new Set(history.filter((e) => e.amount === COURSE_XP).map((e) => courseOf(e.sourceSlug)));
  const inProgress = (mine.data?.courses ?? []).filter((c) => !c.completed && !completed.has(c.slug));

  return (
    <div className="page-outer">
      <div className="page-inner">
        <h1 data-tour="page-title" className="page-title mb-1">Mon XP</h1>
        <p className="text-[.9rem] text-ink-soft mb-7">
          {xp.isSuccess ? `${total.toLocaleString('fr-FR')} points accumulés` : 'Chargement…'}
        </p>

        {xp.isPending ? (
          <LoadingState />
        ) : xp.isError ? (
          <LoadError error={xp.error} onRetry={() => xp.refetch()} />
        ) : (
          <div>
            <div className="xp-hero">
              <div className="xp-orb">⚡</div>
              <div className="flex-1 min-w-0">
                <div className="xp-total">{total.toLocaleString('fr-FR')}</div>
                <div className="xp-total-label">points d'expérience</div>
                <div className="xp-level-chip">
                  Niveau {level} — {levelName}
                </div>
                <div
                  className="xp-progress-bar"
                  role="progressbar"
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-valuenow={progress.percent}
                >
                  <div className="xp-progress-fill" style={{ width: `${progress.percent}%` }} />
                </div>
                <div className="xp-next-label">
                  {progress.next
                    ? `${progress.missing} XP jusqu'au niveau ${progress.next.level} (${progress.next.name})`
                    : 'Niveau maximum atteint 🏆'}
                </div>
              </div>
            </div>

            <div className="xp-levels">
              {XP_LEVELS.map((entry) => (
                <div key={entry.level} className={`xp-level-card${entry.level === level ? ' current' : ''}`}>
                  <div className="xp-level-icon">{entry.icon}</div>
                  <div className="xp-level-name">{entry.name}</div>
                  <div className="xp-level-thresh">
                    {entry.threshold === 0 ? 'Départ' : `${entry.threshold.toLocaleString('fr-FR')} XP`}
                  </div>
                </div>
              ))}
            </div>

            {inProgress.length > 0 && (
              <div>
                <div className="section-title">
                  En cours{' '}
                  <span className="text-[.75rem] font-normal normal-case tracking-normal text-ink-soft">
                    ({inProgress.length} cours)
                  </span>
                </div>
                {inProgress.map((course) => (
                  <Link key={course.slug} to={`/courses/${course.slug}`} className="xp-hist-item">
                    <div className="xp-hist-icon xp-hist-icon-inprogress">📖</div>
                    <div className="xp-hist-meta">
                      <div className="xp-hist-slug">{titleOf(course.slug, course.title)}</div>
                      <div className="xp-hist-source">En cours · Compléter pour +{COURSE_XP} XP</div>
                    </div>
                    <div className="text-[.82rem] font-semibold text-slate-400 whitespace-nowrap">→</div>
                  </Link>
                ))}
              </div>
            )}

            <div className="section-title">Historique des gains</div>
            {history.length === 0 ? (
              <div className="xp-empty">
                Aucun XP gagné pour l'instant.
                <br />
                Complète des leçons, modules et cours pour en gagner !
              </div>
            ) : (
              history.map((event, index) => {
                const slug = courseOf(event.sourceSlug);
                const date = new Date(event.earnedAt).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' });
                return (
                  <Link key={`${event.sourceSlug}-${index}`} to={`/courses/${slug}`} className="xp-hist-item">
                    <div className="xp-hist-icon">{XP_SOURCE_ICONS[event.source] ?? '⚡'}</div>
                    <div className="xp-hist-meta">
                      <div className="xp-hist-slug">{titleOf(slug)}</div>
                      <div className="xp-hist-source">
                        {XP_SOURCE_LABELS[event.source] ?? event.source} · {date}
                      </div>
                    </div>
                    <div className="xp-hist-amount">+{event.amount} XP</div>
                  </Link>
                );
              })
            )}
          </div>
        )}
      </div>
    </div>
  );
}
