import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { toast } from 'sonner';

import {
  adminEnrollUser,
  adminGroups,
  adminUnenrollUser,
  courseEnrollments,
  enrollGroup,
  groupEnrollments,
  searchUsers,
  unenrollGroup,
} from '@pupitre/core/lib/api';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';

// Enrollments tab of a course (docs/specs/042-admin-courses.md): groups, then
// individual users found by search (250 ms debounce, 2 characters at least).

type Removal = { kind: 'group' | 'user'; id: string; name: string };

function useDebounced(value: string, delay: number) {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}

export function EnrollmentsTab({ slug }: { slug: string }) {
  const queryClient = useQueryClient();
  const users = useQuery({ queryKey: ['admin', 'courses', slug, 'enrollments'], queryFn: () => courseEnrollments(slug) });
  const groups = useQuery({ queryKey: ['admin', 'courses', slug, 'group-enrollments'], queryFn: () => groupEnrollments(slug) });
  const allGroups = useQuery({ queryKey: ['admin', 'groups'], queryFn: adminGroups });
  const [groupId, setGroupId] = useState('');
  const [search, setSearch] = useState('');
  const [picked, setPicked] = useState<{ id: string; username: string } | null>(null);
  const [toRemove, setToRemove] = useState<Removal | null>(null);
  const query = useDebounced(search.trim(), 250);
  const suggestions = useQuery({
    queryKey: ['admin', 'users', 'search', query],
    queryFn: () => searchUsers(query),
    enabled: query.length >= 2 && !picked,
  });

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'courses', slug] });
    void queryClient.invalidateQueries({ queryKey: ['admin', 'courses'] });
  };
  const onError = (error: Error) => toast.error(error.message || 'Erreur');

  const addGroup = useMutation({
    mutationFn: () => enrollGroup(slug, groupId),
    onSuccess: (data) => {
      setGroupId('');
      toast.success(`Groupe inscrit (${data.enrolled ?? 0} membre${(data.enrolled ?? 0) > 1 ? 's' : ''} inscrit${(data.enrolled ?? 0) > 1 ? 's' : ''})`);
      refresh();
    },
    onError,
  });
  const addUser = useMutation({
    mutationFn: (id: string) => adminEnrollUser(slug, id),
    onSuccess: () => {
      setPicked(null);
      setSearch('');
      toast.success('Utilisateur inscrit');
      refresh();
    },
    onError,
  });
  const remove = useMutation({
    mutationFn: (item: Removal) => (item.kind === 'group' ? unenrollGroup(slug, item.id) : adminUnenrollUser(slug, item.id)),
    onSuccess: () => {
      toast.success('Inscription retirée');
      refresh();
    },
    onError,
  });

  if (users.isPending || groups.isPending) {
    return (
      <div className="loading-state !py-5 !px-0">
        <div className="spinner" />
      </div>
    );
  }
  if (users.isError) return <p className="text-bad text-sm">{users.error.message}</p>;

  const enrolledGroups = groups.data?.groups ?? [];
  const enrolledUsers = users.data?.enrollments ?? [];
  const available = (allGroups.data?.groups ?? []).filter((g) => !enrolledGroups.some((e) => e.id === g.id));
  const found = (suggestions.data?.users ?? []).filter((u) => !enrolledUsers.some((e) => e.userId === u.id));

  return (
    <div className="stack">
      <div>
        <p className="section-label">Groupes</p>
        {enrolledGroups.length > 0 && (
          <div className="stack-sm mb-2.5">
            {enrolledGroups.map((g) => (
              <div key={g.id} className="enroll-row">
                <span className="text-[.875rem]">
                  <strong>{g.name}</strong>
                  <span className="text-soft text-sm ml-1.5">
                    {g.memberCount} membre{g.memberCount !== 1 ? 's' : ''}
                  </span>
                </span>
                <button className="link-btn red" onClick={() => setToRemove({ kind: 'group', id: g.id, name: g.name })}>
                  Retirer
                </button>
              </div>
            ))}
          </div>
        )}
        {available.length > 0 ? (
          <div className="flex-row">
            <select className="input text-[.85rem] flex-1" aria-label="Groupe à inscrire" value={groupId} onChange={(event) => setGroupId(event.target.value)}>
              <option value="">— Ajouter un groupe —</option>
              {available.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name} ({g.memberCount} membres)
                </option>
              ))}
            </select>
            <button className="btn btn-primary btn-sm" disabled={!groupId || addGroup.isPending} onClick={() => addGroup.mutate()}>
              Ajouter
            </button>
          </div>
        ) : (
          <p className="text-soft text-sm">{allGroups.data?.groups?.length ? 'Tous les groupes déjà inscrits.' : 'Aucun groupe disponible.'}</p>
        )}
      </div>
      <hr className="border-0 border-t border-line" />
      <div>
        <p className="section-label">Utilisateurs individuels</p>
        <div className="flex-row mb-3 relative">
          <div className="flex-1 relative">
            <input
              className="input"
              type="text"
              placeholder="Rechercher un utilisateur…"
              aria-label="Rechercher un utilisateur"
              autoComplete="off"
              value={search}
              onChange={(event) => {
                setSearch(event.target.value);
                setPicked(null);
              }}
            />
            {!picked && query.length >= 2 && found.length > 0 && (
              <div className="user-suggest-dropdown" role="listbox">
                {found.map((u) => (
                  <button
                    key={u.id}
                    type="button"
                    role="option"
                    aria-selected={false}
                    className="block w-full text-left px-3 py-2 hover:bg-fill text-[.85rem]"
                    onClick={() => {
                      setPicked({ id: u.id, username: u.username });
                      setSearch(u.username);
                    }}
                  >
                    <span className="font-medium">{u.username}</span> <span className="text-soft">{u.email}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
          <button
            className="btn btn-primary btn-sm"
            disabled={addUser.isPending}
            onClick={() => (picked ? addUser.mutate(picked.id) : toast.error('Choisissez un utilisateur dans la liste'))}
          >
            Inscrire
          </button>
        </div>
        {enrolledUsers.length === 0 ? (
          <p className="text-soft text-sm">Aucune inscription individuelle.</p>
        ) : (
          <div className="stack-sm">
            {enrolledUsers.map((e) => (
              <div key={e.userId} className="enroll-row">
                <div>
                  <span className="font-medium text-[.875rem]">{e.username}</span>
                  <span className="text-soft text-sm ml-1.5">{e.email}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-soft text-sm">{new Date(e.enrolledAt).toLocaleDateString('fr-FR')}</span>
                  <button className="link-btn red" onClick={() => setToRemove({ kind: 'user', id: e.userId, name: e.username })}>
                    Retirer
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      <ConfirmDialog
        open={toRemove !== null}
        onOpenChange={(open) => !open && setToRemove(null)}
        title={toRemove?.kind === 'group' ? `Retirer le groupe « ${toRemove.name} » ?` : `Désinscrire ${toRemove?.name ?? ''} ?`}
        confirmLabel="Retirer"
        onConfirm={() => toRemove && remove.mutate(toRemove)}
      />
    </div>
  );
}
