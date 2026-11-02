import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import { listPaths, managerEnrollPath, managerPathEnrollments, managerUnenrollPath, managerUsers } from '@pupitre/core/lib/api';
import type { PathEnrolledUser } from '@pupitre/core/lib/api-types';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';

// /manager/paths (docs/specs/030-manager-space.md), markup of pupitre's manager/paths/index.astro.

const STATUS_CLASSES: Record<string, string> = {
  completed: 'course-tag-completed',
  available: 'course-tag-available',
  locked: 'course-tag-locked',
};
const STATUS_ICONS: Record<string, string> = { completed: '✓', available: '▶', locked: '🔒' };

function EnrollmentRow({ user, onUnenroll }: { user: PathEnrolledUser; onUnenroll: () => void }) {
  const pct = user.totalCourses > 0 ? Math.round((user.completedCourses / user.totalCourses) * 100) : 0;
  return (
    <tr className="admin-tr">
      <td className="admin-td">
        <div className="flex-row-gap3">
          <div className="user-avatar-sm">{(user.email || '?')[0]!.toUpperCase()}</div>
          <div>
            <div className="username-text">{user.email.split('@')[0]}</div>
            <div className="row-sub">{user.email}</div>
          </div>
        </div>
      </td>
      <td className="admin-td">
        <div className="flex flex-wrap gap-1">
          {(user.courses ?? []).map((course) => (
            <span key={course.slug} className={`course-tag ${STATUS_CLASSES[course.status] ?? ''}`}>
              {STATUS_ICONS[course.status]} {course.slug}
            </span>
          ))}
        </div>
        {user.totalCourses > 0 && (
          <div className="path-progress-row">
            <div className="progress-track">
              <div className={`progress-fill ${pct === 100 ? 'complete' : 'partial'}`} style={{ width: `${pct}%` }} />
            </div>
            <span className="path-progress-pct">
              {user.completedCourses}/{user.totalCourses}
            </span>
          </div>
        )}
      </td>
      <td className="admin-td text-soft text-sm whitespace-nowrap">
        {new Date(user.enrolledAt).toLocaleDateString('fr-FR')}
      </td>
      <td className="admin-td-right">
        <button className="btn btn-sm btn-danger" onClick={onUnenroll}>
          Désinscrire
        </button>
      </td>
    </tr>
  );
}

export function ManagerPaths() {
  const queryClient = useQueryClient();
  const [slug, setSlug] = useState('');
  const [userId, setUserId] = useState('');
  const [toRemove, setToRemove] = useState<PathEnrolledUser | null>(null);

  const paths = useQuery({ queryKey: ['paths'], queryFn: listPaths });
  const members = useQuery({ queryKey: ['manager', 'users'], queryFn: managerUsers });
  const enrollments = useQuery({
    queryKey: ['manager', 'paths', slug, 'enrollments'],
    queryFn: () => managerPathEnrollments(slug),
    enabled: Boolean(slug),
  });

  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['manager'] });
  const enroll = useMutation({
    mutationFn: () => managerEnrollPath(slug, userId),
    onSuccess: () => {
      setUserId('');
      toast.success('Membre inscrit au parcours');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const unenroll = useMutation({
    mutationFn: (id: string) => managerUnenrollPath(slug, id),
    onSuccess: () => {
      toast.success('Membre désinscrit');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const pathList = paths.data?.paths ?? [];
  const users = enrollments.data?.users ?? [];
  const enrolled = new Set(users.map((u) => u.userId));
  const candidates = (members.data?.users ?? []).filter((u) => !enrolled.has(u.id));

  const message = (text: string, className = 'text-soft') => (
    <tr>
      <td colSpan={4} className={`p-8 text-center ${className}`}>
        {text}
      </td>
    </tr>
  );

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Parcours</h1>
          <p className="text-soft text-sm mt-1">Inscrire vos membres à des parcours d'apprentissage</p>
        </div>
      </div>

      <div className="flex items-center gap-3 mb-5 flex-wrap">
        <label className="label m-0 whitespace-nowrap" htmlFor="path-select">
          Parcours :
        </label>
        <select
          id="path-select"
          className="input w-auto min-w-[220px]"
          value={slug}
          disabled={!paths.isSuccess}
          onChange={(event) => {
            setSlug(event.target.value);
            setUserId('');
          }}
        >
          {paths.isPending ? (
            <option value="">Chargement…</option>
          ) : paths.isError ? (
            <option value="">Erreur de chargement</option>
          ) : pathList.length === 0 ? (
            <option value="">Aucun parcours disponible</option>
          ) : (
            <>
              <option value="">— Choisir un parcours —</option>
              {pathList.map((path) => (
                <option key={path.slug} value={path.slug}>
                  {path.title ?? path.slug}
                </option>
              ))}
            </>
          )}
        </select>
        {slug && enrollments.isSuccess && (
          <span className="text-soft text-sm">
            {users.length} inscrit{users.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      {slug && (
        <div className="card mb-5 !px-5 !py-4">
          <div className="flex-row-gap3-wrap">
            <select
              className="input flex-1 min-w-[200px]"
              aria-label="Membre"
              value={userId}
              onChange={(event) => setUserId(event.target.value)}
            >
              <option value="">Sélectionner un membre…</option>
              {candidates.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.username} ({u.email})
                </option>
              ))}
            </select>
            <button
              className="btn btn-primary btn-sm"
              disabled={enroll.isPending}
              onClick={() => (userId ? enroll.mutate() : toast.error('Sélectionnez un membre'))}
            >
              {enroll.isPending ? 'Inscription…' : 'Inscrire'}
            </button>
          </div>
        </div>
      )}

      <div className="card !p-0 overflow-x-auto">
        <table className="admin-table">
          <thead>
            <tr className="admin-tr-head">
              <th className="admin-th">Membre</th>
              <th className="admin-th">Progression</th>
              <th className="admin-th">Inscrit le</th>
              <th className="admin-th-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {!slug ? (
              message('Sélectionnez un parcours')
            ) : enrollments.isPending ? (
              <tr>
                <td colSpan={4} className="p-8 text-center">
                  <div className="loading-state !p-0">
                    <div className="spinner" />
                  </div>
                </td>
              </tr>
            ) : enrollments.isError ? (
              message(`Erreur : ${enrollments.error.message}`, 'text-bad')
            ) : users.length === 0 ? (
              message('Aucun membre inscrit à ce parcours')
            ) : (
              users.map((user) => <EnrollmentRow key={user.userId} user={user} onUnenroll={() => setToRemove(user)} />)
            )}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={toRemove !== null}
        onOpenChange={(open) => !open && setToRemove(null)}
        title="Désinscrire ce membre du parcours ?"
        description={toRemove?.email}
        confirmLabel="Désinscrire"
        onConfirm={() => toRemove && unenroll.mutate(toRemove.userId)}
      />
    </>
  );
}
