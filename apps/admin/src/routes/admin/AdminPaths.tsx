import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import {
  adminCourses,
  adminEnrollPath,
  adminPathEnrollments,
  adminUnenrollPath,
  adminUsers,
  createPath,
  deletePath,
  listPaths,
  pathDefinition,
  updatePath,
  type PathSpec,
} from '@pupitre/core/lib/api';
import type { LearningPath, PathEnrolledUser } from '@pupitre/core/lib/api-types';
import { moveItem } from '@/lib/reorder';
import { slugError } from '@/lib/slug';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { DefinitionModal } from '@/components/admin/DefinitionModal';
import { SortableList } from '@/components/admin/SortableList';

// /admin/paths (docs/specs/043-admin-paths.md), markup of pupitre's
// admin/paths/index.astro. Definitions are read through the admin route only.

const LEVELS: Record<string, { label: string; badge: string }> = {
  beginner: { label: 'Débutant', badge: 'badge-green' },
  intermediate: { label: 'Intermédiaire', badge: 'badge-orange' },
  advanced: { label: 'Avancé', badge: 'badge-red' },
};

type Draft = { title: string; description: string; kind: 'course' | 'skill'; level: string; courses: string[]; skills: string[] };

const toDraft = (spec: PathSpec): Draft => ({
  title: spec.title ?? '',
  description: spec.description ?? '',
  kind: spec.kind === 'skill' ? 'skill' : 'course',
  level: spec.level ?? '',
  courses: spec.courses ?? [],
  skills: spec.skills ?? [],
});

/** Only the members of the chosen kind are written (a course path gets no typed skills). */
const toSpec = (draft: Draft): PathSpec => ({
  title: draft.title.trim(),
  description: draft.description.trim() || undefined,
  kind: draft.kind,
  level: draft.level || undefined,
  ...(draft.kind === 'skill' ? { skills: draft.skills } : { courses: draft.courses }),
});

function MetaFields({ draft, onChange, small = false }: { draft: Draft; onChange: (draft: Draft) => void; small?: boolean }) {
  const labelClass = `label${small ? ' text-[.78rem]' : ''}`;
  return (
    <>
      <div className="mb-3.5">
        <label className={labelClass} htmlFor="path-desc">
          Description
        </label>
        <textarea
          className="input"
          id="path-desc"
          rows={2}
          placeholder="Une courte description du parcours…"
          value={draft.description}
          onChange={(event) => onChange({ ...draft, description: event.target.value })}
        />
      </div>
      <div className="draft-grid">
        <div>
          <label className={labelClass} htmlFor="path-kind">
            Type
          </label>
          <select
            className="input"
            id="path-kind"
            value={draft.kind}
            onChange={(event) => onChange({ ...draft, kind: event.target.value as Draft['kind'] })}
          >
            <option value="course">Suite de cours</option>
            <option value="skill">Suite de compétences</option>
          </select>
          <p className="label-hint">
            Un parcours de cours débloque ses cours dans l'ordre ; un parcours de compétences enchaîne des compétences.
          </p>
        </div>
        <div>
          <label className={labelClass} htmlFor="path-level">
            Niveau
          </label>
          <select className="input" id="path-level" value={draft.level} onChange={(event) => onChange({ ...draft, level: event.target.value })}>
            <option value="">— Aucun —</option>
            <option value="beginner">Débutant</option>
            <option value="intermediate">Intermédiaire</option>
            <option value="advanced">Avancé</option>
          </select>
        </div>
      </div>
    </>
  );
}

