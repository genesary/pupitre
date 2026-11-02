import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import { adminCourses, clearAllCache, courseDefinition, createCourse, deleteCourse, exportCourseMarkdown } from '@pupitre/core/lib/api';
import type { Course } from '@pupitre/core/lib/api-types';
import { metaOf, saveDefinition, toYaml, withMeta, type CourseMeta } from '@/lib/course-definition';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { DefinitionModal } from '@/components/admin/DefinitionModal';
import { CourseForm } from '@/components/admin/courses/CourseForm';
import { EnrollmentsTab } from '@/components/admin/courses/EnrollmentsTab';
import { ImportDialog } from '@/components/admin/courses/ImportDialog';
import { ModulesTab } from '@/components/admin/courses/ModulesTab';

// /admin/courses (docs/specs/042-admin-courses.md), markup of pupitre's
// admin/courses/index.astro. A course is edited through its whole definition,
// read again and compared before every write.

const DIFFICULTY_BADGES: Record<string, string> = { beginner: 'badge-green', intermediate: 'badge-orange', advanced: 'badge-red' };

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

function CoursePanel({ slug, editing, onEditDone }: { slug: string; editing: boolean; onEditDone: () => void }) {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<'modules' | 'enrollments'>('modules');
  const definition = useQuery({ queryKey: ['admin', 'courses', slug, 'definition'], queryFn: () => courseDefinition(slug) });

  const save = useMutation({
    mutationFn: (meta: CourseMeta) => saveDefinition(slug, definition.data!.spec, (spec) => withMeta(spec, meta)),
    onSuccess: () => {
      toast.success('Cours enregistré');
      void queryClient.invalidateQueries({ queryKey: ['admin', 'courses'] });
      void queryClient.invalidateQueries({ queryKey: ['courses'] });
      onEditDone();
    },
    onError: (error) => {
      toast.error(error.message || 'Erreur');
      if (error.name === 'DefinitionChangedError') void definition.refetch();
    },
  });

  const moduleCount = definition.data?.spec.modules?.length;

  return (
    <>
      {editing &&
        (definition.isSuccess ? (
          <CourseForm
            key={definition.dataUpdatedAt}
            title={`Modifier — ${slug}`}
            initial={metaOf(definition.data.spec)}
            slug={slug}
            submitLabel="Enregistrer"
            pending={save.isPending}
            onCancel={onEditDone}
            onSubmit={(meta) => save.mutate(meta)}
          />
        ) : (
          <div className="course-inline-form">
            <div className="loading-state !p-0">
              <div className="spinner" />
            </div>
          </div>
        ))}
      <div className="panel">
        <div className="tab-bar" role="tablist">
          <button className={`tab-btn${tab === 'modules' ? ' active' : ''}`} role="tab" aria-selected={tab === 'modules'} onClick={() => setTab('modules')}>
            Modules <span className="tab-count">{moduleCount ?? '…'}</span>
          </button>
          <button className={`tab-btn${tab === 'enrollments' ? ' active' : ''}`} role="tab" aria-selected={tab === 'enrollments'} onClick={() => setTab('enrollments')}>
            Inscriptions
          </button>
        </div>
        <div className="tab-content">{tab === 'modules' ? <ModulesTab slug={slug} definition={definition} /> : <EnrollmentsTab slug={slug} />}</div>
      </div>
    </>
  );
}

