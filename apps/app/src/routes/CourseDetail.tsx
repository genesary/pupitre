import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router';
import { toast } from 'sonner';

import { ApiError } from '@pupitre/core/lib/api-client';
import {
  enroll,
  getCourse,
  listModules,
  myBadges,
  myCourses,
  mySessionBookings,
} from '@pupitre/core/lib/api';
import type { Course, ModuleSummary } from '@pupitre/core/lib/api-types';
import { loginPath } from '@pupitre/core/lib/auth-paths';
import { useSession } from '@pupitre/core/lib/session';

import { COURSE_XP, courseIcon, DIFFICULTY_BADGES, DIFFICULTY_LABELS } from '@pupitre/core/lib/course-meta';
import { hostOf, inlineQuizzesByHost, isModuleDone, moduleHref } from '@/lib/module-links';
import { ModuleRow } from '@/components/ModuleList';
import { SessionCard } from '@/components/sessions/SessionCard';

// /courses/:slug (docs/specs/011-course-page-and-enrollment.md), markup of
// pupitre's pages/courses/[slug]/index.astro.

function EnrollArea({ course, enrolled, modules }: { course: Course; enrolled: boolean; modules: ModuleSummary[] }) {
  const session = useSession();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const enrolment = useMutation({
    mutationFn: () => enroll(course.slug),
    onSuccess: () => {
      toast.success('Inscription confirmée');
      void queryClient.invalidateQueries({ queryKey: ['my', 'courses'] });
      void queryClient.invalidateQueries({ queryKey: ['course', course.slug] });
    },
    // The Astro page ignored this error and reloaded (defect A8).
    onError: (error) => toast.error(error.message),
  });

  if (!enrolled) {
    if (!course.isPublic) {
      return <div className="course-restricted">🔒 Accès restreint — contactez un administrateur</div>;
    }
    return (
      <button
        className="btn btn-primary w-full"
        disabled={enrolment.isPending}
        onClick={() => {
          if (!session) navigate(loginPath({ pathname: `/courses/${course.slug}`, search: '' }));
          else enrolment.mutate();
        }}
      >
        S'inscrire au cours
      </button>
    );
  }

  if (modules.length === 0) return null;

  const completed = modules.filter(isModuleDone).length;
  const pct = Math.round((completed / modules.length) * 100);
  const listModules = modules.filter((m) => !m.inline);
  const next = modules.find((m) => !isModuleDone(m) && !m.locked);
  const target = next ? (next.inline ? (hostOf(modules, next) ?? next) : next) : listModules[0];
  const ctaHref = target ? moduleHref(course.slug, target) : '#';
  const xpEarned = Math.round((pct / 100) * COURSE_XP);

  return (
    <>
      <div className="course-enroll-progress">
        <div className="course-enroll-progress-header">
          <span>
            {completed}/{modules.length} complétés
          </span>
          <span className="course-enroll-pct">{pct}%</span>
        </div>
        <div className="progress-bar">
          <div className="progress-bar-fill" style={{ width: `${pct}%` }} />
        </div>
      </div>
      <Link to="/my-xp" className="course-xp-card no-underline cursor-pointer">
        <div className={`course-xp-row ${pct < 100 ? 'mb-[5px]' : ''}`}>
          <span>⚡ XP de ce cours</span>
          <span className="course-xp-amount">{pct === 100 ? `+${COURSE_XP} XP ✓` : `${xpEarned} / ${COURSE_XP} XP`}</span>
        </div>
        {pct < 100 && (
          <div className="course-xp-bar">
            <div className="course-xp-bar-fill" style={{ width: `${pct}%` }} />
          </div>
        )}
      </Link>
      <Link to={ctaHref} className="btn btn-primary w-full inline-flex">
        {pct === 0 ? 'Commencer le cours ▸' : pct === 100 ? 'Revoir le cours ▸' : 'Continuer le cours ▸'}
      </Link>
    </>
  );
}

