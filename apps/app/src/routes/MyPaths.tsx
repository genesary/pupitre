import { useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import { Link, useSearchParams } from 'react-router';

import { myPaths } from '@pupitre/core/lib/api';
import type { MyPath } from '@pupitre/core/lib/api-types';
import { LoadError, LoadingState } from '@pupitre/core/components/PageStates';
import { PathMap } from '@/components/paths/PathMap';

// /my-paths (docs/specs/020-learner-space.md), markup of pupitre's my-paths.astro:
// the list of paths, then the detail of the one chosen. The chosen path is in
// the URL (?path=), so the end of a course and the back button return to it.

const PATH_ICONS: Record<string, string> = {
  'devops-path': '⚙️',
  'security-path': '🔐',
  devops: '⚙️',
  security: '🔐',
};

function progressOf(path: MyPath) {
  const steps = path.courses ?? [];
  const done = steps.filter((s) => s.status === 'completed').length;
  const total = steps.length;
  return { done, total, pct: total > 0 ? Math.round((done / total) * 100) : 0 };
}

function PathListCard({ path, index, onOpen }: { path: MyPath; index: number; onOpen: () => void }) {
  const { done, total, pct } = progressOf(path);
  const allDone = pct === 100;
  const sub =
    path.kind === 'skill'
      ? `${total} compétence${total > 1 ? 's' : ''} · ${allDone ? '✓ Acquises' : done > 0 ? `${done}/${total} acquises` : 'À commencer'}`
      : `${done}/${total} cours · ${allDone ? '✓ Terminé' : pct > 0 ? 'En cours' : 'À commencer'}`;
  return (
    <div
      className={`path-list-card path-palette-${index % 5}`}
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onOpen();
        }
      }}
    >
      <div className="path-list-banner">
        <div className="path-list-icon">{PATH_ICONS[path.slug] ?? '🗺️'}</div>
        <div className="path-list-body">
          <div className="path-list-title">{path.title || path.slug}</div>
          <div className="path-list-sub">{sub}</div>
        </div>
        <span className="path-list-pct">{pct}%</span>
        <span className="path-list-chevron">›</span>
      </div>
      <div className="path-list-progress">
        <div className="path-list-progress-fill" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function PathDetail({ path }: { path: MyPath }) {
  const isSkillPath = path.kind === 'skill';
  const { done, total, pct } = progressOf(path);
  const allDone = total > 0 && done === total;
  const colour = allDone ? 'bg-good text-good' : 'text-blue';
  return (
    <>
      <div>
        <div className="detail-title">{path.title || path.slug}</div>
        {path.description && <div className="detail-desc">{path.description}</div>}
        {!isSkillPath && (path.skills?.length ?? 0) > 0 && (
          <div className="flex flex-wrap gap-[.3rem] my-[.4rem]">
            {path.skills!.map((skill) => (
              <Link key={skill} to={`/skills/${skill}`} className="badge badge-purple text-[.65rem] no-underline">
                🎯 {skill}
              </Link>
            ))}
          </div>
        )}
        <div className="detail-prog">
          <div className="progress-bar flex-1">
            <div className={`progress-bar-fill ${colour}`} style={{ width: `${pct}%` }} />
          </div>
          <span className={`detail-prog-pct ${colour}`}>{pct}%</span>
          <span>
            {done}/{total} {isSkillPath ? `compétence${total > 1 ? 's' : ''}` : 'cours'}
          </span>
        </div>
      </div>
      <div className="snake-wrap">
        <PathMap path={path} />
      </div>
      {allDone && <div className="text-center py-2.5 text-base font-bold text-good">🏆 Parcours terminé !</div>}
    </>
  );
}

export function MyPaths() {
  const paths = useQuery({ queryKey: ['my', 'paths'], queryFn: myPaths });
  const [params, setParams] = useSearchParams();
  const list = paths.data?.paths ?? [];
  const selected = list.find((p) => p.slug === params.get('path'));

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }, [selected]);

  return (
    <div className="page-outer">
      <div className="page-inner">
        <h1 data-tour="page-title" className="page-title mb-1.5">Mes parcours</h1>
        <p className="text-[.9rem] text-ink-soft mb-7">{paths.isSuccess ? `${list.length} parcours en cours` : ''}</p>

        {paths.isPending ? (
          <LoadingState />
        ) : paths.isError ? (
          <LoadError error={paths.error} onRetry={() => paths.refetch()} />
        ) : selected ? (
          <div>
            <button className="detail-back" onClick={() => setParams({})}>
              ◂ Mes parcours
            </button>
            <PathDetail path={selected} />
          </div>
        ) : list.length === 0 ? (
          <div className="text-center py-20 px-6">
            <div className="text-[3rem] mb-3.5">🗺️</div>
            <h2 className="text-[1.15rem] mb-2">Aucun parcours en cours</h2>
            <p className="text-ink-soft text-[.88rem] mb-5">
              Vous n'êtes inscrit à aucun parcours. Contactez un administrateur pour être inscrit.
            </p>
            <Link to="/courses" className="btn btn-primary">
              Voir le catalogue
            </Link>
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            {list.map((path, index) => (
              <PathListCard key={path.slug} path={path} index={index} onOpen={() => setParams({ path: path.slug })} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
