import { useDeferredValue, useMemo, useState } from 'react';

import type { ModuleSpec } from '@pupitre/core/lib/api';
import { cleanModule, takesMarkdown } from '@/lib/course-definition';
import { renderMarkdown } from '@pupitre/core/lib/markdown';
import { MarkdownContent } from '@pupitre/core/components/MarkdownContent';

// « Ajouter » / « Modifier » module form of docs/specs/042-admin-courses.md,
// markup of pupitre's admin/courses page. The fields without a control
// (questions, steps, prerequisites, skills…) are edited as JSON, validated
// before sending. The Astro « Quiz ref » field is not carried over (defect A5).

const TYPES = ['text', 'video', 'image', 'quiz', 'lab', 'modules'];
const BASE_KEYS = ['name', 'type', 'content', 'src', 'ref', 'path', 'hidden', 'inline'];

function advancedOf(module: ModuleSpec): string {
  const rest = Object.fromEntries(Object.entries(module).filter(([key]) => !BASE_KEYS.includes(key)));
  return Object.keys(rest).length ? JSON.stringify(rest, null, 2) : '';
}

type ModuleFormProps = {
  initial: ModuleSpec;
  isNew: boolean;
  pending: boolean;
  onCancel: () => void;
  onSubmit: (module: ModuleSpec) => void;
};

