import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, Navigate, useNavigate, useParams } from 'react-router';
import { toast } from 'sonner';

import { ApiError } from '@pupitre/core/lib/api-client';
import { completeLesson, getCourse, getModule, listModules, listPatterns } from '@pupitre/core/lib/api';
import type { ModuleDetail, ModuleSummary } from '@pupitre/core/lib/api-types';
import { MarkdownContent } from '@pupitre/core/components/MarkdownContent';
import { hostOf, isModuleDone, moduleHref } from '@/lib/module-links';
import { InlineQuiz } from '@/components/quiz/InlineQuiz';
import { patternCss, renderMarkdown, toPatternMap, type TocEntry } from '@pupitre/core/lib/markdown';
import { safeUrl } from '@pupitre/ui/lib/security';
import { readItem, writeItem } from '@pupitre/ui/lib/storage';
import { takeCourseReturn } from '@/lib/course-return';
import { usePatternStyles } from '@pupitre/core/lib/use-pattern-styles';

import { LabContent } from '@/components/lab/LabContent';
import { LessonTip } from '@/components/tour/LessonTip';

// /courses/:slug/lessons/:index (docs/specs/012-lesson-player.md), markup of
// pupitre's pages/courses/[slug]/lessons/[index].astro.

const MODE_KEY = 'pupitre.readingMode';

const ICON_FOCUS = (
  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="5" y="3" width="14" height="18" rx="2" />
    <line x1="9" y1="8" x2="15" y2="8" />
    <line x1="9" y1="12" x2="15" y2="12" />
    <line x1="9" y1="16" x2="13" y2="16" />
  </svg>
);
const ICON_DOCS = (
  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="3" width="6" height="18" rx="1" />
    <rect x="11" y="3" width="10" height="18" rx="1" />
  </svg>
);

function treeIcon(module: ModuleSummary): string {
  if (module.locked) return '🔒';
  if (module.type === 'quiz') return '🧩';
  if (module.type === 'video') return '🎬';
  if (module.type === 'image') return '🖼️';
  if (module.type === 'lab') return '🧪';
  return '📝';
}

function useReadingMode() {
  const [focus, setFocus] = useState(() => readItem('local', MODE_KEY) === 'focus');
  useEffect(() => {
    document.body.classList.toggle('focus-mode', focus);
    return () => document.body.classList.remove('focus-mode');
  }, [focus]);
  const toggle = () =>
    setFocus((current) => {
      writeItem('local', MODE_KEY, current ? 'docs' : 'focus');
      return !current;
    });
  return { focus, toggle };
}

type LessonProps = {
  slug: string;
  courseTitle: string;
  module: ModuleDetail;
  modules: ModuleSummary[];
};

