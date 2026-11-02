import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { listCourses, myBadges } from '@pupitre/core/lib/api';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';

// /my-badges (docs/specs/020-learner-space.md), markup of pupitre's my-badges.astro.
// The possible badges are those of the courses.

const plural = (count: number, word: string) => `${word}${count !== 1 ? 's' : ''}`;

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' });
}

export function MyBadges() {
  const mine = useQuery({ queryKey: ['my', 'badges'], queryFn: myBadges });
  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });

  const earned = mine.data?.badges ?? [];
  const all = courses.data?.courses ?? [];
  const earnedSlugs = new Set(earned.map((b) => b.courseSlug));
  const titleOf = (slug: string) => all.find((c) => c.slug === slug)?.title || slug;
  const available = all.flatMap((c) => (c.badge ? [{ ...c.badge, courseSlug: c.slug, courseTitle: c.title }] : []));
  const locked = available.filter((b) => !earnedSlugs.has(b.courseSlug));
  const remaining = Math.max(available.length - earned.length, 0);

  const pending = mine.isPending || courses.isPending;
  const error = mine.error ?? courses.error;

  return (
    <div className="page-outer">
      <div className="page-inner">
        <div className="mb-8">
          <h1 data-tour="page-title" className="page-title mb-1">Mes badges</h1>
          <p className="text-[.9rem] text-ink-soft">
            {pending || error
              ? 'Chargement…'
              : `${earned.length} ${plural(earned.length, 'badge')} obtenus sur ${available.length} disponibles`}
          </p>
        </div>

        {pending ? (
          <LoadingState />
        ) : error ? (
          <LoadError
            error={error}
            onRetry={() => {
              void mine.refetch();
              void courses.refetch();
            }}
          />
        ) : (
          <div>
            <div className="flex gap-6 flex-wrap mb-8">
              <div className="stat-card items-center">
                <span className="stat-val">{earned.length}</span>
                <span className="stat-label">
                  {plural(earned.length, 'Badge')} {plural(earned.length, 'obtenu')}
                </span>
              </div>
              <div className="stat-card items-center">
                <span className="stat-val">{remaining}</span>
                <span className="stat-label">{plural(remaining, 'Badge')} à débloquer</span>
              </div>
              <div className="stat-card items-center">
                <span className="stat-val">
                  {available.length > 0 ? Math.round((earned.length / available.length) * 100) : 0}%
                </span>
                <span className="stat-label">Complété</span>
              </div>
            </div>

            {earned.length > 0 && (
              <div>
                <div className="section-title">Obtenus ({earned.length})</div>
                <div className="auto-grid">
                  {earned.map((badge) => (
                    <Link key={badge.courseSlug} to={`/courses/${badge.courseSlug}`} className="mb-card">
                      <span className="mb-card-ribbon">✓ Obtenu</span>
                      <div className="mb-card-icon">{badge.icon || '🏅'}</div>
                      <div className="mb-card-name">{badge.name || badge.courseSlug}</div>
                      <div className="mb-card-course">{titleOf(badge.courseSlug)}</div>
                      <div className="mb-card-earned-by">
                        {badge.earnedBy} apprenant{badge.earnedBy > 1 ? 's' : ''}
                      </div>
                      <div className="mb-card-date">Obtenu le {formatDate(badge.earnedAt)}</div>
                    </Link>
                  ))}
                </div>
              </div>
            )}

            {locked.length > 0 && (
              <div>
                <div className="section-title">À débloquer ({locked.length})</div>
                <div className="auto-grid">
                  {locked.map((badge) => (
                    <Link
                      key={badge.courseSlug}
                      to={`/courses/${badge.courseSlug}`}
                      className="mb-card-locked no-underline text-inherit"
                    >
                      <div className="mb-card-locked-icon">{badge.icon || '🏅'}</div>
                      <div className="mb-card-locked-name">{badge.name || badge.courseSlug}</div>
                      <div className="mb-card-locked-course">{badge.courseTitle}</div>
                      <div className="mb-lock-label">🔒 Non obtenu</div>
                    </Link>
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
