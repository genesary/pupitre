import { useState } from 'react';

import type { CourseMeta } from '@/lib/course-definition';
import { slugError } from '@/lib/slug';

// Course metadata of docs/specs/042-admin-courses.md, used by « Nouveau cours »
// (with the slug) and « Modifier ». Markup of pupitre's draft panel.

type CourseFormProps = {
  title: string;
  initial: CourseMeta;
  /** Locked slug when editing; null on creation. */
  slug: string | null;
  submitLabel: string;
  pending: boolean;
  onCancel: () => void;
  onSubmit: (meta: CourseMeta, slug: string) => void;
};

export function CourseForm({ title, initial, slug: lockedSlug, submitLabel, pending, onCancel, onSubmit }: CourseFormProps) {
  const [slug, setSlug] = useState(lockedSlug ?? '');
  const [meta, setMeta] = useState(initial);
  const [errors, setErrors] = useState<{ slug?: string; title?: string; xp?: string }>({});
  const set = (patch: Partial<CourseMeta>) => setMeta((current) => ({ ...current, ...patch }));
  const small = lockedSlug !== null;
  const labelClass = `label${small ? ' text-[.78rem]' : ''}`;

  const submit = () => {
    const next: typeof errors = {};
    if (lockedSlug === null) {
      const error = slugError(slug.trim());
      if (error) next.slug = error;
    }
    if (!meta.title.trim()) next.title = 'Le titre est requis.';
    if (!Number.isInteger(meta.xpRequired) || meta.xpRequired < 0) next.xp = 'Un nombre entier positif ou nul.';
    setErrors(next);
    if (Object.keys(next).length === 0) onSubmit(meta, lockedSlug ?? slug.trim());
  };

  return (
    <div className={small ? 'course-inline-form' : 'draft-panel'}>
      <h3 className={small ? 'text-[.78rem] font-bold text-blue mb-3.5' : 'mb-4'}>{title}</h3>
      <div className="draft-grid">
        <div>
          <label className={labelClass} htmlFor="c-slug">
            Slug {lockedSlug === null && <span className="text-bad">*</span>}
          </label>
          <input
            className="input font-mono"
            id="c-slug"
            type="text"
            placeholder="mon-cours"
            value={slug}
            disabled={lockedSlug !== null}
            aria-invalid={Boolean(errors.slug)}
            aria-describedby={errors.slug ? 'c-slug-error' : undefined}
            onChange={(event) => setSlug(event.target.value)}
          />
          {errors.slug && (
            <p className="slug-warn" id="c-slug-error">
              {errors.slug}
            </p>
          )}
        </div>
        <div>
          <label className={labelClass} htmlFor="c-title">
            Titre <span className="text-bad">*</span>
          </label>
          <input
            className="input"
            id="c-title"
            type="text"
            placeholder="Introduction à Kubernetes"
            value={meta.title}
            aria-invalid={Boolean(errors.title)}
            onChange={(event) => set({ title: event.target.value })}
          />
          {errors.title && <p className="slug-warn">{errors.title}</p>}
        </div>
      </div>
      <div className="mb-3.5">
        <label className={labelClass} htmlFor="c-desc">
          Description
        </label>
        <textarea
          className="input"
          id="c-desc"
          rows={2}
          placeholder="Une courte description du cours…"
          value={meta.description}
          onChange={(event) => set({ description: event.target.value })}
        />
      </div>
      <div className="draft-grid">
        <div>
          <label className={labelClass} htmlFor="c-category">
            Catégorie
          </label>
          <input className="input" id="c-category" type="text" placeholder="devops" value={meta.category} onChange={(event) => set({ category: event.target.value })} />
        </div>
        <div>
          <label className={labelClass} htmlFor="c-difficulty">
            Difficulté
          </label>
          <select className="input" id="c-difficulty" value={meta.difficulty} onChange={(event) => set({ difficulty: event.target.value })}>
            <option value="beginner">Débutant</option>
            <option value="intermediate">Intermédiaire</option>
            <option value="advanced">Avancé</option>
          </select>
        </div>
      </div>
      <div className="draft-grid">
        <div>
          <label className={labelClass} htmlFor="c-xp">
            XP requis
          </label>
          <input
            className="input"
            id="c-xp"
            type="number"
            min={0}
            step={1}
            value={meta.xpRequired}
            aria-invalid={Boolean(errors.xp)}
            onChange={(event) => set({ xpRequired: Number(event.target.value) })}
          />
          {errors.xp && <p className="slug-warn">{errors.xp}</p>}
        </div>
        <div className="flex gap-2">
          <div className="flex-1">
            <label className={labelClass} htmlFor="c-badge-name">
              Badge
            </label>
            <input className="input" id="c-badge-name" type="text" placeholder="Explorateur Linux" value={meta.badgeName} onChange={(event) => set({ badgeName: event.target.value })} />
          </div>
          <div className="w-20">
            <label className={labelClass} htmlFor="c-badge-icon">
              Emoji
            </label>
            <input className="input text-center" id="c-badge-icon" type="text" placeholder="🐧" value={meta.badgeIcon} onChange={(event) => set({ badgeIcon: event.target.value })} />
          </div>
        </div>
      </div>
      <div className="flex flex-wrap gap-x-5 gap-y-2 mb-4 text-[.875rem]">
        <label className="flex items-center gap-2 cursor-pointer">
          <input type="checkbox" className="w-4 h-4" checked={meta.public} onChange={(event) => set({ public: event.target.checked })} />
          <span>
            Public <span className="text-soft text-[.8rem]">(visible sans inscription)</span>
          </span>
        </label>
        <label className="flex items-center gap-2 cursor-pointer">
          <input type="checkbox" className="w-4 h-4" checked={meta.hidden} onChange={(event) => set({ hidden: event.target.checked })} />
          <span>
            Masqué <span className="text-soft text-[.8rem]">(admins uniquement)</span>
          </span>
        </label>
        <label className="flex items-center gap-2 cursor-pointer">
          <input type="checkbox" className="w-4 h-4" checked={meta.inPerson} onChange={(event) => set({ inPerson: event.target.checked })} />
          <span>Présentiel</span>
        </label>
      </div>
      <div className="flex gap-2">
        <button className="btn btn-primary btn-sm" disabled={pending} onClick={submit}>
          {pending ? 'Enregistrement…' : submitLabel}
        </button>
        <button className="btn btn-secondary btn-sm" onClick={onCancel}>
          Annuler
        </button>
      </div>
    </div>
  );
}