export function ModuleForm({ initial, isNew, pending, onCancel, onSubmit }: ModuleFormProps) {
  const [module, setModule] = useState<ModuleSpec>(initial);
  const [advanced, setAdvanced] = useState(() => advancedOf(initial));
  const [errors, setErrors] = useState<{ name?: string; advanced?: string }>({});
  const [preview, setPreview] = useState(false);
  const deferredContent = useDeferredValue(module.content ?? '');
  const previewHtml = useMemo(() => (preview ? renderMarkdown(deferredContent).html : ''), [preview, deferredContent]);

  const markdown = takesMarkdown(module.type);
  const hasGit = Boolean(module.src?.trim());
  const set = (patch: Partial<ModuleSpec>) => setModule((current) => ({ ...current, ...patch }));

  const submit = () => {
    const next: typeof errors = {};
    if (!module.name.trim()) next.name = 'Le nom est requis.';
    let extra: Record<string, unknown> = {};
    if (advanced.trim()) {
      try {
        const parsed: unknown = JSON.parse(advanced);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error();
        extra = parsed as Record<string, unknown>;
      } catch {
        next.advanced = 'JSON invalide : un objet est attendu, par exemple { "questions": [] }.';
      }
    }
    setErrors(next);
    if (Object.keys(next).length) return;
    const base = Object.fromEntries(Object.entries(module).filter(([key]) => BASE_KEYS.includes(key))) as ModuleSpec;
    onSubmit(cleanModule({ ...extra, ...base }));
  };

  return (
    <div>
      <div className="mod-form-grid">
        <div>
          <label className="label text-[.78rem]" htmlFor="mf-name">
            Nom *
          </label>
          <input
            className="input text-[.82rem]"
            id="mf-name"
            type="text"
            placeholder="Introduction"
            value={module.name}
            aria-invalid={Boolean(errors.name)}
            onChange={(event) => set({ name: event.target.value })}
          />
          {errors.name && <p className="text-bad text-[.78rem] mt-1">{errors.name}</p>}
        </div>
        <div>
          <label className="label text-[.78rem]" htmlFor="mf-type">
            Type
          </label>
          <select className="input text-[.82rem]" id="mf-type" value={module.type} onChange={(event) => set({ type: event.target.value })}>
            {TYPES.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </div>
      </div>

      {markdown && (
        <div className="mb-2.5">
          <div className="flex items-center justify-between">
            <label className="label text-[.78rem]" htmlFor="mf-content">
              Contenu (markdown)
            </label>
            <button type="button" className="link-btn gray" onClick={() => setPreview((value) => !value)}>
              {preview ? 'Éditer' : 'Aperçu'}
            </button>
          </div>
          <p className="label-hint">Le contenu du module. Laissez vide pour le lire depuis un dépôt git à la place.</p>
          {hasGit && <p className="label-hint text-orange">Une source git est renseignée : elle remplacera ce contenu à l'enregistrement.</p>}
          {preview ? (
            <div className="preview-area">
              <MarkdownContent html={previewHtml} />
            </div>
          ) : (
            <textarea
              className="input import-editor"
              id="mf-content"
              rows={10}
              spellCheck={false}
              placeholder={'## Introduction\n\nDu **markdown**…'}
              value={module.content ?? ''}
              onChange={(event) => set({ content: event.target.value })}
            />
          )}
        </div>
      )}

      <details className="mod-git-source" open={hasGit || !markdown}>
        <summary>{markdown ? 'Source git — alternative au contenu markdown ci-dessus' : 'Source git / URL de la ressource'}</summary>
        <div className="mod-form-grid">
          <div>
            <label className="label text-[.78rem]" htmlFor="mf-src">
              URL repo Git
            </label>
            <input
              className="input text-[.78rem] font-mono"
              id="mf-src"
              type="text"
              placeholder="https://github.com/org/repo"
              value={module.src ?? ''}
              onChange={(event) => set({ src: event.target.value })}
            />
          </div>
          <div>
            <label className="label text-[.78rem]" htmlFor="mf-ref">
              Branche / tag
            </label>
            <input
              className="input text-[.82rem] font-mono"
              id="mf-ref"
              type="text"
              placeholder="main"
              value={module.ref ?? ''}
              onChange={(event) => set({ ref: event.target.value })}
            />
          </div>
        </div>
        <div>
          <label className="label text-[.78rem]" htmlFor="mf-path">
            Chemin dans le repo
          </label>
          <input
            className="input text-[.78rem] font-mono"
            id="mf-path"
            type="text"
            placeholder="modules/01-intro/index.md"
            value={module.path ?? ''}
            onChange={(event) => set({ path: event.target.value })}
          />
        </div>
        {markdown && hasGit && (module.content ?? '').trim() && (
          <p className="label-hint text-orange">Le contenu markdown sera retiré : le serveur lit la source git en priorité.</p>
        )}
      </details>

      <label className="flex items-center gap-1.5 cursor-pointer mb-2 text-[.82rem]">
        <input type="checkbox" className="w-3.5 h-3.5" checked={Boolean(module.hidden)} onChange={(event) => set({ hidden: event.target.checked })} />
        Masqué <span className="text-soft text-[.75rem]">(non visible par les étudiants)</span>
      </label>
      {module.type === 'quiz' && (
        <label className="flex items-center gap-1.5 cursor-pointer mb-3 text-[.82rem]">
          <input type="checkbox" className="w-3.5 h-3.5" checked={Boolean(module.inline)} onChange={(event) => set({ inline: event.target.checked })} />
          Inline <span className="text-soft text-[.75rem]">(mini-quiz affiché en bas du module précédent)</span>
        </label>
      )}

      <details className="mb-3">
        <summary className="cursor-pointer text-[.82rem] font-medium">Champs avancés (JSON)</summary>
        <p className="label-hint mt-1.5">Questions, étapes de lab, prérequis, compétences, seuil de réussite… au format de la définition.</p>
        <textarea
          className="input import-editor"
          rows={8}
          spellCheck={false}
          aria-label="Champs avancés (JSON)"
          aria-invalid={Boolean(errors.advanced)}
          placeholder={'{\n  "skills": ["linux"]\n}'}
          value={advanced}
          onChange={(event) => setAdvanced(event.target.value)}
        />
        {errors.advanced && <p className="text-bad text-[.78rem] mt-1">{errors.advanced}</p>}
      </details>

      <div className="flex gap-1.5">
        <button type="button" className="btn btn-primary btn-sm" disabled={pending} onClick={submit}>
          {pending ? 'Enregistrement…' : isNew ? 'Ajouter' : 'Enregistrer'}
        </button>
        <button type="button" className="btn btn-secondary btn-sm" onClick={onCancel}>
          Annuler
        </button>
      </div>
    </div>
  );
}