function Lesson({ slug, courseTitle, module, modules }: LessonProps) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { focus, toggle } = useReadingMode();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [labToc, setLabToc] = useState<TocEntry[]>([]);
  const [quizPassed, setQuizPassed] = useState(false);

  const patternsQuery = useQuery({ queryKey: ['patterns'], queryFn: listPatterns, staleTime: Infinity });
  const patterns = useMemo(() => toPatternMap(patternsQuery.data?.patterns), [patternsQuery.data]);
  usePatternStyles(useMemo(() => patternCss(patterns), [patterns]));

  const rendered = useMemo(
    () => (module.type === 'text' && module.content ? renderMarkdown(module.content, { patterns, dropFirstHeading: true }) : null),
    [module.type, module.content, patterns],
  );
  const toc = module.type === 'lab' ? labToc : (rendered?.toc ?? []);

  const navModules = modules.filter((m) => !m.inline);
  const position = modules.findIndex((m) => m.index === module.index);
  const next = modules.slice(position + 1).find((m) => !m.inline) ?? null;
  const previous = modules.slice(0, position).filter((m) => !m.inline).at(-1) ?? null;
  // The module list is refetched once the inline quiz is passed, so the count
  // stays the server's; quizPassed only unblocks the navigation meanwhile.
  const completed = modules.filter(isModuleDone).length;
  const total = modules.length;
  const pct = total > 0 ? Math.round((completed / total) * 100) : 0;
  const quizPending = (module.inlineQuiz?.questions?.length ?? 0) > 0 && !module.inlineQuiz?.passed && !quizPassed;
  const isLast = !next;

  const finish = useCallback(() => {
    if (next) {
      navigate(moduleHref(slug, next));
      return;
    }
    navigate(takeCourseReturn() ?? `/courses/${slug}`);
  }, [navigate, next, slug]);

  const complete = useMutation({
    mutationFn: () => completeLesson(slug, module.slug),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['course', slug, 'modules'] });
      void queryClient.invalidateQueries({ queryKey: ['my'] });
      finish();
    },
    onError: () => toast.error('Erreur lors de la mise à jour'),
  });

  const labCompleted = useCallback(() => {
    completeLesson(slug, module.slug)
      .catch(() => {}) // already completed
      .finally(() => {
        void queryClient.invalidateQueries({ queryKey: ['course', slug, 'modules'] });
        finish();
      });
  }, [finish, module.slug, queryClient, slug]);

  const nextLabel = next?.type === 'lab' ? 'Passer au Lab ▸' : 'Suivant ▸';
  const buttonLabel = next?.type === 'lab' ? 'Passer au Lab ▸' : next ? 'Suivant ▸' : quizPending ? 'Terminer la leçon ▸' : 'Terminer le cours ▸';
  const mediaUrl = safeUrl(module.content);

  return (
    <>
      <div className="lesson-layout">
        <aside className={`lesson-left${drawerOpen ? ' nav-open' : ''}`} data-tour="lesson-modules">
          <Link to={`/courses/${slug}`} className="lesson-back-link">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M19 12H5M12 19l-7-7 7-7" />
            </svg>
            Retour
          </Link>
          <div className="left-inner">
            <div className="lesson-sidebar-inner">
              <div className="lesson-sidebar-label">Modules</div>
              <div className="progress-bar mb-1">
                <div className="progress-bar-fill" style={{ width: `${pct}%` }} />
              </div>
              <div className="lesson-progress-text">
                {completed}/{total} complétés
              </div>
            </div>
            <nav className="sidebar-nav px-2" onClick={() => setDrawerOpen(false)}>
              {navModules.map((m) =>
                m.locked ? (
                  <span key={m.index} className="sidebar-link locked">
                    <span className="sidebar-link-icon">{treeIcon(m)}</span>
                    <span className="sidebar-link-text">{m.name}</span>
                  </span>
                ) : (
                  <Link key={m.index} to={moduleHref(slug, m)} className={`sidebar-link${m.index === module.index ? ' active' : ''}`}>
                    <span className="sidebar-link-icon">{treeIcon(m)}</span>
                    <span className="sidebar-link-text">{m.name}</span>
                    {(m.viewed || m.passed) && <span className="sidebar-done-check">✓</span>}
                  </Link>
                ),
              )}
            </nav>
          </div>
        </aside>

        <main className="lesson-center">
          <button className="lesson-mobile-open" data-tour="lesson-modules-mobile" onClick={() => setDrawerOpen(true)}>
            ☰ Modules
          </button>
          <div className="lesson-breadcrumb">
            <Link to="/courses">Catalogue</Link>
            <span>›</span>
            <Link to={`/courses/${slug}`}>{courseTitle}</Link>
            <span>›</span>
            <span>{module.name}</span>
          </div>
          <h1 className="lesson-title" data-tour="page-title">{module.name}</h1>

          {module.type === 'lab' ? (
            <div className="prose">
              <LabContent slug={slug} module={module} patterns={patterns} onTocChange={setLabToc} onCompleted={labCompleted} />
            </div>
          ) : module.type === 'video' && mediaUrl ? (
            <div className="prose">
              <div className="media-wrap">
                <video controls className="max-w-full rounded-[10px]">
                  <source src={mediaUrl} type="video/mp4" />
                </video>
              </div>
            </div>
          ) : module.type === 'image' && mediaUrl ? (
            <div className="prose">
              <div className="media-wrap">
                <img src={mediaUrl} alt={module.name} className="max-w-full rounded-[10px]" />
              </div>
            </div>
          ) : rendered ? (
            <MarkdownContent html={rendered.html} />
          ) : (
            <div className="prose">
              <p className="text-ink-soft">Aucun contenu disponible.</p>
            </div>
          )}

          {module.inlineQuiz && (module.inlineQuiz.questions?.length ?? 0) > 0 && (
            <InlineQuiz
              slug={slug}
              quiz={module.inlineQuiz}
              onPassed={() => {
                setQuizPassed(true);
                void queryClient.invalidateQueries({ queryKey: ['course', slug, 'modules'] });
              }}
            />
          )}

          <div className="lesson-nav" data-tour="lesson-nav">
            {previous ? (
              <Link to={moduleHref(slug, previous)} className="btn btn-secondary">
                ◂ Précédent
              </Link>
            ) : (
              <Link to={`/courses/${slug}`} className="btn btn-secondary">
                ◂ Cours
              </Link>
            )}
            {isLast && quizPending && <span className="lesson-nav-hint">Validez le quiz ci-dessous pour terminer le cours.</span>}
            {!module.viewed || isLast ? (
              // A button, not a link: the last module must always call complete,
              // which records the end of the course on the server.
              <button className="btn btn-primary" disabled={complete.isPending} onClick={() => complete.mutate()}>
                {complete.isSuccess ? '✓ Terminé' : buttonLabel}
              </button>
            ) : (
              <Link to={moduleHref(slug, next!)} className="btn btn-primary">
                {nextLabel}
              </Link>
            )}
          </div>
        </main>

        <aside className="lesson-right">
          <div className="toc">
            <div className="toc-title">Sur cette page</div>
            <nav>
              {toc.map((entry) => (
                <a key={entry.id} href={`#${entry.id}`} className={entry.level === 3 ? 'toc-h3' : ''}>
                  {entry.text}
                </a>
              ))}
            </nav>
          </div>
          <div className="divider my-4" />
        </aside>
      </div>

      <div className={`nav-overlay${drawerOpen ? ' active' : ''}`} onClick={() => setDrawerOpen(false)} />
      <LessonTip />

      <div className="floating-progress">
        <div className="floating-prog-bar">
          <div className="floating-prog-fill" style={{ width: `${pct}%` }} />
        </div>
        <span className="text-[.75rem] text-ink-soft">
          {completed}/{total}
        </span>
      </div>

      <button
        className="floating-mode-btn"
        title={focus ? 'Revenir au mode 3 colonnes' : 'Passer en mode lecture concentrée'}
        onClick={toggle}
      >
        <span className="lesson-fmb-icon">{focus ? ICON_DOCS : ICON_FOCUS}</span>
        <span>{focus ? 'Mode docs' : 'Lecture focus'}</span>
      </button>
    </>
  );
}

