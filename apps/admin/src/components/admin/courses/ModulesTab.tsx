import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import { clearCourseCache, clearModuleCache, type CourseDefinition, type ModuleSpec } from '@pupitre/core/lib/api';
import { removeModule, saveDefinition, setModule, type DefinitionChangedError } from '@/lib/course-definition';
import { moveItem } from '@/lib/reorder';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { ModuleForm } from '@/components/admin/courses/ModuleForm';
import { SortableList } from '@/components/admin/SortableList';

// Modules tab of a course (docs/specs/042-admin-courses.md). The list is the
// definition's modules, hidden ones and git sources included; every change goes
// through saveDefinition (read again, compare, write whole).

const TYPE_BADGES: Record<string, string> = { text: 'badge-blue', video: 'badge-orange', quiz: 'badge-green', lab: 'badge-purple' };

const EMPTY: ModuleSpec = { name: '', type: 'text', content: '' };

export function ModulesTab({ slug, definition }: { slug: string; definition: UseQueryResult<CourseDefinition> }) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<number | 'new' | null>(null);
  const [order, setOrder] = useState<ModuleSpec[] | null>(null);
  const [toDelete, setToDelete] = useState<number | null>(null);

  const opened = definition.data?.spec;
  const modules = opened?.modules ?? [];
  const reordering = order !== null;

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'courses'] });
    void queryClient.invalidateQueries({ queryKey: ['course', slug] });
  };
  const onError = (error: Error | DefinitionChangedError) => {
    toast.error(error.message || 'Erreur');
    if (error.name === 'DefinitionChangedError') refresh();
  };

  const save = useMutation({
    mutationFn: ({ change }: { change: Parameters<typeof saveDefinition>[2]; message: string }) => saveDefinition(slug, opened!, change),
    onSuccess: (_, { message }) => {
      toast.success(message);
      setEditing(null);
      setOrder(null);
      refresh();
    },
    onError,
  });
  const cache = useMutation({
    mutationFn: (index: number | null) => (index === null ? clearCourseCache(slug) : clearModuleCache(slug, index)),
    onSuccess: (data) => toast.success(`Cache vidé${data.reposCleared !== undefined ? ` (${data.reposCleared} dépôt${data.reposCleared > 1 ? 's' : ''})` : ''}`),
    onError,
  });

  if (definition.isPending) {
    return (
      <div className="loading-state !py-5 !px-0">
        <div className="spinner" />
      </div>
    );
  }
  if (definition.isError) return <p className="text-bad text-sm">{definition.error.message}</p>;

  const list = order ?? modules;

  return (
    <>
      <div className="flex items-center justify-between mb-3 gap-2 flex-wrap">
        {reordering ? (
          <div className="flex gap-1.5">
            <button
              className="btn btn-primary btn-sm"
              disabled={save.isPending}
              onClick={() => save.mutate({ change: (spec) => ({ ...spec, modules: order }), message: 'Ordre enregistré' })}
            >
              Enregistrer l'ordre
            </button>
            <button className="btn btn-secondary btn-sm" onClick={() => setOrder(null)}>
              Annuler
            </button>
          </div>
        ) : (
          <button
            className="btn btn-secondary btn-sm"
            title="Activer le glisser-déposer pour réordonner"
            disabled={modules.length < 2 || editing !== null}
            onClick={() => setOrder(modules)}
          >
            ↕ Réordonner
          </button>
        )}
        <div className="flex gap-1.5">
          {!reordering && editing === null && (
            <button className="btn btn-primary btn-sm" onClick={() => setEditing('new')}>
              + Ajouter
            </button>
          )}
          <button className="btn btn-secondary btn-sm" disabled={cache.isPending} onClick={() => cache.mutate(null)}>
            🗑 Cache
          </button>
        </div>
      </div>

      {editing === 'new' && (
        <div className="mod-form">
          <p className="text-[.78rem] font-bold text-blue mb-2.5">Nouveau module</p>
          <ModuleForm
            initial={EMPTY}
            isNew
            pending={save.isPending}
            onCancel={() => setEditing(null)}
            onSubmit={(module) => save.mutate({ change: (spec) => setModule(spec, null, module), message: 'Module ajouté' })}
          />
        </div>
      )}

      {list.length === 0 ? (
        <p className="text-soft text-sm">
          Aucun module. Cliquez <strong>+ Ajouter</strong>.
        </p>
      ) : (
        <SortableList
          items={list}
          locked={!reordering}
          keyOf={(module, index) => `${index}-${module.name}`}
          labelOf={(module) => module.name}
          onMove={(from, to) => setOrder((current) => moveItem(current ?? modules, from, to))}
          render={(module, index) => (
            <>
              <div className="mod-info">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="mod-name">{module.name}</span>
                  <span className={`badge ${TYPE_BADGES[module.type] ?? 'badge-gray'} text-[.7rem]`}>{module.type}</span>
                  {module.hidden && <span className="badge badge-red text-[.7rem]">masqué</span>}
                  {module.inline && <span className="badge badge-blue text-[.7rem]">inline</span>}
                  {module.src && <span className="badge badge-gray text-[.7rem]">git</span>}
                </div>
                {module.src && (
                  <div className="text-[.72rem] text-ink-soft mt-0.5 font-mono">
                    {module.path ? `${module.path} · ` : ''}
                    {module.src} @ {module.ref || 'main'}
                  </div>
                )}
                {editing === index && (
                  <div className="mod-inline-form">
                    <ModuleForm
                      initial={module}
                      isNew={false}
                      pending={save.isPending}
                      onCancel={() => setEditing(null)}
                      onSubmit={(next) => save.mutate({ change: (spec) => setModule(spec, index, next), message: 'Module enregistré' })}
                    />
                  </div>
                )}
              </div>
              {!reordering && editing !== index && (
                <div className="mod-actions">
                  {module.src && (
                    <button className="link-btn gray" title="Vider le cache de ce module" aria-label={`Vider le cache de ${module.name}`} onClick={() => cache.mutate(index)}>
                      🗑
                    </button>
                  )}
                  <button className="link-btn gray" disabled={editing !== null} onClick={() => setEditing(index)}>
                    Modifier
                  </button>
                  <button className="link-btn red" onClick={() => setToDelete(index)}>
                    Supprimer
                  </button>
                </div>
              )}
            </>
          )}
        />
      )}

      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title={`Supprimer le module « ${toDelete !== null ? (modules[toDelete]?.name ?? '') : ''} » ?`}
        confirmLabel="Supprimer"
        onConfirm={() => toDelete !== null && save.mutate({ change: (spec) => removeModule(spec, toDelete), message: 'Module supprimé' })}
      />
    </>
  );
}
