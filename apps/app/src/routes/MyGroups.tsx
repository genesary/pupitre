import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { listCourses, myGroups } from '@pupitre/core/lib/api';
import type { Course, MyGroup } from '@pupitre/core/lib/api-types';
import { courseIcon } from '@pupitre/core/lib/course-meta';
import { LoadError } from '@pupitre/core/components/PageStates';

// /my-groups (docs/specs/020-learner-space.md), markup of pupitre's my-groups.astro.

const ROLES: Record<string, { className: string; label: string }> = {
  admin: { className: 'mg-role-admin', label: 'Admin' },
  manager: { className: 'mg-role-manager', label: 'Manager' },
  student: { className: 'mg-role-student', label: 'Étudiant' },
};

function GroupCard({ group, courses }: { group: MyGroup; courses: Map<string, Course> }) {
  const isLocal = group.source === 'local';
  const role = group.mappedRole ? (ROLES[group.mappedRole] ?? ROLES.student!) : null;
  const linked = group.courseSlugs.flatMap((slug) => courses.get(slug) ?? []);
  return (
    <div className="mg-card">
      <div className="mg-card-top">
        <div className={`mg-icon ${isLocal ? 'local' : 'remote'}`}>{isLocal ? '🏠' : '🔗'}</div>
        <div>
          <div className="mg-name">{group.name}</div>
          <div className="mg-source">{isLocal ? 'Groupe local' : `Synchronisé depuis ${group.source}`}</div>
        </div>
        {role && <span className={`mg-role-badge ${role.className}`}>{role.label}</span>}
      </div>
      <div className="h-px bg-line mb-3.5" />
      <div className="mg-courses-label">Cours du groupe</div>
      {linked.length > 0 ? (
        linked.map((course) => (
          <Link key={course.slug} to={`/courses/${course.slug}`} className="mg-course-link">
            <span className="mg-course-icon">{courseIcon(course, '📘')}</span>
            <span className="mg-course-title">{course.title || course.slug}</span>
          </Link>
        ))
      ) : (
        <p className="text-[.8rem] text-ink-soft italic">Aucun cours associé à ce groupe.</p>
      )}
    </div>
  );
}

export function MyGroups() {
  const groups = useQuery({ queryKey: ['my', 'groups'], queryFn: myGroups });
  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });

  const list = groups.data?.groups ?? [];
  const bySlug = new Map((courses.data?.courses ?? []).map((c) => [c.slug, c]));

  return (
    <div className="page-outer">
      <div className="page-inner">
        <div className="mb-9">
          <h1 data-tour="page-title" className="page-title mb-1">Mes groupes</h1>
          <p className="text-[.9rem] text-ink-soft">
            {!groups.isSuccess
              ? 'Chargement…'
              : list.length
                ? `${list.length} groupe${list.length > 1 ? 's' : ''}`
                : "Vous n'appartenez à aucun groupe."}
          </p>
        </div>

        {groups.isPending ? (
          <div className="loading-state py-10">
            <div className="spinner" />
          </div>
        ) : groups.isError ? (
          <LoadError error={groups.error} onRetry={() => groups.refetch()} />
        ) : list.length === 0 ? (
          <div className="mg-empty-card">
            <div className="text-[3rem] mb-3">👥</div>
            <p>Vous n'appartenez à aucun groupe pour le moment.</p>
            <p className="text-[.8rem] text-ink-soft mt-1.5">Un administrateur peut vous ajouter à un groupe.</p>
          </div>
        ) : (
          <div className="auto-grid-lg">
            {list.map((group) => (
              <GroupCard key={group.id} group={group} courses={bySlug} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