export function CourseDetail() {
  const { slug = '' } = useParams();
  const signedIn = Boolean(useSession());

  const course = useQuery({ queryKey: ['course', slug], queryFn: () => getCourse(slug) });
  const mine = useQuery({ queryKey: ['my', 'courses'], queryFn: myCourses, enabled: signedIn });
  const badges = useQuery({ queryKey: ['my', 'badges'], queryFn: myBadges, enabled: signedIn });
  const enrolled = Boolean(mine.data?.courses.some((c) => c.slug === slug || c.id === slug));
  const modules = useQuery({
    queryKey: ['course', slug, 'modules'],
    queryFn: () => listModules(slug),
    enabled: signedIn && enrolled,
  });
  const hasSessions = Boolean(course.data?.inPerson && course.data.sessions?.length);
  const bookings = useQuery({
    queryKey: ['my', 'session-bookings'],
    queryFn: mySessionBookings,
    enabled: signedIn && hasSessions,
  });

  if (course.isPending || (signedIn && mine.isPending)) {
    return (
      <div className="course-outer">
        <div className="loading-state">
          <div className="spinner" />
          <span>Chargement…</span>
        </div>
      </div>
    );
  }

  if (course.isError) {
    return (
      <div className="course-outer">
        <div className="not-found-state">
          <div className="not-found-icon">😕</div>
          <h2>Cours introuvable</h2>
          <Link to="/courses" className="btn btn-primary mt-4 inline-flex">
            Retour au catalogue
          </Link>
        </div>
      </div>
    );
  }

  const data = course.data;
  const allModules = modules.data?.modules ?? [];
  const listed = allModules.filter((m) => !m.inline);
  const inlineByHost = inlineQuizzesByHost(allModules);
  const prereqBlocked = modules.error instanceof ApiError && modules.error.status === 403;
  const earned = badges.data?.badges.some((b) => b.courseSlug === slug) ?? false;
  const bookedIds = new Set(
    (bookings.data?.bookings ?? []).filter((b) => b.courseSlug === slug).map((b) => b.sessionId),
  );

  return (
    <div className="course-outer">
      <div className="course-page">
        <aside className="course-sidebar">
          <div className="course-thumb">{courseIcon(data, '📖')}</div>
          <h1 className="course-title-head" data-tour="page-title">{data.title}</h1>
          <div className="course-tags">
            {data.difficulty && (
              <span className={`badge ${DIFFICULTY_BADGES[data.difficulty] ?? 'badge-blue'}`}>
                {DIFFICULTY_LABELS[data.difficulty] ?? data.difficulty}
              </span>
            )}
            {data.category && <span className="badge badge-blue">{data.category}</span>}
            {(data.skills ?? []).map((skill) => (
              <Link key={skill} to={`/skills/${skill}`} className="badge badge-purple text-[.75rem] no-underline">
                🎯 {skill}
              </Link>
            ))}
          </div>
          <p className="text-soft text-sm course-desc-text">{data.description}</p>
          {data.badge && (
            <div>
              {earned ? (
                <Link to="/my-badges" className="course-badge-card course-badge-earned">
                  <span className="text-[1.6rem]">{data.badge.icon || '🏅'}</span>
                  <div>
                    <div className="course-badge-label course-badge-label-earned">✓ Badge obtenu</div>
                    <div className="course-badge-name">{data.badge.name || 'Badge'}</div>
                  </div>
                </Link>
              ) : (
                <div className="course-badge-card course-badge-locked">
                  <span className="text-[1.6rem]">{data.badge.icon || '🏅'}</span>
                  <div>
                    <div className="course-badge-label course-badge-label-locked">Badge à obtenir</div>
                    <div className="course-badge-name">{data.badge.name || 'Badge'}</div>
                  </div>
                </div>
              )}
            </div>
          )}
          <div data-tour="course-action">
            <EnrollArea course={data} enrolled={enrolled} modules={allModules} />
          </div>
        </aside>

        <div className="course-main" data-tour="course-modules">
          <div className="course-main-header">
            <h2 className="text-[1rem] font-bold">Contenu du cours</h2>
            <span className="text-soft text-sm">
              {listed.length > 0 ? `${listed.length} module${listed.length !== 1 ? 's' : ''}` : ''}
            </span>
          </div>

          {prereqBlocked ? (
            <div className="prereq-blocked">
              <div className="text-[2rem] mb-2.5">🔒</div>
              <p className="font-semibold mb-1">Prérequis non satisfaits</p>
              <p className="text-soft text-sm">Terminez les cours prérequis pour accéder à ce contenu.</p>
              {(data.prerequisites ?? []).length > 0 && (
                <p className="text-sm mt-2.5 flex flex-wrap gap-1.5 justify-center">
                  {data.prerequisites!.map((prereq) => (
                    <Link key={prereq.course} to={`/courses/${prereq.course}`} className="badge badge-blue no-underline">
                      {prereq.course}
                    </Link>
                  ))}
                </p>
              )}
            </div>
          ) : enrolled && modules.isPending ? (
            <div className="loading-state">
              <div className="spinner" />
            </div>
          ) : enrolled && listed.length > 0 ? (
            <div>
              {listed.map((module) => (
                <ModuleRow key={module.index} slug={slug} module={module} inlineQuiz={inlineByHost.get(module.index)} />
              ))}
            </div>
          ) : (
            <div className="modules-empty">
              <div className="text-[2.5rem] mb-3">📭</div>
              <p>{enrolled ? 'Aucun contenu disponible pour le moment.' : 'Inscrivez-vous pour accéder aux modules.'}</p>
            </div>
          )}

          {hasSessions && (
            <div className="mt-8">
              <div className="sessions-section-header">
                <span className="text-[1.2rem]">📍</span>
                <h2 className="text-[1rem] font-bold">Sessions en présentiel</h2>
              </div>
              <div>
                {data.sessions!.map((session) => (
                  <SessionCard key={session.id} slug={slug} session={session} booked={bookedIds.has(session.id)} />
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
