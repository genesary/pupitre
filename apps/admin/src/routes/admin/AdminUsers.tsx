import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import { adminUsers, authProviders, deleteUser, updateUserAdmin } from '@pupitre/core/lib/api';
import type { Role, ScopeUser } from '@pupitre/core/lib/api-types';
import { useCurrentUser } from '@pupitre/core/lib/session';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { Modal } from '@pupitre/core/components/Modal';

// /admin/users (docs/specs/041-admin-users.md), markup of pupitre's admin/users/index.astro.

const PAGE_SIZE = 50;

const ROLE_BADGES: Record<string, { className: string; label: string }> = {
  admin: { className: 'badge-red', label: 'Admin' },
  manager: { className: 'badge-orange', label: 'Manager' },
  student: { className: 'badge-blue', label: 'Étudiant' },
};

function EditUserModal({ user, isSelf, onClose }: { user: ScopeUser; isSelf: boolean; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [role, setRole] = useState<Role>(user.role);
  const [active, setActive] = useState(user.isActive);

  const save = useMutation({
    mutationFn: () => updateUserAdmin(user.id, { role, isActive: active }),
    onSuccess: (updated) => {
      // The row changes at once, then the list is read again.
      queryClient.setQueryData<Awaited<ReturnType<typeof adminUsers>>>(['admin', 'users'], (data) =>
        data
          ? { ...data, users: (data.users ?? []).map((u) => (u.id === user.id ? { ...u, role: updated.role, isActive: updated.isActive } : u)) }
          : data,
      );
      void queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
      toast.success('Utilisateur mis à jour');
      onClose();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title={`Modifier — ${user.username}`}
      overlayClassName="modal-overlay"
      contentClassName="card modal-box-md"
      titleClassName="mb-5"
    >
      <div className="stack gap-[14px]">
        <div className="form-group">
          <label className="label" htmlFor="user-role">
            Rôle
          </label>
          <select
            id="user-role"
            className="input"
            value={role}
            disabled={isSelf}
            onChange={(event) => setRole(event.target.value as Role)}
          >
            <option value="student">Étudiant</option>
            <option value="manager">Manager de groupe</option>
            <option value="admin">Administrateur</option>
          </select>
        </div>
        <div className="form-group">
          <label className="label flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              className="w-4 h-4 accent-blue"
              checked={active}
              disabled={isSelf}
              onChange={(event) => setActive(event.target.checked)}
            />
            Compte actif
          </label>
        </div>
        <p className="text-soft text-[.8rem]">
          {isSelf
            ? 'Vous ne pouvez pas modifier votre propre rôle ni désactiver votre compte.'
            : "Le changement s'appliquera à la prochaine connexion de l'utilisateur (au plus tard dans 8 h)."}
        </p>
      </div>
      <div className="modal-footer">
        <button className="btn btn-secondary" onClick={onClose}>
          Annuler
        </button>
        <button className="btn btn-primary" disabled={isSelf || save.isPending} onClick={() => save.mutate()}>
          {save.isPending ? 'Enregistrement…' : 'Enregistrer'}
        </button>
      </div>
    </Modal>
  );
}

export function AdminUsers() {
  const me = useCurrentUser();
  const queryClient = useQueryClient();
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: adminUsers });
  const providers = useQuery({ queryKey: ['admin', 'providers'], queryFn: authProviders });
  const [provider, setProvider] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(0);
  const [editing, setEditing] = useState<ScopeUser | null>(null);
  const [toDelete, setToDelete] = useState<ScopeUser | null>(null);

  const remove = useMutation({
    mutationFn: (id: string) => deleteUser(id),
    onSuccess: () => {
      toast.success('Utilisateur supprimé');
      void queryClient.invalidateQueries({ queryKey: ['admin'] });
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const all = users.data?.users ?? [];
  const counts = providers.data?.providers ?? [];
  const totalCount = counts.reduce((sum, p) => sum + p.count, 0) || all.length;
  const q = search.toLowerCase().trim();
  const visible = all
    .filter((u) => !provider || (u.authProvider || 'local') === provider)
    .filter((u) => !q || u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
  const pages = Math.max(1, Math.ceil(visible.length / PAGE_SIZE));
  const current = Math.min(page, pages - 1);
  const shown = visible.slice(current * PAGE_SIZE, (current + 1) * PAGE_SIZE);

  const tabs = [{ provider: '', label: `Tous (${totalCount})` }, ...counts.map((p) => ({ provider: p.provider, label: `${p.provider} (${p.count})` }))];

  const message = (text: string, className = 'text-soft') => (
    <tr>
      <td colSpan={6} className={`p-8 text-center ${className}`}>
        {text}
      </td>
    </tr>
  );

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Utilisateurs</h1>
          <p className="text-soft text-sm mt-1">
            {users.isSuccess ? `${users.data.total ?? all.length} utilisateurs` : 'Chargement…'}
          </p>
        </div>
      </div>

      <div className="flex items-center gap-[10px] mb-5 flex-wrap">
        <input
          className="input w-full max-w-[260px]"
          placeholder="Rechercher par nom ou email…"
          aria-label="Rechercher un utilisateur"
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
            setPage(0);
          }}
        />
        <div className="flex gap-1.5 flex-wrap">
          {users.isSuccess &&
            tabs.map((tab) => (
              <button
                key={tab.provider || 'all'}
                className={`btn btn-sm ${provider === tab.provider ? 'btn-primary' : 'btn-secondary'}`}
                aria-pressed={provider === tab.provider}
                onClick={() => {
                  setProvider(tab.provider);
                  setPage(0);
                }}
              >
                {tab.label}
              </button>
            ))}
        </div>
      </div>

      <div className="card !p-0 overflow-x-auto">
        <table className="admin-table">
          <thead>
            <tr className="admin-tr-head">
              <th className="admin-th">Utilisateur</th>
              <th className="admin-th">Rôle</th>
              <th className="admin-th">Provider</th>
              <th className="admin-th">Cours</th>
              <th className="admin-th">Inscrit le</th>
              <th className="admin-th-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.isPending ? (
              <tr>
                <td colSpan={6} className="p-8 text-center">
                  <div className="loading-state !p-0">
                    <div className="spinner" />
                    <span className="text-soft text-sm">Chargement…</span>
                  </div>
                </td>
              </tr>
            ) : users.isError ? (
              message(`Erreur : ${users.error.message}`, 'text-bad')
            ) : shown.length === 0 ? (
              message('Aucun utilisateur trouvé')
            ) : (
              shown.map((u) => {
                const badge = ROLE_BADGES[u.role] ?? ROLE_BADGES.student!;
                const isSelf = u.id === me?.id;
                return (
                  <tr key={u.id} className="admin-tr">
                    <td className="admin-td">
                      <div className="flex-row-gap3">
                        <div className={`user-avatar-34 ${u.role === 'admin' ? 'lb-avatar-admin' : 'lb-avatar-user'}`}>
                          {(u.username.trim()[0] ?? '?').toUpperCase()}
                        </div>
                        <div>
                          <div className="username-text">{u.username}</div>
                          <div className="email-text">{u.email}</div>
                        </div>
                      </div>
                    </td>
                    <td className="admin-td">
                      <span className={`badge ${badge.className}`}>{badge.label}</span>
                      {!u.isActive && <span className="badge badge-gray ml-1">Inactif</span>}
                    </td>
                    <td className="admin-td">
                      <span className="badge badge-gray">{u.authProvider || 'local'}</span>
                    </td>
                    <td className="admin-td text-soft text-sm">{u.enrolledCourses ?? '—'}</td>
                    <td className="admin-td text-soft text-sm">{new Date(u.createdAt).toLocaleDateString('fr-FR')}</td>
                    <td className="admin-td-right whitespace-nowrap">
                      <button className="btn btn-secondary btn-sm" onClick={() => setEditing(u)}>
                        Modifier
                      </button>
                      <button
                        className="btn btn-sm btn-danger ml-1"
                        disabled={isSelf}
                        title={isSelf ? 'Vous ne pouvez pas supprimer votre propre compte' : undefined}
                        onClick={() => setToDelete(u)}
                      >
                        Supprimer
                      </button>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div className="flex items-center justify-center gap-3 mt-4">
          <button className="btn btn-secondary btn-sm" disabled={current === 0} onClick={() => setPage(current - 1)}>
            ‹ Précédent
          </button>
          <span className="text-soft text-sm">
            Page {current + 1} / {pages}
          </span>
          <button className="btn btn-secondary btn-sm" disabled={current >= pages - 1} onClick={() => setPage(current + 1)}>
            Suivant ›
          </button>
        </div>
      )}

      {editing && <EditUserModal user={editing} isSelf={editing.id === me?.id} onClose={() => setEditing(null)} />}
      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title={`Supprimer ${toDelete?.username ?? ''} ?`}
        description="Cette action est irréversible."
        confirmLabel="Supprimer"
        onConfirm={() => toDelete && remove.mutate(toDelete.id)}
      />
    </>
  );
}
