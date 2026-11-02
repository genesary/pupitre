import type { CSSProperties } from 'react';
import { Link } from 'react-router';

import type { Course } from '@pupitre/core/lib/api-types';
import {
  categoryIcon,
  categoryLabel,
  courseIcon,
  DIFFICULTY_BADGES,
  DIFFICULTY_LABELS,
  type CategoryColor,
} from '@pupitre/core/lib/course-meta';

// A course card of the catalogue grid (docs/specs/010-catalogue.md).

type CourseCardProps = {
  course: Course;
  color: CategoryColor;
  enrolled: boolean;
  /** Position in the grid: staggers the entrance animation (index.css, .cat-grid). */
  index: number;
  /** The card the guided tour points at (docs/specs/007-guided-tour.md). */
  tour?: boolean;
};

export function CourseCard({ course, color, enrolled, index, tour = false }: CourseCardProps) {
  const difficulty = course.difficulty;
  return (
    <div
      className="rail-card relative"
      data-tour={tour ? 'course-card' : undefined}
      style={{ '--cat-color': color.dot, '--i': index } as CSSProperties}
    >
      {/* Stretched link: the whole card opens the course, skill chips stay links of their own. */}
      <Link to={`/courses/${course.slug}`} className="absolute inset-0 z-0" aria-label={course.title} />
      <div className="rail-card-thumb-wrap">
        {/* --thumb-bg, not an inline background: dark mode tints the thumbnail instead (index.css). */}
        <div className="rail-card-thumb" style={{ '--thumb-bg': color.thumbBg } as CSSProperties}>
          <span className="text-[2rem]">{courseIcon(course)}</span>
        </div>
        <span className="rail-card-cat">
          {categoryIcon(course.category)} {categoryLabel(course.category)}
        </span>
      </div>
      <div className="rail-card-body">
        <div className="rail-card-title">{course.title}</div>
        <div className="rail-card-footer">
          {difficulty && (
            <span className={`badge ${DIFFICULTY_BADGES[difficulty] ?? 'badge-blue'}`}>
              {DIFFICULTY_LABELS[difficulty] ?? difficulty}
            </span>
          )}
          {course.inPerson && <span className="badge badge-inperson text-[.65rem]">🏢 Présentiel</span>}
          {(course.xpRequired ?? 0) > 0 && (
            <span className="badge badge-orange" title={`${course.xpRequired} XP requis`}>
              ⚡ {course.xpRequired} XP requis
            </span>
          )}
          <span className="badge badge-xp-reward text-[.65rem]">+200 XP</span>
          {(course.skills ?? []).slice(0, 3).map((skill) => (
            <Link key={skill} to={`/skills/${skill}`} className="badge badge-purple text-[.65rem] no-underline relative z-10">
              🎯 {skill}
            </Link>
          ))}
        </div>
        {enrolled && (
          <div className="rail-card-prog">
            <div className="rail-card-prog-fill" style={{ width: '100%' }} />
          </div>
        )}
      </div>
    </div>
  );
}