function Members({ draft, onChange }: { draft: Draft; onChange: (draft: Draft) => void }) {
  const courses = useQuery({ queryKey: ['admin', 'courses'], queryFn: adminCourses });
  const [toAdd, setToAdd] = useState('');
  const all = courses.data?.courses ?? [];
  const knownSkills = [...new Set(all.flatMap((c) => c.skills ?? []))].sort();
  const isSkill = draft.kind === 'skill';
  const list = isSkill ? draft.skills : draft.courses;
  const setList = (next: string[]) => onChange(isSkill ? { ...draft, skills: next } : { ...draft, courses: next });
  const available = isSkill ? knownSkills.filter((s) => !list.includes(s)) : all.filter((c) => !list.includes(c.slug));

  const add = () => {
    const value = toAdd.trim();
    if (!value || list.includes(value)) return;
    setList([...list, value]);
    setToAdd('');
  };

  return (
    <>
      <p className="section-label">{isSkill ? 'Compétences' : 'Cours'} — dans l'ordre de progression</p>
      {list.length === 0 ? (
        <p className="text-soft text-sm mb-3">Aucun élément. Ajoutez-en un ci-dessous.</p>
      ) : (
        <SortableList
          items={list}
          keyOf={(member) => member}
          labelOf={(member) => member}
          onMove={(from, to) => setList(moveItem(list, from, to))}
          render={(member, index) => {
            const course = isSkill ? null : all.find((c) => c.slug === member);
            const missing = courses.isSuccess && (isSkill ? !knownSkills.includes(member) : !course);
            return (
              <>
                <div className="mod-info">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <span className="mod-name">{course ? course.title : member}</span>
                    {missing && (
                      <span
                        className="badge badge-red text-[.7rem]"
                        title={isSkill ? 'Aucun module ne porte cette compétence' : 'Ce cours n’existe pas'}
                      >
                        {isSkill ? 'inutilisée' : 'introuvable'}
                      </span>
                    )}
                  </div>
                  {course && <div className="text-[.72rem] text-ink-soft mt-0.5 font-mono">{member}</div>}
                </div>
                <div className="mod-actions">
                  <button type="button" className="link-btn red" onClick={() => setList(list.filter((_, i) => i !== index))}>
                    Retirer
                  </button>
                </div>
              </>
            );
          }}
        />
      )}
      <div className="flex-row">
        {isSkill ? (
          <>
            <input
              className="input text-[.85rem] flex-1"
              list="skill-options"
              placeholder="Nom de la compétence…"
              aria-label="Compétence à ajouter"
              value={toAdd}
              onChange={(event) => setToAdd(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  add();
                }
              }}
            />
            <datalist id="skill-options">
              {(available as string[]).map((skill) => (
                <option key={skill} value={skill} />
              ))}
            </datalist>
          </>
        ) : available.length === 0 ? (
          <p className="text-soft text-sm flex-1">{all.length ? 'Tous les cours sont déjà dans ce parcours.' : 'Aucun cours disponible.'}</p>
        ) : (
          <select className="input text-[.85rem] flex-1" aria-label="Cours à ajouter" value={toAdd} onChange={(event) => setToAdd(event.target.value)}>
            <option value="">— Ajouter un cours —</option>
            {all
              .filter((c) => !list.includes(c.slug))
              .map((c) => (
                <option key={c.slug} value={c.slug}>
                  {c.title} ({c.slug})
                </option>
              ))}
          </select>
        )}
        <button type="button" className="btn btn-primary btn-sm" disabled={!toAdd.trim()} onClick={add}>
          Ajouter
        </button>
      </div>
    </>
  );
}