export function LessonPage() {
  const { slug = '', index: rawIndex = '' } = useParams();
  const index = Number(rawIndex);
  const valid = Number.isInteger(index) && index >= 0;

  const course = useQuery({ queryKey: ['course', slug], queryFn: () => getCourse(slug) });
  const modules = useQuery({ queryKey: ['course', slug, 'modules'], queryFn: () => listModules(slug), enabled: valid });
  const module = useQuery({ queryKey: ['course', slug, 'module', index], queryFn: () => getModule(slug, index), enabled: valid });

  if (!valid) return <Navigate to={`/courses/${slug}`} replace />;

  const forbidden = [modules.error, module.error].some((e) => e instanceof ApiError && e.status === 403);
  const current = modules.data?.modules.find((m) => m.index === index);
  if (forbidden || current?.locked) return <Navigate to={`/courses/${slug}`} replace />;

  if (module.data?.type === 'quiz') {
    if (module.data.inline && modules.data) {
      const host = hostOf(modules.data.modules, module.data);
      return <Navigate to={host ? `/courses/${slug}/lessons/${host.index}` : `/courses/${slug}`} replace />;
    }
    if (!module.data.inline) return <Navigate to={`/courses/${slug}/quiz/${index}`} replace />;
  }

  if (module.isError || modules.isError) {
    return (
      <div className="text-center py-16">
        <h2>Module introuvable</h2>
        <Link to="/courses" className="btn btn-primary mt-4 inline-flex">
          Catalogue
        </Link>
      </div>
    );
  }

  if (!module.data || !modules.data) {
    return (
      <div className="loading-state">
        <div className="spinner" />
        <span>Chargement…</span>
      </div>
    );
  }

  return (
    <Lesson
      key={`${slug}-${index}`}
      slug={slug}
      courseTitle={course.data?.title ?? slug}
      module={module.data}
      modules={modules.data.modules}
    />
  );
}
