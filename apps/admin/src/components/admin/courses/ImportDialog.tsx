import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';

import { importCourse, importPreview, type ImportPreview, type ImportRequest } from '@pupitre/core/lib/api';
import type { Course } from '@pupitre/core/lib/api-types';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { Modal, ModalClose } from '@pupitre/core/components/Modal';

// « Importer depuis du markdown » of docs/specs/042-admin-courses.md, markup of
// pupitre's admin/courses page. The server refuses documents above 10 MB.

const MAX_BYTES = 10 * 1024 * 1024;
const TYPE_BADGES: Record<string, string> = { text: 'badge-blue', video: 'badge-orange', quiz: 'badge-green', lab: 'badge-purple' };
const DIFFICULTY_BADGES: Record<string, string> = { beginner: 'badge-green', intermediate: 'badge-orange', advanced: 'badge-red' };

function PreviewResult({ preview }: { preview: ImportPreview }) {
  const course = preview.course ?? {};
  const hasMeta = Boolean(course.title || course.category || course.difficulty);
  return (
    <div className="import-preview">
      <p className="section-label">
        Résultat — <span className="font-mono">{preview.slug}</span>
      </p>
      <p className="mb-2 text-[.85rem] flex gap-1.5 flex-wrap items-center">
        {hasMeta ? (
          <>
            {course.title && <strong>{course.title}</strong>}
            {course.category && <span className="badge badge-blue text-[.7rem]">{course.category}</span>}
            {course.difficulty && <span className={`badge ${DIFFICULTY_BADGES[course.difficulty] ?? 'badge-gray'} text-[.7rem]`}>{course.difficulty}</span>}
            <span className={`badge ${course.public ? 'badge-green' : 'badge-gray'} text-[.7rem]`}>{course.public ? 'Public' : 'Privé'}</span>
          </>
        ) : (
          <span className="text-soft">Aucune métadonnée de cours dans le document.</span>
        )}
      </p>
      <p className="section-label">{preview.modules.length} module(s)</p>
      {preview.modules.length === 0 ? (
        <p className="text-soft text-sm">Aucun module.</p>
      ) : (
        preview.modules.map((m, index) => (
          <div key={`${index}-${m.slug}`} className={`import-mod${m.existing ? ' existing' : ''}`}>
            <span className="import-mod-idx">{index}</span>
            <span className="import-mod-name">{m.name}</span>
            <span className={`badge ${TYPE_BADGES[m.type] ?? 'badge-gray'} text-[.7rem]`}>{m.type}</span>
            {m.existing && <span className="badge badge-gray text-[.7rem]">existant</span>}
            {m.hidden && <span className="badge badge-red text-[.7rem]">masqué</span>}
            {m.src && (
              <span className="text-[.72rem] text-ink-soft font-mono">
                {m.src}
                {m.path ? ` · ${m.path}` : ''}
              </span>
            )}
            <span className="text-[.72rem] text-ink-soft">{m.bytes ? `${m.bytes} c.` : '—'}</span>
          </div>
        ))
      )}
      {(preview.warnings ?? []).map((warning, index) => (
        <div key={index} className="import-warn">
          ⚠ {warning}
        </div>
      ))}
    </div>
  );
}

