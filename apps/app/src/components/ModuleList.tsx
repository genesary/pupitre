import { Link, useNavigate } from 'react-router';

import type { ModuleSummary } from '@pupitre/core/lib/api-types';
import { moduleHref, moduleIcon } from '@/lib/module-links';

// Module rows of the course page (docs/specs/011-course-page-and-enrollment.md).

const TYPE_LABELS: Record<string, string> = { quiz: 'Quiz', video: 'Vidéo', image: 'Image', lab: 'Lab', lesson: 'Lecture' };

function ModuleBadges({ module, inlineQuiz }: { module: ModuleSummary; inlineQuiz?: ModuleSummary }) {
  const typeLabel = TYPE_LABELS[module.type];
  return (
    <div className="mod-badges">
      {typeLabel && <span className="badge badge-gray text-[.7rem]">{typeLabel}</span>}
      {module.locked ? (
        <span className="badge badge-gray text-[.7rem]">Verrouillé</span>
      ) : module.type === 'quiz' ? (
        module.passed ? (
          <span className="badge badge-green text-[.7rem]">
            ✓ {module.bestScore}/{module.maxScore} pts
          </span>
        ) : (module.attempts ?? 0) > 0 ? (
          <span className="badge badge-orange text-[.7rem]">
            {module.attempts} tentative{(module.attempts ?? 0) > 1 ? 's' : ''}
          </span>
        ) : (
          <span className="badge badge-orange text-[.7rem]">★ quiz</span>
        )
      ) : (
        module.viewed && <span className="badge badge-green text-[.7rem]">✓ Vu</span>
      )}
      {inlineQuiz && !module.locked &&
        (inlineQuiz.passed ? (
          <span className="badge badge-green text-[.7rem]">✓ Quiz</span>
        ) : (
          <span className="badge badge-orange text-[.7rem]">🧩 Quiz à valider</span>
        ))}
    </div>
  );
}

export function ModuleRow({ slug, module, inlineQuiz }: { slug: string; module: ModuleSummary; inlineQuiz?: ModuleSummary }) {
  const navigate = useNavigate();
  const icon = moduleIcon(module);
  const iconClass = module.locked
    ? ''
    : module.type === 'quiz'
      ? ' quiz'
      : module.viewed || module.passed
        ? ' done'
        : module.type === 'lab'
          ? ' lab'
          : '';
  const body = (
    <>
      {icon && <div className={`mod-icon${iconClass}`}>{icon}</div>}
      <div className="mod-name flex-1">
        {module.name}
        {(module.skills ?? []).length > 0 && (
          <span className="mod-skill-badges">
            {module.skills!.map((skill) => (
              <span
                key={skill}
                role="link"
                tabIndex={0}
                className="badge badge-purple text-[.65rem] cursor-pointer"
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  navigate(`/skills/${skill}`);
                }}
                onKeyDown={(event) => event.key === 'Enter' && navigate(`/skills/${skill}`)}
              >
                🎯 {skill}
              </span>
            ))}
          </span>
        )}
      </div>
      <ModuleBadges module={module} inlineQuiz={inlineQuiz} />
      {!module.locked && (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--ink-soft)" strokeWidth="2">
          <path d="M9 5l7 7-7 7" />
        </svg>
      )}
    </>
  );
  return module.locked ? (
    <div className="mod-row locked">{body}</div>
  ) : (
    <Link to={moduleHref(slug, module)} className="mod-row">
      {body}
    </Link>
  );
}
