import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { toast } from 'sonner';

import {
  addGroupMember,
  adminGroups,
  adminUsers,
  createGroup,
  createSubgroup,
  deleteGroup,
  deleteGroupMapping,
  groupMappings,
  groupMembers,
  moveGroup,
  removeGroupMember,
  renameGroup,
  saveGroupMapping,
} from '@pupitre/core/lib/api';
import type { AdminGroup, Role } from '@pupitre/core/lib/api-types';
import { buildTree, subtreeIds, type TreeNode } from '@/lib/group-tree';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { Modal, ModalClose } from '@pupitre/core/components/Modal';

// /admin/groups (docs/specs/044-admin-groups.md), markup of pupitre's
// admin/groups/index.astro; its inline styles are written as Tailwind classes.
// Descendants are computed from the group list: the descendants route returns
// a struct without json tags (defect A22).

const ROLE_BADGES: Record<string, string> = { admin: 'badge-red', manager: 'badge-orange' };
const roleBadge = (role: string) => ROLE_BADGES[role] ?? 'badge-blue';
const DEFAULT_GROUP = 'everyone';

type Target = { id: string; name: string };
type Dialog =
  | { kind: 'create' }
  | { kind: 'rename'; group: Target }
  | { kind: 'subgroup'; group: Target }
  | { kind: 'move'; group: Target }
  | { kind: 'members'; group: Target }
  | { kind: 'delete'; group: Target }
  | null;

const sectionTitle = 'text-[.82rem] font-semibold uppercase tracking-[.05em] text-ink-soft mb-2.5';

function useRefresh() {
  const queryClient = useQueryClient();
  return () => void queryClient.invalidateQueries({ queryKey: ['admin', 'groups'] });
}