function NewPathPanel({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient();
  const [slug, setSlug] = useState('');
  const [touched, setTouched] = useState(false);
  const [draft, setDraft] = useState<Draft>(toDraft({}));
  const error = slugError(slug);

  const create = useMutation({
    mutationFn: () => createPath(slug, toSpec(draft)),
    onSuccess: () => {
      toast.success('Parcours créé');
      void queryClient.invalidateQueries({ queryKey: ['paths'] });
      onClose();
    },
    onError: (e) => toast.error(e.message || 'Erreur'),
  });

  return (
    <div className="draft-panel">
      <h3 className="mb-4">Nouveau parcours</h3>
      <div className="draft-grid">
        <div>
          <label className="label" htmlFor="d-slug">
            Slug <span className="text-bad">*</span>
          </label>
          <input
            className="input font-mono"
            id="d-slug"
            type="text"
            placeholder="mon-parcours"
            value={slug}
            aria-invalid={touched && Boolean(error)}
            onChange={(event) => setSlug(event.target.value)}
            onBlur={() => setTouched(true)}
          />
          {touched && error && <p className="slug-warn">{error}</p>}
        </div>
        <div>
          <label className="label" htmlFor="d-title">
            Titre <span className="text-bad">*</span>
          </label>
          <input
            className="input"
            id="d-title"
            type="text"
            placeholder="Parcours DevOps"
            value={draft.title}
            onChange={(event) => setDraft({ ...draft, title: event.target.value })}
          />
        </div>
      </div>
      <MetaFields draft={draft} onChange={setDraft} />
      <Members draft={draft} onChange={setDraft} />
      <div className="flex gap-2 mt-4">
        <button
          className="btn btn-primary btn-sm"
          disabled={create.isPending}
          onClick={() => {
            setTouched(true);
            if (error) return;
            if (!draft.title.trim()) {
              toast.error('Le titre est requis');
              return;
            }
            create.mutate();
          }}
        >
          Créer le parcours
        </button>
        <button className="btn btn-secondary btn-sm" onClick={onClose}>
          Annuler
        </button>
      </div>
    </div>
  );
}

function PathEditor({ slug, onClose }: { slug: string; onClose: () => void }) {
  const queryClient = useQueryClient();
  const definition = useQuery({ queryKey: ['admin', 'paths', slug, 'definition'], queryFn: () => pathDefinition(slug) });
  const [draft, setDraft] = useState<Draft | null>(null);
  const current = draft ?? (definition.data ? toDraft(definition.data.spec) : null);

  const save = useMutation({
    mutationFn: (next: Draft) => updatePath(slug, toSpec(next)),
    onSuccess: () => {
      toast.success('Parcours enregistré');
      void queryClient.invalidateQueries({ queryKey: ['paths'] });
      void queryClient.invalidateQueries({ queryKey: ['admin', 'paths', slug] });
      onClose();
    },
    onError: (e) => toast.error(e.message || 'Erreur'),
  });

  if (definition.isError) return <p className="text-bad p-4">{definition.error.message}</p>;
  if (!current) {
    return (
      <div className="course-inline-form">
        <div className="loading-state !p-0">
          <div className="spinner" />
        </div>
      </div>
    );
  }

  return (
    <div className="course-inline-form">
      <div className="draft-grid">
        <div>
          <label className="label text-[.78rem]" htmlFor="pe-slug">
            Slug
          </label>
          <input className="input font-mono text-[.82rem]" id="pe-slug" type="text" value={slug} disabled />
        </div>
        <div>
          <label className="label text-[.78rem]" htmlFor="pe-title">
            Titre <span className="text-bad">*</span>
          </label>
          <input
            className="input"
            id="pe-title"
            type="text"
            placeholder="Parcours DevOps"
            value={current.title}
            onChange={(event) => setDraft({ ...current, title: event.target.value })}
          />
        </div>
      </div>
      <MetaFields draft={current} onChange={setDraft} small />
      <Members draft={current} onChange={setDraft} />
      <div className="flex gap-2 mt-4">
        <button
          className="btn btn-primary btn-sm"
          disabled={save.isPending}
          onClick={() => (current.title.trim() ? save.mutate(current) : toast.error('Le titre est requis'))}
        >
          Enregistrer
        </button>
        <button className="btn btn-secondary btn-sm" onClick={onClose}>
          Annuler
        </button>
      </div>
    </div>
  );
}

function PathItem({
  path,
  open,
  onToggle,
  onDefinition,
  onDelete,
}: {
  path: LearningPath;
  open: boolean;
  onToggle: () => void;
  onDefinition: () => void;
  onDelete: () => void;
}) {
  const isSkill = path.kind === 'skill';
  const count = (isSkill ? path.skills : path.courses)?.length ?? 0;
  const level = path.level ? LEVELS[path.level] : undefined;
  return (
    <div className={`course-item${open ? ' editing' : ''}`}>
      <div className="course-header" role="button" tabIndex={0} aria-expanded={open} onClick={onToggle} onKeyDown={(event) => event.key === 'Enter' && onToggle()}>
        <div className="course-info">
          <div className="course-title">{path.title}</div>
          <div className="course-meta">
            <span className="course-slug">{path.slug}</span>
            <span className={`badge ${isSkill ? 'badge-purple' : 'badge-blue'} text-[.7rem]`}>{isSkill ? 'compétences' : 'cours'}</span>
            {path.level && <span className={`badge ${level?.badge ?? 'badge-gray'} text-[.7rem]`}>{level?.label ?? path.level}</span>}
            <span className="text-soft text-[.72rem]">{isSkill ? `${count} compétence${count > 1 ? 's' : ''}` : `${count} cours`}</span>
          </div>
        </div>
        <div className="course-actions" onClick={(event) => event.stopPropagation()}>
          <button className="link-btn violet" onClick={onDefinition}>
            ⎈ Définition
          </button>
          <button className="link-btn gray" onClick={onToggle}>
            {open ? 'Fermer' : 'Modifier'}
          </button>
          <button className="link-btn red" onClick={onDelete}>
            Supprimer
          </button>
        </div>
        <svg className={`chevron${open ? ' open' : ''}`} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path
            fillRule="evenodd"
            d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
            clipRule="evenodd"
          />
        </svg>
      </div>
      {open && <PathEditor slug={path.slug} onClose={onToggle} />}
    </div>
  );
}

function DefinitionOf({ slug, onClose }: { slug: string; onClose: () => void }) {
  const definition = useQuery({ queryKey: ['admin', 'paths', slug, 'definition'], queryFn: () => pathDefinition(slug) });
  return (
    <DefinitionModal
      title={`Définition — ${slug}`}
      text={definition.isSuccess ? JSON.stringify(definition.data, null, 2) : definition.isError ? definition.error.message : 'Chargement…'}
      onClose={onClose}
    />
  );
}

function Enrollments({ paths }: { paths: LearningPath[] }) {
  const queryClient = useQueryClient();
  const [slug, setSlug] = useState('');
  const [userId, setUserId] = useState('');
  const [toRemove, setToRemove] = useState<PathEnrolledUser | null>(null);
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: adminUsers });
  const enrollments = useQuery({ queryKey: ['admin', 'paths', slug, 'enrollments'], queryFn: () => adminPathEnrollments(slug), enabled: Boolean(slug) });

  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['admin', 'paths', slug, 'enrollments'] });
  const enroll = useMutation({
    mutationFn: () => adminEnrollPath(slug, userId),
    onSuccess: () => {
      setUserId('');
      toast.success('Utilisateur inscrit au parcours');
      refresh();
    },
    onError: (e) => toast.error(e.message || 'Erreur'),
  });
  const unenroll = useMutation({
    mutationFn: (id: string) => adminUnenrollPath(slug, id),
    onSuccess: () => {
      toast.success('Utilisateur désinscrit');
      refresh();
    },
    onError: (e) => toast.error(e.message || 'Erreur'),
  });

  const list = enrollments.data?.users ?? [];
  const enrolled = new Set(list.map((u) => u.userId));
  const candidates = (users.data?.users ?? []).filter((u) => !enrolled.has(u.id));
  const row = (text: string, className = 'text-soft') => (
    <tr>
      <td colSpan={4} className={`p-8 text-center ${className}`}>
        {text}
      </td>
    </tr>
  );

  return (
    <section>
      <div className="flex items-center gap-3 mb-5 flex-wrap">
        <label className="label m-0 whitespace-nowrap" htmlFor="path-select">
          Parcours :
        </label>
        <select id="path-select" className="input w-auto min-w-[220px]" value={slug} onChange={(event) => setSlug(event.target.value)}>
          <option value="">— Choisir un parcours —</option>
          {paths.map((p) => (
            <option key={p.slug} value={p.slug}>
              {p.title}
            </option>
          ))}
        </select>
        {slug && enrollments.isSuccess && (
          <span className="text-soft text-sm">
            {list.length} inscrit{list.length !== 1 ? 's' : ''}
          </span>
        )}
      </div>
      {slug && (
        <div className="card mb-5 !px-5 !py-4">
          <div className="flex-row-gap3-wrap">
            <select className="input flex-1 min-w-[200px]" aria-label="Utilisateur" value={userId} onChange={(event) => setUserId(event.target.value)}>
              <option value="">Sélectionner un utilisateur…</option>
              {candidates.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.username} ({u.email})
                </option>
              ))}
            </select>
            <button className="btn btn-primary btn-sm" disabled={enroll.isPending} onClick={() => (userId ? enroll.mutate() : toast.error('Sélectionnez un utilisateur'))}>
              Inscrire
            </button>
          </div>
        </div>
      )}
      <div className="card !p-0 overflow-x-auto">
        <table className="admin-table">
          <thead>
            <tr className="admin-tr-head">
              <th className="admin-th">Utilisateur</th>
              <th className="admin-th">Progression</th>
              <th className="admin-th">Inscrit le</th>
              <th className="admin-th-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {!slug
              ? row('Sélectionnez un parcours')
              : enrollments.isPending
                ? row('Chargement…')
                : enrollments.isError
                  ? row(`Erreur : ${enrollments.error.message}`, 'text-bad')
                  : list.length === 0
                    ? row('Aucun utilisateur inscrit à ce parcours')
                    : list.map((u) => {
                        const pct = u.totalCourses > 0 ? Math.round((u.completedCourses / u.totalCourses) * 100) : 0;
                        return (
                          <tr key={u.userId} className="admin-tr">
                            <td className="admin-td">
                              <div className="username-text">{u.email}</div>
                              <div className="row-sub">{u.role}</div>
                            </td>
                            <td className="admin-td">
                              <div className="path-progress-row">
                                <div className="progress-track">
                                  <div className={`progress-fill ${pct === 100 ? 'complete' : 'partial'}`} style={{ width: `${pct}%` }} />
                                </div>
                                <span className="path-progress-pct">
                                  {u.completedCourses}/{u.totalCourses}
                                </span>
                              </div>
                            </td>
                            <td className="admin-td text-soft text-sm whitespace-nowrap">{new Date(u.enrolledAt).toLocaleDateString('fr-FR')}</td>
                            <td className="admin-td-right">
                              <button className="btn btn-sm btn-danger" onClick={() => setToRemove(u)}>
                                Désinscrire
                              </button>
                            </td>
                          </tr>
                        );
                      })}
          </tbody>
        </table>
      </div>
      <ConfirmDialog
        open={toRemove !== null}
        onOpenChange={(open) => !open && setToRemove(null)}
        title="Désinscrire cet utilisateur du parcours ?"
        description={toRemove?.email}
        confirmLabel="Désinscrire"
        onConfirm={() => toRemove && unenroll.mutate(toRemove.userId)}
      />
    </section>
  );
}

