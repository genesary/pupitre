import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useDeferredValue, useMemo, useState } from 'react';
import { toast } from 'sonner';

import { createPattern, deletePattern, listPatterns, updatePattern, type PatternInput } from '@pupitre/core/lib/api';
import type { MarkdownPattern } from '@pupitre/core/lib/api-types';
import { renderMarkdown } from '@pupitre/core/lib/markdown';
import { usePatternStyles } from '@pupitre/core/lib/use-pattern-styles';
import { ConfirmDialog } from '@pupitre/core/components/ConfirmDialog';
import { MarkdownContent } from '@pupitre/core/components/MarkdownContent';
import { DefinitionModal } from '@/components/admin/DefinitionModal';

// /admin/patterns (docs/specs/045-admin-patterns.md), markup of pupitre's
// admin/patterns/index.astro. The preview uses the lesson pipeline (sanitized
// HTML, one <style> element); the JS field is stored but never run (defect A13).

const EMPTY: PatternInput = {
  name: '',
  label: '',
  scope: 'global',
  html: '<span class="md-pat-{{name}}">{{content}}</span>',
  css: '.md-pat-{{name}} {\n  \n}',
  js: '',
};

/** The body accepted by POST /api/admin/patterns, for seeding. */
function patternDefinition(p: PatternInput): string {
  return JSON.stringify(
    { name: p.name, label: p.label || p.name, scope: p.scope || 'global', html: p.html, css: p.css || '', js: p.js || '' },
    null,
    2,
  );
}

function Preview({ draft }: { draft: PatternInput }) {
  const [markdown, setMarkdown] = useState('');
  const deferred = useDeferredValue(draft);
  const name = deferred.name.trim();
  const sample =
    markdown.trim() ||
    `Exemple inline : |||${name} texte en ligne|||.\n\n|||${name}\nContenu **bloc** sur plusieurs lignes.\n|||`;
  const html = useMemo(
    () => (name && deferred.html.trim() ? renderMarkdown(sample, { patterns: { [name]: { html: deferred.html, css: deferred.css } } }).html : ''),
    [name, deferred.html, deferred.css, sample],
  );
  usePatternStyles(name ? deferred.css : '');

  return (
    <div className="form-card sticky top-4">
      <h3 className="mb-3">Aperçu</h3>
      {name && (
        <div className="mb-3 flex gap-2 flex-wrap">
          <code className="code-snippet">|||{name}\n…\n|||</code>
          <code className="code-snippet">|||{name} texte|||</code>
        </div>
      )}
      <div>
        <label className="label" htmlFor="f-preview-md">
          Markdown personnalisé
        </label>
        <textarea
          className="input font-mono text-[.82rem]"
          id="f-preview-md"
          rows={4}
          placeholder="Tapez du markdown pour tester…"
          value={markdown}
          onChange={(event) => setMarkdown(event.target.value)}
        />
      </div>
      <div className="preview-area">
        {html ? (
          <MarkdownContent html={html} />
        ) : (
          <p className="text-soft text-sm italic">Remplissez le nom et le HTML pour prévisualiser.</p>
        )}
      </div>
    </div>
  );
}

