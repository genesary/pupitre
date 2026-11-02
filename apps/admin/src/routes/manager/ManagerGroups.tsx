import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';

import {
  managerAddGroupMember,
  managerCreateGroup,
  managerCreateSubgroup,
  managerDeleteGroup,
  managerGroupMembers,
  managerGroups,
  managerRemoveGroupMember,
  managerUsers,
} from '@pupitre/core/lib/api';
import { buildTree, type GroupNode as Node } from '@/lib/group-tree';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { Modal, ModalClose } from '@pupitre/core/components/Modal';

// /manager/groups (docs/specs/030-manager-space.md), markup of pupitre's
// manager/groups/index.astro; its inline styles are written as Tailwind classes.

type Target = { id: string; name: string };

function GroupRow({
  node,
  indent,
  onMembers,
  onSubgroup,
  onDelete,
}: {
  node: Node;
  indent: number;
  onMembers: (group: Target) => void;
  onSubgroup: (group: Target) => void;
  onDelete: (group: Target) => void;
}) {
  return (
    <>
      <div
        className="flex items-center gap-2.5 py-3 pr-[18px] border-b border-line"
        style={{ paddingLeft: indent * 20 + 18 }}
      >
        {indent > 0 && <span className="text-ink-soft text-[.8rem] shrink-0">└</span>}
        <div className="flex-1 min-w-0">
          <div className="font-medium">{node.name}</div>
          <div className="text-[.78rem] text-ink-soft mt-0.5">
            {node.memberCount} membre{node.memberCount !== 1 ? 's' : ''}
            {node.depth > 0 ? ' · sous-groupe' : ''}
          </div>
        </div>
        <button className="btn btn-secondary btn-sm" onClick={() => onMembers(node)}>
          Membres
        </button>
        <button
          className="btn btn-secondary btn-sm"
          title="Créer un sous-groupe"
          aria-label={`Créer un sous-groupe de ${node.name}`}
          onClick={() => onSubgroup(node)}
        >
          +
        </button>
        <button className="btn btn-sm bg-bad-soft text-bad shrink-0" onClick={() => onDelete(node)}>
          Supprimer
        </button>
      </div>
      {node.children.map((child) => (
        <GroupRow
          key={child.id}
          node={child}
          indent={indent + 1}
          onMembers={onMembers}
          onSubgroup={onSubgroup}
          onDelete={onDelete}
        />
      ))}
    </>
  );
}

function NameModal({
  title,
  subtitle,
  label,
  placeholder,
  onClose,
  onSubmit,
  pending,
}: {
  title: string;
  subtitle?: string;
  label: string;
  placeholder: string;
  onClose: () => void;
  onSubmit: (name: string) => void;
  pending: boolean;
}) {
  const [name, setName] = useState('');
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim()) {
      toast.error('Nom requis');
      return;
    }
    onSubmit(name.trim());
  };
  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title={title}
      contentClassName="card w-[380px] max-w-[95vw] p-7"
      titleClassName={subtitle ? 'mb-1.5' : 'mb-5'}
    >
      {subtitle && <p className="text-soft text-sm mb-5">{subtitle}</p>}
      <form onSubmit={submit}>
        <div className="form-group mb-5">
          <label className="label" htmlFor="group-name">
            {label}
          </label>
          <input
            id="group-name"
            className="input"
            placeholder={placeholder}
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </div>
        <div className="flex gap-2 justify-end">
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            Annuler
          </button>
          <button type="submit" className="btn btn-primary" disabled={pending}>
            {pending ? '…' : 'Créer'}
          </button>
        </div>
      </form>
    </Modal>
  );
}

const ROLE_BADGES: Record<string, string> = { admin: 'badge-red', manager: 'badge-orange' };