export function AdminPaths() {
  const queryClient = useQueryClient();
  const paths = useQuery({ queryKey: ['paths'], queryFn: listPaths });
  const [view, setView] = useState<'paths' | 'enrollments'>('paths');
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const [open, setOpen] = useState<string | null>(null);
  const [definition, setDefinition] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<LearningPath | null>(null);

  const remove = useMutation({
    mutationFn: (slug: string) => deletePath(slug),
    onSuccess: () => {
      toast.success('Parcours supprimé');
      void queryClient.invalidateQueries({ queryKey: ['paths'] });
    },
    onError: (e) => toast.error(e.message || 'Erreur'),
  });

  const all = paths.data?.paths ?? [];
  const q = search.trim().toLowerCase();
  const shown = all.filter((p) => !q || `${p.title} ${p.slug} ${p.description ?? ''}`.toLowerCase().includes(q));

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Parcours</h1>
          <p className="text-soft text-sm mt-1">{paths.isSuccess ? `${all.length} parcours` : 'Chargement…'}</p>
        </div>
        <button className="btn btn-primary btn-sm" onClick={() => setCreating(true)}>
          + Nouveau parcours
        </button>
      </div>

      <div className="tab-bar !px-0 mb-5" role="tablist">
        <button className={`tab-btn${view === 'paths' ? ' active' : ''}`} role="tab" aria-selected={view === 'paths'} onClick={() => setView('paths')}>
          Parcours <span className="tab-count">{paths.isSuccess ? all.length : '…'}</span>
        </button>
        <button className={`tab-btn${view === 'enrollments' ? ' active' : ''}`} role="tab" aria-selected={view === 'enrollments'} onClick={() => setView('enrollments')}>
          Inscriptions
        </button>
      </div>

      {view === 'enrollments' ? (
        <Enrollments paths={all} />
      ) : (
        <section>
          {creating && <NewPathPanel onClose={() => setCreating(false)} />}
          <div className="mb-4">
            <input
              className="input w-full max-w-[300px]"
              placeholder="Rechercher un parcours…"
              aria-label="Rechercher un parcours"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          {paths.isPending ? (
            <div className="loading-state">
              <div className="spinner" />
              <span className="text-soft text-sm">Chargement…</span>
            </div>
          ) : paths.isError ? (
            <p className="text-bad p-6">{paths.error.message}</p>
          ) : shown.length === 0 ? (
            <div className="card text-center p-10 text-ink-soft">
              {all.length ? (
                'Aucun parcours trouvé'
              ) : (
                <>
                  Aucun parcours. Cliquez <strong>+ Nouveau parcours</strong>.
                </>
              )}
            </div>
          ) : (
            shown.map((path) => (
              <PathItem
                key={path.slug}
                path={path}
                open={open === path.slug}
                onToggle={() => setOpen((current) => (current === path.slug ? null : path.slug))}
                onDefinition={() => setDefinition(path.slug)}
                onDelete={() => setToDelete(path)}
              />
            ))
          )}
        </section>
      )}

      {definition && <DefinitionOf slug={definition} onClose={() => setDefinition(null)} />}
      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(isOpen) => !isOpen && setToDelete(null)}
        title={`Supprimer le parcours « ${toDelete?.title ?? ''} » ?`}
        description="Cette action est irréversible."
        confirmLabel="Supprimer"
        onConfirm={() => toDelete && remove.mutate(toDelete.slug)}
      />
    </>
  );
}