function PatternForm({
  initial,
  editing,
  onClose,
  onDefinition,
}: {
  initial: PatternInput;
  editing: boolean;
  onClose: () => void;
  onDefinition: (draft: PatternInput) => void;
}) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(initial);
  const [previousName, setPreviousName] = useState(initial.name);

  const save = useMutation({
    mutationFn: (body: PatternInput) => (editing ? updatePattern(body.name, body) : createPattern(body)),
    onSuccess: (_, body) => {
      toast.success(editing ? `Pattern « ${body.name} » mis à jour` : `Pattern « ${body.name} » créé`);
      void queryClient.invalidateQueries({ queryKey: ['patterns'] });
      onClose();
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const set = (key: keyof PatternInput, value: string) => setDraft((current) => ({ ...current, [key]: value }));

  // Like the Astro page, renaming updates the md-pat-<name> class in the HTML and CSS.
  const rename = (value: string) => {
    const name = value.trim();
    const from = previousName ? `md-pat-${previousName}` : 'md-pat-{{name}}';
    const to = name ? `md-pat-${name}` : 'md-pat-{{name}}';
    setDraft((current) => ({
      ...current,
      name: value,
      html: current.html.replaceAll(from, to),
      css: current.css.replaceAll(from, to),
    }));
    setPreviousName(name);
  };

  const submit = () => {
    const trimmed = { ...draft, name: draft.name.trim(), label: draft.label.trim(), scope: draft.scope.trim() || 'global' };
    // The server requires a label (patterns.go): the name stands in when empty.
    if (!trimmed.label) trimmed.label = trimmed.name;
    if (!trimmed.name || !trimmed.html.trim()) {
      toast.error('Nom et HTML requis');
      return;
    }
    setDraft(trimmed);
    save.mutate(trimmed);
  };

  return (
    <div className="mb-6">
      <div className="pat-grid">
        <div className="form-card">
          <h3 className="mb-4">{editing ? `Modifier — ${initial.name}` : 'Nouveau pattern'}</h3>
          <div className="form-stack">
            <div className="form-grid2">
              <div>
                <label className="label" htmlFor="f-name">
                  Nom <span className="text-bad">*</span>
                </label>
                <input
                  className="input font-mono"
                  id="f-name"
                  type="text"
                  placeholder="blink"
                  value={draft.name}
                  disabled={editing}
                  onChange={(event) => rename(event.target.value)}
                />
                <p className="label-hint">
                  Utilisé comme identifiant : <code>|||{draft.name.trim() || 'blink'}</code>
                </p>
              </div>
              <div>
                <label className="label" htmlFor="f-label">
                  Label
                </label>
                <input
                  className="input"
                  id="f-label"
                  type="text"
                  placeholder="Clignotant"
                  value={draft.label}
                  onChange={(event) => set('label', event.target.value)}
                />
              </div>
            </div>
            <div>
              <label className="label" htmlFor="f-scope">
                Scope
              </label>
              <input
                className="input font-mono"
                id="f-scope"
                type="text"
                placeholder="global"
                value={draft.scope}
                onChange={(event) => set('scope', event.target.value)}
              />
            </div>
            <div>
              <label className="label" htmlFor="f-html">
                HTML <span className="text-bad">*</span>
              </label>
              <p className="label-hint">
                <code>{'{{content}}'}</code> → markdown rendu.
              </p>
              <textarea
                className="input font-mono text-[.82rem]"
                id="f-html"
                rows={3}
                value={draft.html}
                onChange={(event) => set('html', event.target.value)}
              />
            </div>
            <div>
              <label className="label" htmlFor="f-css">
                CSS
              </label>
              <textarea
                className="input font-mono text-[.82rem]"
                id="f-css"
                rows={4}
                value={draft.css}
                onChange={(event) => set('css', event.target.value)}
              />
            </div>
            <div>
              <label className="label" htmlFor="f-js">
                JS <span className="font-normal text-ink-soft">(non exécuté par la plateforme)</span>
              </label>
              <textarea
                className="input font-mono text-[.82rem]"
                id="f-js"
                rows={3}
                value={draft.js}
                onChange={(event) => set('js', event.target.value)}
              />
            </div>
            <div className="form-actions">
              <button className="btn btn-primary btn-sm" disabled={save.isPending} onClick={submit}>
                {save.isPending ? 'Enregistrement…' : editing ? 'Enregistrer' : 'Créer le pattern'}
              </button>
              <button
                className="btn btn-secondary btn-sm"
                disabled={!draft.name.trim() || !draft.html.trim()}
                onClick={() => onDefinition({ ...draft, name: draft.name.trim() })}
              >
                ⎈ YAML
              </button>
              <button className="btn btn-secondary btn-sm" onClick={onClose}>
                Annuler
              </button>
            </div>
          </div>
        </div>
        <Preview draft={draft} />
      </div>
    </div>
  );
}

const toInput = (p: MarkdownPattern): PatternInput => ({
  name: p.name,
  label: p.label,
  scope: p.scope || 'global',
  html: p.html,
  css: p.css,
  js: p.js,
});

export function Patterns() {
  const queryClient = useQueryClient();
  const patterns = useQuery({ queryKey: ['patterns'], queryFn: listPatterns });
  const [form, setForm] = useState<{ initial: PatternInput; editing: boolean } | null>(null);
  const [definition, setDefinition] = useState<PatternInput | null>(null);
  const [toDelete, setToDelete] = useState<MarkdownPattern | null>(null);

  const remove = useMutation({
    mutationFn: (name: string) => deletePattern(name),
    onSuccess: (_, name) => {
      toast.success(`Pattern « ${name} » supprimé`);
      void queryClient.invalidateQueries({ queryKey: ['patterns'] });
    },
    onError: (error) => toast.error(error.message || 'Erreur'),
  });

  const list = patterns.data?.patterns ?? [];

  return (
    <>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl">Markdown Patterns</h1>
          <p className="text-soft text-sm mt-1">Stockés en base et gérés depuis cette page.</p>
        </div>
        <button className="btn btn-primary btn-sm" onClick={() => setForm({ initial: EMPTY, editing: false })}>
          + Nouveau pattern
        </button>
      </div>

      {form && (
        <PatternForm
          key={`${form.initial.name}-${form.editing}`}
          initial={form.initial}
          editing={form.editing}
          onClose={() => setForm(null)}
          onDefinition={setDefinition}
        />
      )}

      {patterns.isPending ? (
        <div className="loading-state">
          <div className="spinner" />
          <span className="text-soft text-sm">Chargement…</span>
        </div>
      ) : patterns.isError ? (
        <p className="text-bad p-6">{patterns.error.message}</p>
      ) : list.length === 0 ? (
        <div className="card text-center p-10">
          <p className="text-soft">
            Aucun pattern. Cliquez <strong>+ Nouveau pattern</strong>.
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          {list.map((p) => (
            <div key={p.name} className={`pat-item${form?.editing && form.initial.name === p.name ? ' open' : ''}`}>
              <div className="pat-row">
                <div className="pat-info">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="pat-name">|||{p.name}</span>
                    {p.label && <span className="text-soft text-sm">{p.label}</span>}
                    {p.scope && p.scope !== 'global' ? (
                      <span className="badge badge-amber text-[.7rem]">{p.scope}</span>
                    ) : (
                      <span className="badge badge-gray text-[.7rem]">global</span>
                    )}
                    {p.fromConfig && <span className="badge badge-blue text-[.7rem]">config</span>}
                  </div>
                  <div className="pat-html">{p.html}</div>
                </div>
                <div className="pat-actions">
                  <button className="link-btn violet" onClick={() => setDefinition(toInput(p))}>
                    ⎈ YAML
                  </button>
                  <button
                    className="link-btn gray"
                    onClick={() => {
                      setForm({ initial: toInput(p), editing: true });
                      window.scrollTo({ top: 0, behavior: 'smooth' });
                    }}
                  >
                    Modifier
                  </button>
                  <button className="link-btn red" onClick={() => setToDelete(p)}>
                    Supprimer
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {definition && (
        <DefinitionModal
          title={`Définition — ${definition.name}`}
          text={patternDefinition(definition)}
          onClose={() => setDefinition(null)}
        />
      )}
      <ConfirmDialog
        open={toDelete !== null}
        onOpenChange={(open) => !open && setToDelete(null)}
        title={`Supprimer le pattern « ${toDelete?.name ?? ''} » ?`}
        description="Cette action est irréversible."
        confirmLabel="Supprimer"
        onConfirm={() => toDelete && remove.mutate(toDelete.name)}
      />
    </>
  );
}
