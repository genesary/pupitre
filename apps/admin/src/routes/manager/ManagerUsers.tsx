import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import {
  listCourses,
  listPaths,
  managerEnroll,
  managerEnrollPath,
  managerUnenroll,
  managerUnenrollPath,
  managerUserEnrollments,
  managerUserPathEnrollments,
  managerUsers,
} from '@pupitre/core/lib/api';
import type { ScopeUser } from '@pupitre/core/lib/api-types';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { Modal, ModalClose } from '@pupitre/core/components/Modal';

// /manager/users (docs/specs/030-manager-space.md), markup of pupitre's
// manager/users/index.astro. The group filter of the Astro page never worked
// (defect A21); the search by name or e-mail is new.

type Removal = { kind: 'course' | 'path'; slug: string; label: string };

function EnrollmentsModal({ member, onClose }: { member: ScopeUser; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [course, setCourse] = useState('');
  const [path, setPath] = useState('');
  const [removal, setRemoval] = useState<Removal | null>(null);

  const courses = useQuery({ queryKey: ['courses'], queryFn: listCourses });
  const paths = useQuery({ queryKey: ['paths'], queryFn: listPaths });
  const enrollments = useQuery({
    queryKey: ['manager', 'users', member.id, 'enrollments'],
    queryFn: () => managerUserEnrollments(member.id),
  });
  const pathEnrollments = useQuery({
    queryKey: ['manager', 'users', member.id, 'path-enrollments'],
    queryFn: () => managerUserPathEnrollments(member.id),
  });

  const courseList = courses.data?.courses ?? [];
  const pathList = paths.data?.paths ?? [];
  const enrolledCourses = enrollments.data?.enrollments ?? [];
  const enrolledPaths = pathEnrollments.data?.pathEnrollments ?? [];
  const courseTitle = (slug: string) => courseList.find((c) => c.slug === slug)?.title ?? slug;
  const pathTitle = (slug: string) => pathList.find((p) => p.slug === slug)?.title ?? slug;

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['manager', 'users'] });
  };

  const enrollCourse = useMutation({
    mutationFn: () => managerEnroll(course, member.id),
    onSuccess: () => {
      setCourse('');
      toast.success('Inscrit avec succès');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const enrollPath = useMutation({
    mutationFn: () => managerEnrollPath(path, member.id),
    onSuccess: () => {
      setPath('');
      toast.success('Inscrit au parcours');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const remove = useMutation({
    mutationFn: (item: Removal) =>
      item.kind === 'course' ? managerUnenroll(item.slug, member.id) : managerUnenrollPath(item.slug, member.id),
    onSuccess: (_, item) => {
      toast.success(item.kind === 'course' ? 'Inscription retirée' : 'Parcours retiré');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const spinner = (
    <div className="loading-state !py-3 !px-0">
      <div className="spinner w-5 h-5" />
    </div>
  );

  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title={`${member.username} — Inscriptions`}
      overlayClassName="mu-modal-overlay"
      contentClassName="card mu-modal-box"
      header={(title) => (
        <div className="mu-modal-header">
          <h3>{title}</h3>
          <ModalClose className="mu-modal-close" />
        </div>
      )}
    >
      <div className="mu-section">
        <h4 className="mu-section-title">Cours actuels</h4>
        {enrollments.isPending ? (
          spinner
        ) : enrollments.isError ? (
          <p className="text-bad text-[.875rem]">{enrollments.error.message}</p>
        ) : enrolledCourses.length === 0 ? (
          <p className="text-soft text-sm">Aucun cours inscrit.</p>
        ) : (
          enrolledCourses.map((e) => (
            <div key={e.courseSlug} className="mu-enroll-item">
              <div className="mu-enroll-name">
                <span className="font-medium">{courseTitle(e.courseSlug)}</span>
              </div>
              <button
                className="btn btn-sm btn-remove"
                onClick={() => setRemoval({ kind: 'course', slug: e.courseSlug, label: courseTitle(e.courseSlug) })}
              >
                Retirer
              </button>
            </div>
          ))
        )}
      </div>

      <div className="mu-section-divider">
        <h4 className="mu-section-title">Parcours actuels</h4>
        {pathEnrollments.isPending ? (
          spinner
        ) : pathEnrollments.isError ? (
          <p className="text-bad text-[.875rem]">{pathEnrollments.error.message}</p>
        ) : enrolledPaths.length === 0 ? (
          <p className="text-soft text-sm">Aucun parcours inscrit.</p>
        ) : (
          enrolledPaths.map((e) => (
            <div key={e.Slug} className="mu-enroll-item">
              <div className="mu-enroll-name">
                <span className="font-medium">{pathTitle(e.Slug)}</span>
              </div>
              <button
                className="btn btn-sm btn-remove"
                onClick={() => setRemoval({ kind: 'path', slug: e.Slug, label: pathTitle(e.Slug) })}
              >
                Retirer
              </button>
            </div>
          ))
        )}
      </div>

      <div className="mu-section-divider mb-4">
        <h4 className="mu-section-title">Inscrire à un cours</h4>
        <div className="mu-select-row">
          <select
            className="input mu-select-input"
            aria-label="Cours"
            value={course}
            onChange={(event) => setCourse(event.target.value)}
          >
            <option value="">Choisir un cours…</option>
            {courseList
              .filter((c) => !enrolledCourses.some((e) => e.courseSlug === c.slug))
              .map((c) => (
                <option key={c.slug} value={c.slug}>
                  {c.title}
                  {c.scope ? ` [${c.scope}]` : ''}
                </option>
              ))}
          </select>
          <button
            className="btn btn-primary"
            disabled={enrollCourse.isPending}
            onClick={() => (course ? enrollCourse.mutate() : toast.error('Choisissez un cours'))}
          >
            {enrollCourse.isPending ? '…' : 'Inscrire'}
          </button>
        </div>
      </div>

      <div className="mu-section-divider">
        <h4 className="mu-section-title">Inscrire à un parcours</h4>
        <div className="mu-select-row">
          <select
            className="input mu-select-input"
            aria-label="Parcours"
            value={path}
            onChange={(event) => setPath(event.target.value)}
          >
            <option value="">Choisir un parcours…</option>
            {pathList
              .filter((p) => !enrolledPaths.some((e) => e.Slug === p.slug))
              .map((p) => (
                <option key={p.slug} value={p.slug}>
                  {p.title}
                </option>
              ))}
          </select>
          <button
            className="btn btn-primary"
            disabled={enrollPath.isPending}
            onClick={() => (path ? enrollPath.mutate() : toast.error('Choisissez un parcours'))}
          >
            {enrollPath.isPending ? '…' : 'Inscrire'}
          </button>
        </div>
      </div>

      <ConfirmDialog
        open={removal !== null}
        onOpenChange={(open) => !open && setRemoval(null)}
        title={removal?.kind === 'path' ? 'Retirer ce parcours ?' : 'Retirer cette inscription ?'}
        description={removal ? `${member.username} sera désinscrit de « ${removal.label} ».` : undefined}
        confirmLabel="Retirer"
        onConfirm={() => removal && remove.mutate(removal)}
      />
    </Modal>
  );
}

export function ManagerUsers() {
  const users = useQuery({ queryKey: ['manager', 'users'], queryFn: managerUsers });
  const [group, setGroup] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<string | null>(null);

  const members = users.data?.users ?? [];
  const groups = users.data?.groups ?? [];
  const groupMembers = group ? new Set(groups.find((g) => g.id === group)?.memberIds ?? []) : null;
  const inGroup = members.filter((u) => !groupMembers || groupMembers.has(u.id));
  const q = search.toLowerCase().trim();
  const visible = inGroup.filter(
    (u) => !q || u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q),
  );
  const where = group ? ' dans ce groupe' : ' dans vos groupes';
  const member = members.find((u) => u.id === selected);

  return (
    <>
      <div className="flex items-baseline justify-between gap-4 mb-7 flex-wrap">
        <div>
          <h1 className="mu-title">Mes membres</h1>
          <p className="mu-subtitle">
            {users.isSuccess ? `${inGroup.length} membre${inGroup.length !== 1 ? 's' : ''}${where}` : 'Chargement…'}
          </p>
          {users.isSuccess && (
            <div className="flex flex-wrap gap-1.5 mt-2 items-center">
              {groups.length === 0 ? (
                <span className="text-soft text-sm italic">Aucun groupe assigné</span>
              ) : (
                <>
                  <span className="text-soft text-[.75rem] mr-0.5">Filtrer :</span>
                  {groups.map((g) => (
                    <button
                      key={g.id}
                      className={`badge badge-orange mu-group-filter-btn${group === g.id ? ' active' : ''}`}
                      aria-pressed={group === g.id}
                      onClick={() => setGroup((current) => (current === g.id ? null : g.id))}
                    >
                      {g.name}
                    </button>
                  ))}
                </>
              )}
            </div>
          )}
        </div>
        {members.length > 0 && (
          <input
            className="input w-auto min-w-[220px]"
            type="search"
            placeholder="Rechercher un membre…"
            aria-label="Rechercher un membre"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        )}
      </div>

      <div className="mu-table-wrap">
        <table className="mu-table">
          <thead>
            <tr>
              <th>Membre</th>
              <th>Cours inscrits</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.isPending ? (
              <tr>
                <td colSpan={3} className="mu-empty-td">
                  <div className="loading-state !p-0">
                    <div className="spinner" />
                    <span className="text-soft text-sm">Chargement…</span>
                  </div>
                </td>
              </tr>
            ) : users.isError ? (
              <tr>
                <td colSpan={3} className="mu-error-td">
                  Erreur : {users.error.message}
                </td>
              </tr>
            ) : visible.length === 0 ? (
              <tr>
                <td colSpan={3} className="mu-empty-td text-ink-soft">
                  {q ? 'Aucun membre ne correspond à la recherche' : `Aucun membre${where}`}
                </td>
              </tr>
            ) : (
              visible.map((u) => (
                <tr key={u.id}>
                  <td className="mu-td-member">
                    <div className="mu-member-info">
                      <div className="mu-avatar">{(u.username.trim()[0] ?? '?').toUpperCase()}</div>
                      <div>
                        <div className="mu-member-name">{u.username}</div>
                        <div className="mu-member-email">{u.email}</div>
                      </div>
                    </div>
                  </td>
                  <td className="mu-td-count">{u.enrolledCourses ?? 0} cours</td>
                  <td className="mu-td-action">
                    <button className="btn btn-secondary btn-sm" onClick={() => setSelected(u.id)}>
                      Gérer les inscriptions
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {member && <EnrollmentsModal member={member} onClose={() => setSelected(null)} />}
    </>
  );
}
