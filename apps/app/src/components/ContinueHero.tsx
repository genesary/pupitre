import { useNavigate } from 'react-router';

import type { MyCourse } from '@pupitre/core/lib/api-types';
import { courseIcon } from '@pupitre/core/lib/course-meta';

// The « Reprendre » block of the catalogue (docs/specs/010-catalogue.md).

export function ContinueHero({ course }: { course: MyCourse }) {
  const navigate = useNavigate();
  const done = course.completedLabs ?? 0;
  const total = course.labCount ?? 0;
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  const open = () => navigate(`/courses/${course.slug}`);
  return (
    <div className="continue-hero" data-tour="continue-hero" role="link" tabIndex={0} onClick={open} onKeyDown={(e) => e.key === 'Enter' && open()}>
      <div className="continue-thumb">{courseIcon({ category: course.category })}</div>
      <div className="continue-meta">
        <div className="continue-badge">Reprendre</div>
        <div className="continue-title">{course.title}</div>
        <div className="continue-prog">
          <div className="continue-prog-fill" style={{ width: `${pct}%` }} />
        </div>
        <div className="continue-sub">
          {done}/{total} modules · {pct}%
        </div>
      </div>
      <button
        className="continue-btn"
        onClick={(event) => {
          event.stopPropagation();
          open();
        }}
      >
        Continuer ▸
      </button>
    </div>
  );
}