export function ImportDialog({ courses, onClose, onImported }: { courses: Course[]; onClose: () => void; onImported: (slug: string) => void }) {
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<ImportRequest['mode']>('create');
  const [slug, setSlug] = useState('');
  const [target, setTarget] = useState(courses[0]?.slug ?? '');
  const [split, setSplit] = useState('');
  const [markdown, setMarkdown] = useState('');
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [confirmReplace, setConfirmReplace] = useState(false);

  const bytes = new TextEncoder().encode(markdown).length;
  const body = (): ImportRequest => ({
    markdown,
    mode,
    split: split || undefined,
    slug: mode === 'create' ? slug.trim() || undefined : target,
  });
  const check = () => {
    if (!markdown.trim()) {
      toast.error('Collez ou chargez un document');
      return false;
    }
    if (bytes > MAX_BYTES) {
      toast.error('Le document dépasse 10 Mo');
      return false;
    }
    return true;
  };

  const analyse = useMutation({
    mutationFn: () => importPreview(body()),
    onSuccess: setPreview,
    onError: (error) => toast.error(error.message || 'Erreur'),
  });
  const submit = useMutation({
    mutationFn: () => importCourse(body()),
    onSuccess: (result) => {
      toast.success(`${result.moduleCount} module(s) importé(s) dans « ${result.slug} »`);
      (result.warnings ?? []).forEach((warning) => toast.warning(warning));
      void queryClient.invalidateQueries({ queryKey: ['admin', 'courses'] });
      onImported(result.slug);
      onClose();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const readFile = (file: File | undefined) => {
    if (!file) return;
    if (file.size > MAX_BYTES) {
      toast.error('Le fichier dépasse 10 Mo');
      return;
    }
    void file.text().then((text) => {
      setMarkdown(text);
      setPreview(null);
    });
  };

  return (
    <Modal
      open
      onOpenChange={(open) => !open && onClose()}
      title="Importer depuis du markdown"
      overlayClassName="modal-overlay open"
      contentClassName="modal-box modal-box-wide"
      header={(title) => (
        <div className="modal-head">
          <h3>{title}</h3>
          <ModalClose className="btn btn-ghost btn-sm" />
        </div>
      )}
    >
      <div className="modal-body">
        <div className="import-grid">
          <div>
            <label className="label" htmlFor="im-mode">
              Destination
            </label>
            <select
              className="input"
              id="im-mode"
              value={mode}
              onChange={(event) => {
                setMode(event.target.value as ImportRequest['mode']);
                setPreview(null);
              }}
            >
              <option value="create">Nouveau cours</option>
              <option value="replace">Remplacer les modules d'un cours</option>
              <option value="append">Ajouter des modules à un cours</option>
            </select>
          </div>
          {mode === 'create' ? (
            <div>
              <label className="label" htmlFor="im-slug">
                Slug
              </label>
              <input className="input font-mono" id="im-slug" type="text" placeholder="depuis le frontmatter" value={slug} onChange={(event) => setSlug(event.target.value)} />
            </div>
          ) : (
            <div>
              <label className="label" htmlFor="im-course">
                Cours cible
              </label>
              <select className="input" id="im-course" value={target} onChange={(event) => setTarget(event.target.value)}>
                {courses.map((course) => (
                  <option key={course.slug} value={course.slug}>
                    {course.title}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div>
            <label className="label" htmlFor="im-split">
              Découpage
            </label>
            <select className="input" id="im-split" value={split} onChange={(event) => setSplit(event.target.value)}>
              <option value="">Auto (frontmatter, sinon un seul module)</option>
              <option value="none">Un seul module</option>
              <option value="h1"># — un module par titre de niveau 1</option>
              <option value="h2">## — niveau 2</option>
              <option value="h3">### — niveau 3</option>
            </select>
          </div>
        </div>
        <label className="label" htmlFor="im-markdown">
          Document
        </label>
        <p className="label-hint">
          Collez le markdown, déposez un fichier dessus, ou choisissez-en un. Un en-tête <code>---</code> optionnel décrit le cours (title,
          description, category, difficulty, public…).
        </p>
        <textarea
          className="input import-editor"
          id="im-markdown"
          rows={14}
          spellCheck={false}
          placeholder={"---\ntitle: Introduction à Linux\ncategory: linux\n---\n\n## Qu'est-ce que Linux ?\n\nUn noyau…"}
          value={markdown}
          onChange={(event) => {
            setMarkdown(event.target.value);
            setPreview(null);
          }}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault();
            readFile(event.dataTransfer.files[0]);
          }}
        />
        <div className="flex items-center gap-3 mt-2">
          <input
            type="file"
            accept=".md,.markdown,.txt,text/markdown,text/plain"
            className="text-[.78rem]"
            aria-label="Fichier markdown"
            onChange={(event) => readFile(event.target.files?.[0])}
          />
          {markdown && <span className="text-soft text-[.75rem]">{(bytes / 1024).toFixed(1)} Ko</span>}
        </div>
        {preview && (
          <div className="mt-4">
            <PreviewResult preview={preview} />
          </div>
        )}
      </div>
      <div className="modal-foot">
        <button className="btn btn-secondary btn-sm" disabled={analyse.isPending} onClick={() => check() && analyse.mutate()}>
          {analyse.isPending ? 'Analyse…' : 'Prévisualiser'}
        </button>
        <button
          className="btn btn-primary btn-sm"
          disabled={submit.isPending}
          onClick={() => {
            if (!check()) return;
            if (mode === 'replace') setConfirmReplace(true);
            else submit.mutate();
          }}
        >
          {submit.isPending ? 'Import…' : 'Importer'}
        </button>
      </div>
      <ConfirmDialog
        open={confirmReplace}
        onOpenChange={setConfirmReplace}
        title={`Remplacer tous les modules de « ${target} » par ce document ?`}
        confirmLabel="Remplacer"
        onConfirm={() => submit.mutate()}
      />
    </Modal>
  );
}