function MembersModal({ group, onClose }: { group: Target; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [userId, setUserId] = useState('');
  const members = useQuery({
    queryKey: ['manager', 'groups', group.id, 'members'],
    queryFn: () => managerGroupMembers(group.id),
  });
  // Only members of the scope are offered (spec 030, audit finding H2).
  const scope = useQuery({ queryKey: ['manager', 'users'], queryFn: managerUsers });

  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['manager'] });
  const add = useMutation({
    mutationFn: () => managerAddGroupMember(group.id, userId),
    onSuccess: () => {
      setUserId('');
      toast.success('Membre ajouté');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const remove = useMutation({
    mutationFn: (memberId: string) => managerRemoveGroupMember(group.id, memberId),
    onSuccess: () => {
      toast.success('Membre retiré');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const list = members.data?.members ?? [];
  const candidates = (scope.data?.users ?? []).filter((u) => !list.some((m) => m.id === u.id));
  const sectionTitle = 'text-[.82rem] font-semibold uppercase tracking-[.05em] text-ink-soft mb-2.5';

  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title={`Membres — ${group.name}`}
      contentClassName="card w-[540px] max-w-[95vw] p-7 max-h-[90vh] overflow-y-auto"
      header={(title) => (
        <div className="flex items-center justify-between mb-5">
          <h3>{title}</h3>
          <ModalClose className="bg-transparent border-0 cursor-pointer text-[1.25rem] text-ink-soft" />
        </div>
      )}
    >
      <div className="mb-5">
        <h4 className={sectionTitle}>Membres actuels</h4>
        {members.isPending ? (
          <div className="loading-state py-3 px-0">
            <div className="spinner w-5 h-5" />
          </div>
        ) : members.isError ? (
          <p className="text-bad text-[.875rem]">{members.error.message}</p>
        ) : list.length === 0 ? (
          <p className="text-soft text-sm">Aucun membre.</p>
        ) : (
          list.map((u) => (
            <div key={u.id} className="flex items-center gap-2.5 py-2 border-b border-line">
              <div className="w-[30px] h-[30px] rounded-full bg-blue-soft flex items-center justify-center font-bold text-[.78rem] text-blue shrink-0">
                {(u.username.trim()[0] ?? '?').toUpperCase()}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-[.875rem] font-medium">{u.username}</div>
                <div className="text-[.75rem] text-ink-soft">{u.email}</div>
              </div>
              <span className={`badge ${ROLE_BADGES[u.role] ?? 'badge-blue'} shrink-0`}>{u.role}</span>
              <button
                className="btn btn-sm bg-bad-soft text-bad shrink-0"
                disabled={remove.isPending}
                onClick={() => remove.mutate(u.id)}
              >
                Retirer
              </button>
            </div>
          ))
        )}
      </div>
      <div className="border-t border-line pt-5">
        <h4 className={sectionTitle}>Ajouter un membre</h4>
        <div className="flex gap-2">
          <select
            className="input flex-1 h-auto py-[9px] px-3.5"
            aria-label="Utilisateur"
            value={userId}
            onChange={(event) => setUserId(event.target.value)}
          >
            <option value="">Choisir un utilisateur…</option>
            {candidates.map((u) => (
              <option key={u.id} value={u.id}>
                {u.username} ({u.email})
              </option>
            ))}
          </select>
          <button
            className="btn btn-primary"
            disabled={add.isPending}
            onClick={() => (userId ? add.mutate() : toast.error('Choisissez un utilisateur'))}
          >
            {add.isPending ? '…' : 'Ajouter'}
          </button>
        </div>
      </div>
    </Modal>
  );
}

export function ManagerGroups() {
  const queryClient = useQueryClient();
  const groups = useQuery({ queryKey: ['manager', 'groups'], queryFn: managerGroups });
  const [creating, setCreating] = useState(false);
  const [parent, setParent] = useState<Target | null>(null);
  const [membersOf, setMembersOf] = useState<Target | null>(null);
  const [toDelete, setToDelete] = useState<Target | null>(null);

  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['manager'] });
  const create = useMutation({
    mutationFn: (name: string) => managerCreateGroup(name),
    onSuccess: () => {
      setCreating(false);
      toast.success('Groupe créé');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const createSub = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => managerCreateSubgroup(id, name),
    onSuccess: () => {
      setParent(null);
      toast.success('Sous-groupe créé');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const remove = useMutation({
    mutationFn: (id: string) => managerDeleteGroup(id),
    onSuccess: () => {
      toast.success('Groupe supprimé');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const roots = buildTree(groups.data?.groups ?? []);

  return (
    <>
      <div className="flex items-center justify-between mb-7 flex-wrap gap-3">
        <div>
          <h1 className="text-[1.5rem]">Mes groupes</h1>
          <p className="text-soft text-sm mt-1">Gérez les groupes dont vous êtes responsable et leurs membres</p>
        </div>
        <button className="btn btn-primary" onClick={() => setCreating(true)}>
          + Nouveau groupe
        </button>
      </div>

      <div className="card p-0 overflow-hidden">
        {groups.isPending ? (
          <div className="loading-state p-8">
            <div className="spinner" />
          </div>
        ) : groups.isError ? (
          <p className="text-bad p-5">{groups.error.message}</p>
        ) : roots.length === 0 ? (
          <p className="text-soft text-sm p-5">Aucun groupe. Créez-en un pour commencer.</p>
        ) : (
          roots.map((node) => (
            <GroupRow
              key={node.id}
              node={node}
              indent={0}
              onMembers={setMembersOf}
              onSubgroup={setParent}
              onDelete={setToDelete}
            />
          ))
        )}
      </div>

      {creating && (
        <NameModal
          title="Nouveau groupe"
          label="Nom du groupe"
          placeholder="ex : equipe-frontend"
          pending={create.isPending}
          onClose={() => setCreating(false)}
          onSubmit={(name) => create.mutate(name)}
        />
      )}
      {parent && (
        <NameModal
          title="Nouveau sous-groupe"
          subtitle={`Sous-groupe de : ${parent.name}`}
          label="Nom du sous-groupe"
          placeholder="ex : equipe-backend"
          pending={createSub.isPending}
          onClose={() => setParent(null)}
          onSubmit={(name) => createSub.mutate({ id: parent.id, name })}
        />
      )}
      {membersOf && <MembersModal group={membersOf} onClose={() => setMembersOf(null)} />}
      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title={`Supprimer le groupe « ${toDelete?.name ?? ''} » ?`}
        description="Cette action est irréversible."
        confirmLabel="Supprimer"
        onConfirm={() => toDelete && remove.mutate(toDelete.id)}
      />
    </>
  );
}