function NameDialog({
  title,
  subtitle,
  label,
  placeholder,
  initial = '',
  submitLabel,
  onClose,
  onSubmit,
  pending,
}: {
  title: string;
  subtitle?: string;
  label: string;
  placeholder: string;
  initial?: string;
  submitLabel: string;
  onClose: () => void;
  onSubmit: (name: string) => void;
  pending: boolean;
}) {
  const [name, setName] = useState(initial);
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
          <input id="group-name" className="input" placeholder={placeholder} value={name} onChange={(event) => setName(event.target.value)} />
        </div>
        <div className="flex gap-2 justify-end">
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            Annuler
          </button>
          <button type="submit" className="btn btn-primary" disabled={pending}>
            {pending ? '…' : submitLabel}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function MoveDialog({ group, groups, onClose }: { group: Target; groups: AdminGroup[]; onClose: () => void }) {
  const refresh = useRefresh();
  const current = groups.find((g) => g.id === group.id)?.parentId ?? '';
  const [parent, setParent] = useState(current);
  // Neither the group nor its descendants: a cycle is impossible from here.
  const excluded = subtreeIds(groups, group.id);
  const options = groups.filter((g) => !excluded.has(g.id) && g.name !== DEFAULT_GROUP);
  const move = useMutation({
    mutationFn: () => moveGroup(group.id, parent || null),
    onSuccess: () => {
      toast.success('Groupe déplacé');
      refresh();
      onClose();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  return (
    <Modal open onOpenChange={(open) => !open && onClose()} title="Déplacer le groupe" contentClassName="card w-[420px] max-w-[95vw] p-7" titleClassName="mb-1.5">
      <p className="text-soft text-sm mb-5">{group.name}</p>
      <div className="form-group mb-5">
        <label className="label" htmlFor="move-parent">
          Nouveau parent
        </label>
        <select id="move-parent" className="input h-auto py-[9px] px-3.5" value={parent} onChange={(event) => setParent(event.target.value)}>
          <option value="">Racine (aucun parent)</option>
          {options.map((g) => (
            <option key={g.id} value={g.id}>
              {'— '.repeat(g.depth)}
              {g.name}
            </option>
          ))}
        </select>
      </div>
      <div className="flex gap-2 justify-end">
        <button className="btn btn-secondary" onClick={onClose}>
          Annuler
        </button>
        <button className="btn btn-primary" disabled={move.isPending || parent === current} onClick={() => move.mutate()}>
          Déplacer
        </button>
      </div>
    </Modal>
  );
}

function MembersDialog({ group, onClose }: { group: Target; onClose: () => void }) {
  const queryClient = useQueryClient();
  const refresh = useRefresh();
  const [recursive, setRecursive] = useState(false);
  const [search, setSearch] = useState('');
  const [userId, setUserId] = useState('');
  const members = useQuery({
    queryKey: ['admin', 'groups', group.id, 'members', recursive],
    queryFn: () => groupMembers(group.id, recursive),
  });
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: adminUsers });

  const afterChange = () => {
    refresh();
    void queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
  };
  const add = useMutation({
    mutationFn: () => addGroupMember(group.id, userId),
    onSuccess: () => {
      setUserId('');
      toast.success('Membre ajouté');
      afterChange();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const remove = useMutation({
    mutationFn: (id: string) => removeGroupMember(group.id, id),
    onSuccess: () => {
      toast.success('Membre retiré');
      afterChange();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const list = members.data?.members ?? [];
  const q = search.toLowerCase().trim();
  const candidates = (users.data?.users ?? [])
    .filter((u) => !list.some((m) => m.id === u.id && m.directMembership !== false))
    .filter((u) => !q || u.username.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));

  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title={`Membres — ${group.name}`}
      contentClassName="card w-[520px] max-w-[95vw] p-7 max-h-[90vh] overflow-y-auto"
      header={(title) => (
        <div className="flex items-center justify-between mb-4">
          <h3>{title}</h3>
          <ModalClose className="bg-transparent border-0 cursor-pointer text-[1.25rem] text-ink-soft" />
        </div>
      )}
    >
      <label className="flex items-center gap-2 mb-4 text-[.82rem] cursor-pointer text-ink-soft">
        <input type="checkbox" className="cursor-pointer" checked={recursive} onChange={(event) => setRecursive(event.target.checked)} />
        Inclure les membres des sous-groupes
      </label>
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
          list.map((u) => {
            const direct = !recursive || u.directMembership !== false;
            return (
              <div key={u.id} className="flex items-center gap-2.5 py-2 border-b border-line">
                <div className="w-[30px] h-[30px] rounded-full bg-blue-soft flex items-center justify-center font-bold text-[.78rem] text-blue shrink-0">
                  {(u.username.trim()[0] ?? '?').toUpperCase()}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[.875rem] font-medium">{u.username}</div>
                  <div className="text-[.75rem] text-ink-soft">{u.email}</div>
                </div>
                {recursive && (
                  <span className={`badge ${direct ? 'badge-blue' : 'badge-gray'} shrink-0 text-[.7rem]`}>{direct ? 'Direct' : 'Hérité'}</span>
                )}
                <span className={`badge ${roleBadge(u.role)} shrink-0`}>{u.role}</span>
                {direct && (
                  <button className="btn btn-sm bg-bad-soft text-bad shrink-0" disabled={remove.isPending} onClick={() => remove.mutate(u.id)}>
                    Retirer
                  </button>
                )}
              </div>
            );
          })
        )}
      </div>
      <div className="border-t border-line pt-5">
        <h4 className={sectionTitle}>Ajouter un membre</h4>
        <input
          className="input mb-2"
          type="search"
          placeholder="Filtrer les utilisateurs…"
          aria-label="Filtrer les utilisateurs"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div className="flex gap-2">
          <select className="input flex-1 h-auto py-[9px] px-3.5" aria-label="Utilisateur" value={userId} onChange={(event) => setUserId(event.target.value)}>
            <option value="">Choisir un utilisateur…</option>
            {candidates.map((u) => (
              <option key={u.id} value={u.id}>
                {u.username} ({u.email})
              </option>
            ))}
          </select>
          <button className="btn btn-primary" disabled={add.isPending} onClick={() => (userId ? add.mutate() : toast.error('Choisissez un utilisateur'))}>
            {add.isPending ? '…' : 'Ajouter'}
          </button>
        </div>
      </div>
    </Modal>
  );
}

function GroupRow({
  node,
  indent,
  expanded,
  onToggle,
  onOpen,
}: {
  node: TreeNode<AdminGroup>;
  indent: number;
  expanded: Set<string>;
  onToggle: (id: string) => void;
  onOpen: (dialog: Dialog) => void;
}) {
  const open = expanded.has(node.id);
  const hasChildren = node.children.length > 0;
  const local = node.source === 'local';
  const editable = local && node.name !== DEFAULT_GROUP;
  const target = { id: node.id, name: node.name };
  return (
    <>
      <div className="flex items-center gap-3 py-4 pr-5 border-b border-line" style={{ paddingLeft: indent * 20 + 20 }}>
        {hasChildren ? (
          <button
            className="bg-transparent border-0 cursor-pointer text-[.75rem] text-ink-soft px-1 py-0.5 shrink-0"
            aria-expanded={open}
            aria-label={open ? `Replier ${node.name}` : `Déplier ${node.name}`}
            onClick={() => onToggle(node.id)}
          >
            {open ? '▼' : '▶'}
          </button>
        ) : (
          <span className="inline-block w-[22px] shrink-0" />
        )}
        <div className="flex-1 min-w-0">
          <div className="font-medium text-[.875rem] whitespace-nowrap overflow-hidden text-ellipsis">{node.name}</div>
          <div className="text-[.75rem] text-ink-soft">
            {node.memberCount} membre{node.memberCount !== 1 ? 's' : ''}
            {!local && (
              <>
                {' · '}
                <span className="badge badge-orange text-[.7rem] px-[5px] py-px">SSO · {node.source}</span>
              </>
            )}
            {node.depth > 0 && (
              <>
                {' · '}
                <span className="badge badge-gray text-[.7rem] px-[5px] py-px">sous-groupe</span>
              </>
            )}
          </div>
        </div>
        {node.mappedRole ? (
          <span className={`badge ${roleBadge(node.mappedRole)}`}>{node.mappedRole}</span>
        ) : (
          !local && <span className="badge badge-gray">Non mappé</span>
        )}
        <button className="btn btn-secondary btn-sm shrink-0" onClick={() => onOpen({ kind: 'members', group: target })}>
          Membres
        </button>
        {editable && (
          <>
            <button className="btn btn-secondary btn-sm shrink-0" title="Nouveau sous-groupe" aria-label={`Nouveau sous-groupe de ${node.name}`} onClick={() => onOpen({ kind: 'subgroup', group: target })}>
              +
            </button>
            <button className="btn btn-secondary btn-sm shrink-0" title="Renommer" aria-label={`Renommer ${node.name}`} onClick={() => onOpen({ kind: 'rename', group: target })}>
              ✎
            </button>
            <button className="btn btn-secondary btn-sm shrink-0" title="Déplacer" aria-label={`Déplacer ${node.name}`} onClick={() => onOpen({ kind: 'move', group: target })}>
              ⇄
            </button>
          </>
        )}
        <button
          className="btn btn-sm bg-bad-soft text-bad shrink-0"
          title={editable ? 'Supprimer' : 'Seuls les groupes locaux peuvent être supprimés'}
          aria-label={`Supprimer ${node.name}`}
          disabled={!editable}
          onClick={() => onOpen({ kind: 'delete', group: target })}
        >
          ✕
        </button>
      </div>
      {open &&
        node.children.map((child) => (
          <GroupRow key={child.id} node={child} indent={indent + 1} expanded={expanded} onToggle={onToggle} onOpen={onOpen} />
        ))}
    </>
  );
}

function Mappings() {
  const queryClient = useQueryClient();
  const refresh = useRefresh();
  const mappings = useQuery({ queryKey: ['admin', 'groups', 'mappings'], queryFn: groupMappings });
  const [groupName, setGroupName] = useState('');
  const [role, setRole] = useState<Role>('student');

  const after = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'groups'] });
    refresh();
  };
  const save = useMutation({
    mutationFn: () => saveGroupMapping({ groupName: groupName.trim(), platformRole: role }),
    onSuccess: () => {
      setGroupName('');
      toast.success('Mapping enregistré');
      after();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const remove = useMutation({
    mutationFn: (name: string) => deleteGroupMapping(name),
    onSuccess: () => {
      toast.success('Mapping supprimé');
      after();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const list = mappings.data?.mappings ?? [];

  return (
    <div>
      <div className="mb-[18px]">
        <h2 className="text-[1.1rem]">Mappings de rôles</h2>
      </div>
      <div className="card p-0 overflow-hidden mb-5">
        {mappings.isPending ? (
          <div className="loading-state p-6">
            <div className="spinner" />
          </div>
        ) : mappings.isError ? (
          <p className="text-bad p-4">{mappings.error.message}</p>
        ) : list.length === 0 ? (
          <p className="text-soft text-sm p-5">Aucun mapping défini.</p>
        ) : (
          list.map((m) => (
            <div key={m.groupName} className="flex items-center gap-3 py-4 px-5 border-b border-line">
              <code className="flex-1 text-[.82rem] bg-fill py-[3px] px-[7px] rounded overflow-hidden text-ellipsis whitespace-nowrap">{m.groupName}</code>
              <span className="text-ink-soft shrink-0">→</span>
              <span className={`badge ${roleBadge(m.platformRole)} shrink-0`}>{m.platformRole}</span>
              <button
                className="btn btn-sm bg-bad-soft text-bad shrink-0"
                aria-label={`Supprimer le mapping ${m.groupName}`}
                disabled={remove.isPending}
                onClick={() => remove.mutate(m.groupName)}
              >
                ✕
              </button>
            </div>
          ))
        )}
      </div>
      <div className="card">
        <h3 className="text-[.95rem] mb-3.5">Ajouter un mapping</h3>
        <div className="stack gap-2.5">
          <div className="form-group">
            <label className="label" htmlFor="mapping-group">
              Nom du groupe SSO
            </label>
            <input id="mapping-group" className="input" placeholder="ex : devops-team" value={groupName} onChange={(event) => setGroupName(event.target.value)} />
          </div>
          <div className="form-group">
            <label className="label" htmlFor="mapping-role">
              Rôle plateforme
            </label>
            <select id="mapping-role" className="input h-auto py-[9px] px-3.5" value={role} onChange={(event) => setRole(event.target.value as Role)}>
              <option value="student">Étudiant</option>
              <option value="manager">Manager de groupe</option>
              <option value="admin">Administrateur</option>
            </select>
          </div>
          <button
            className="btn btn-primary btn-sm self-start"
            disabled={save.isPending}
            onClick={() => (groupName.trim() ? save.mutate() : toast.error('Nom du groupe requis'))}
          >
            Ajouter
          </button>
          <p className="text-soft text-[.8rem]">
            Le paramètre « Groupes administrateurs » (oidc_group_admins) fait foi pour le rôle admin : un utilisateur hors de ces
            groupes redevient étudiant à sa prochaine connexion.
          </p>
        </div>
      </div>
    </div>
  );
}

export function AdminGroups() {
  const refresh = useRefresh();
  const groups = useQuery({ queryKey: ['admin', 'groups'], queryFn: adminGroups });
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [dialog, setDialog] = useState<Dialog>(null);
  const close = () => setDialog(null);

  const all = groups.data?.groups ?? [];
  const roots = buildTree(all);

  const done = (message: string) => () => {
    toast.success(message);
    refresh();
    close();
  };
  const onError = (error: Error) => toast.error(error.message || 'Erreur');
  const create = useMutation({ mutationFn: createGroup, onSuccess: done('Groupe créé'), onError });
  const rename = useMutation({ mutationFn: ({ id, name }: { id: string; name: string }) => renameGroup(id, name), onSuccess: done('Groupe renommé'), onError });
  const sub = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => createSubgroup(id, name),
    onSuccess: (_, { id }) => {
      setExpanded((current) => new Set(current).add(id));
      done('Sous-groupe créé')();
    },
    onError,
  });
  const remove = useMutation({ mutationFn: deleteGroup, onSuccess: done('Groupe supprimé'), onError });

  const toggle = (id: string) =>
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  return (
    <>
      <div className="flex items-center justify-between mb-9 flex-wrap gap-4">
        <div>
          <h1 className="text-[1.5rem]">Groupes</h1>
          <p className="text-soft text-sm mt-1.5">Groupes synchronisés depuis Keycloak et groupes locaux</p>
        </div>
        <button className="btn btn-primary" onClick={() => setDialog({ kind: 'create' })}>
          + Nouveau groupe
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 items-start">
        <div>
          <div className="flex items-center justify-between mb-[18px]">
            <h2 className="text-[1.1rem]">
              Groupes {groups.isSuccess && <span className="badge badge-gray ml-1.5">{all.length}</span>}
            </h2>
          </div>
          <div className="card p-0 overflow-hidden">
            {groups.isPending ? (
              <div className="loading-state p-6">
                <div className="spinner" />
              </div>
            ) : groups.isError ? (
              <p className="text-bad p-4">{groups.error.message}</p>
            ) : roots.length === 0 ? (
              <p className="text-soft text-sm p-5">Aucun groupe.</p>
            ) : (
              roots.map((node) => (
                <GroupRow key={node.id} node={node} indent={0} expanded={expanded} onToggle={toggle} onOpen={setDialog} />
              ))
            )}
          </div>
        </div>
        <Mappings />
      </div>

      {dialog?.kind === 'create' && (
        <NameDialog
          title="Nouveau groupe local"
          label="Nom du groupe"
          placeholder="ex : equipe-rh"
          submitLabel="Créer"
          pending={create.isPending}
          onClose={close}
          onSubmit={(name) => create.mutate(name)}
        />
      )}
      {dialog?.kind === 'rename' && (
        <NameDialog
          title="Renommer le groupe"
          label="Nouveau nom"
          placeholder="ex : equipe-rh"
          initial={dialog.group.name}
          submitLabel="Enregistrer"
          pending={rename.isPending}
          onClose={close}
          onSubmit={(name) => rename.mutate({ id: dialog.group.id, name })}
        />
      )}
      {dialog?.kind === 'subgroup' && (
        <NameDialog
          title="Nouveau sous-groupe"
          subtitle={`Sous-groupe de : ${dialog.group.name}`}
          label="Nom du sous-groupe"
          placeholder="ex : equipe-backend"
          submitLabel="Créer"
          pending={sub.isPending}
          onClose={close}
          onSubmit={(name) => sub.mutate({ id: dialog.group.id, name })}
        />
      )}
      {dialog?.kind === 'move' && <MoveDialog group={dialog.group} groups={all} onClose={close} />}
      {dialog?.kind === 'members' && <MembersDialog group={dialog.group} onClose={close} />}
      <ConfirmDialog
        open={dialog?.kind === 'delete'}
        onOpenChange={(open) => !open && close()}
        title={`Supprimer le groupe « ${dialog?.kind === 'delete' ? dialog.group.name : ''} » ?`}
        description="Un groupe qui a des sous-groupes ou des membres ne peut pas être supprimé."
        confirmLabel="Supprimer"
        onConfirm={() => dialog?.kind === 'delete' && remove.mutate(dialog.group.id)}
      />
    </>
  );
}