function CourseItem({
  course,
  open,
  editing,
  onToggle,
  onEdit,
  onEditDone,
  onDefinition,
  onDelete,
}: {
  course: Course;
  open: boolean;
  editing: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onEditDone: () => void;
  onDefinition: () => void;
  onDelete: () => void;
}) {
  const exportMd = useMutation({
    mutationFn: () => exportCourseMarkdown(course.slug),
    onSuccess: ({ blob, filename }) => saveBlob(blob, filename ?? `${course.slug}.md`),
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  return (
    <div className={`course-item${editing ? ' editing' : ''}`}>
      <div className="course-header" role="button" tabIndex={0} aria-expanded={open} onClick={onToggle} onKeyDown={(event) => event.key === 'Enter' && onToggle()}>
        <div className="course-info">
          <div className="course-title">{course.title}</div>
          <div className="course-meta">
            <span className="course-slug">{course.slug}</span>
            {course.difficulty && <span className={`badge ${DIFFICULTY_BADGES[course.difficulty] ?? 'badge-gray'} text-[.7rem]`}>{course.difficulty}</span>}
            {course.category && <span className="badge badge-blue text-[.7rem]">{course.category}</span>}
            <span className={`badge ${course.isPublic ? 'badge-green' : 'badge-gray'} text-[.7rem]`}>{course.isPublic ? 'Public' : 'Privé'}</span>
            <span className="text-soft text-[.72rem]">
              {course.moduleCount} module{course.moduleCount > 1 ? 's' : ''}
            </span>
          </div>
        </div>
        <div className="course-actions" onClick={(event) => event.stopPropagation()}>
          <button className="link-btn violet" onClick={onDefinition}>
            ⎈ Définition
          </button>
          <button className="link-btn violet" title="Télécharger le cours en markdown" disabled={exportMd.isPending} onClick={() => exportMd.mutate()}>
            ↓ MD
          </button>
          <button className="link-btn gray" onClick={editing ? onEditDone : onEdit}>
            {editing ? 'Annuler' : 'Modifier'}
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
      {(open || editing) && <CoursePanel slug={course.slug} editing={editing} onEditDone={onEditDone} />}
    </div>
  );
}

function CourseDefinitionModal({ slug, onClose }: { slug: string; onClose: () => void }) {
  const definition = useQuery({ queryKey: ['admin', 'courses', slug, 'definition'], queryFn: () => courseDefinition(slug) });
  return (
    <DefinitionModal
      title={`Définition — ${slug}`}
      text={definition.isSuccess ? toYaml(definition.data) : definition.isError ? definition.error.message : 'Chargement…'}
      onClose={onClose}
    />
  );
}

export function AdminCourses() {
  const queryClient = useQueryClient();
  const courses = useQuery({ queryKey: ['admin', 'courses'], queryFn: adminCourses });
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);
  const [importing, setImporting] = useState(false);
  const [open, setOpen] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [definition, setDefinition] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<Course | null>(null);
  const [confirmCache, setConfirmCache] = useState(false);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'courses'] });
    void queryClient.invalidateQueries({ queryKey: ['courses'] });
  };
  const create = useMutation({
    mutationFn: ({ slug, meta }: { slug: string; meta: CourseMeta }) => createCourse(slug, withMeta({}, meta)),
    onSuccess: ({ slug }) => {
      toast.success('Cours créé');
      setCreating(false);
      setOpen(slug);
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const remove = useMutation({
    mutationFn: (slug: string) => deleteCourse(slug),
    onSuccess: () => {
      toast.success('Cours supprimé');
      refresh();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const cache = useMutation({
    mutationFn: clearAllCache,
    onSuccess: (data) => toast.success(`Cache Git vidé${data.reposCleared !== undefined ? ` (${data.reposCleared} dépôts)` : ''}`),
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const all = courses.data?.courses ?? [];
  const q = search.trim().toLowerCase();
  const shown = all.filter((c) => !q || `${c.title} ${c.slug} ${c.category}`.toLowerCase().includes(q));

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Cours</h1>
          <p className="text-soft text-sm mt-1">{courses.isSuccess ? `${all.length} cours` : 'Chargement…'}</p>
        </div>
        <div className="flex gap-2 flex-wrap">
          <button className="btn btn-primary btn-sm" onClick={() => setCreating(true)}>
            + Nouveau cours
          </button>
          <button className="btn btn-secondary btn-sm" onClick={() => setImporting(true)}>
            ⬆ Importer du markdown
          </button>
          <button className="btn btn-secondary btn-sm" disabled={cache.isPending} onClick={() => setConfirmCache(true)}>
            🗑 Vider tout le cache
          </button>
        </div>
      </div>

      {creating && (
        <CourseForm
          title="Nouveau cours"
          initial={metaOf({})}
          slug={null}
          submitLabel="Créer le cours"
          pending={create.isPending}
          onCancel={() => setCreating(false)}
          onSubmit={(meta, slug) => create.mutate({ slug, meta })}
        />
      )}

      <div className="mb-4">
        <input
          className="input w-full max-w-[300px]"
          placeholder="Rechercher un cours…"
          aria-label="Rechercher un cours"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {courses.isPending ? (
        <div className="loading-state">
          <div className="spinner" />
          <span className="text-soft text-sm">Chargement…</span>
        </div>
      ) : courses.isError ? (
        <p className="text-bad p-6">{courses.error.message}</p>
      ) : shown.length === 0 ? (
        <div className="card text-center p-10 text-ink-soft">{all.length ? 'Aucun cours trouvé' : 'Aucun cours.'}</div>
      ) : (
        shown.map((course) => (
          <CourseItem
            key={course.slug}
            course={course}
            open={open === course.slug}
            editing={editing === course.slug}
            onToggle={() => setOpen((current) => (current === course.slug ? null : course.slug))}
            onEdit={() => setEditing(course.slug)}
            onEditDone={() => setEditing(null)}
            onDefinition={() => setDefinition(course.slug)}
            onDelete={() => setToDelete(course)}
          />
        ))
      )}

      {importing && <ImportDialog courses={all} onClose={() => setImporting(false)} onImported={setOpen} />}
      {definition && <CourseDefinitionModal slug={definition} onClose={() => setDefinition(null)} />}
      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(isOpen) => !isOpen && setToDelete(null)}
        title={`Supprimer le cours « ${toDelete?.title ?? ''} » ?`}
        description="Ses modules et sa définition seront supprimés. Cette action est irréversible."
        confirmLabel="Supprimer"
        onConfirm={() => toDelete && remove.mutate(toDelete.slug)}
      />
      <ConfirmDialog
        open={confirmCache}
        onOpenChange={setConfirmCache}
        title="Vider tout le cache Git ?"
        description="Les modules git seront relus depuis leurs dépôts."
        confirmLabel="Vider"
        onConfirm={() => cache.mutate()}
      />
    </>
  );
}
