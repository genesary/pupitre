import { courseDefinition, updateCourse, type CourseSpec, type ModuleSpec } from '@pupitre/core/lib/api';

// Editing a course definition (docs/specs/042-admin-courses.md). The server
// replaces the definition whole, so every change reads it again, checks that
// nobody changed it since the editor opened it, and sends it back whole.

export class DefinitionChangedError extends Error {
  constructor() {
    super("La définition du cours a été modifiée depuis l'ouverture de l'éditeur. Rechargez-la avant d'enregistrer.");
    this.name = 'DefinitionChangedError';
  }
}

/** Key order does not matter: two specs are the same when their sorted JSON is. */
function canonical(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.keys(value as Record<string, unknown>)
        .sort()
        .map((key) => [key, canonical((value as Record<string, unknown>)[key])]),
    );
  }
  return value;
}

export function sameSpec(a: CourseSpec, b: CourseSpec): boolean {
  return JSON.stringify(canonical(a)) === JSON.stringify(canonical(b));
}

/**
 * Reads the definition again, refuses if it differs from `opened`, then sends
 * `change(fresh)`. Returns the spec written.
 */
export async function saveDefinition(slug: string, opened: CourseSpec, change: (spec: CourseSpec) => CourseSpec): Promise<CourseSpec> {
  const fresh = await courseDefinition(slug);
  if (!sameSpec(fresh.spec, opened)) throw new DefinitionChangedError();
  const next = change(fresh.spec);
  await updateCourse(slug, next);
  return next;
}

export type CourseMeta = {
  title: string;
  description: string;
  category: string;
  difficulty: string;
  public: boolean;
  hidden: boolean;
  inPerson: boolean;
  xpRequired: number;
  badgeName: string;
  badgeIcon: string;
};

export function metaOf(spec: CourseSpec): CourseMeta {
  return {
    title: spec.title ?? '',
    description: spec.description ?? '',
    category: spec.category ?? '',
    difficulty: spec.difficulty || 'beginner',
    public: Boolean(spec.public),
    hidden: Boolean(spec.hidden),
    inPerson: Boolean(spec.inPerson),
    xpRequired: spec.xpRequired ?? 0,
    badgeName: spec.badge?.name ?? '',
    badgeIcon: spec.badge?.icon ?? '',
  };
}

/** The metadata changed, everything else (modules, sessions…) kept as read. */
export function withMeta(spec: CourseSpec, meta: CourseMeta): CourseSpec {
  const next: CourseSpec = {
    ...spec,
    title: meta.title.trim(),
    description: meta.description.trim() || undefined,
    category: meta.category.trim() || undefined,
    difficulty: meta.difficulty || undefined,
    public: meta.public || undefined,
    hidden: meta.hidden || undefined,
    inPerson: meta.inPerson || undefined,
    xpRequired: meta.xpRequired > 0 ? meta.xpRequired : undefined,
    badge: meta.badgeName.trim() ? { name: meta.badgeName.trim(), icon: meta.badgeIcon.trim() || undefined } : undefined,
  };
  return Object.fromEntries(Object.entries(next).filter(([, value]) => value !== undefined)) as CourseSpec;
}

/** Types whose body is markdown stored on the module (Astro modTakesMarkdown). */
export const takesMarkdown = (type: string) => type === 'text' || type === 'lab';

/**
 * A module from the form: git wins on the server when both are set, so a
 * filled git source clears the markdown and the other way round.
 */
export function cleanModule(module: ModuleSpec): ModuleSpec {
  const next: ModuleSpec = { ...module, name: module.name.trim() };
  const hasGit = Boolean(next.src?.trim());
  if (hasGit) {
    delete next.content;
    next.src = next.src!.trim();
    next.ref = next.ref?.trim() || 'main';
    if (next.path !== undefined) next.path = next.path.trim();
  } else {
    delete next.src;
    delete next.ref;
    delete next.path;
    if (!next.content) delete next.content;
  }
  if (!next.hidden) delete next.hidden;
  if (next.type !== 'quiz' || !next.inline) delete next.inline;
  return next;
}

export function setModule(spec: CourseSpec, index: number | null, module: ModuleSpec): CourseSpec {
  const modules = [...(spec.modules ?? [])];
  if (index === null) modules.push(module);
  else modules[index] = module;
  return { ...spec, modules };
}

export function removeModule(spec: CourseSpec, index: number): CourseSpec {
  return { ...spec, modules: (spec.modules ?? []).filter((_, i) => i !== index) };
}

// ── Read-only YAML view ─────────────────────────────────────────────────────

const PLAIN = /^[A-Za-z0-9 _./@:()+-]*$/;

function scalar(value: unknown): string {
  if (typeof value === 'string') return value !== '' && PLAIN.test(value) && !/^(true|false|null|-?\d)/.test(value) && !value.includes(': ') ? value : JSON.stringify(value);
  return JSON.stringify(value);
}

/** A small YAML writer for the « Définition » dialog: objects, arrays, multi-line strings. */
export function toYaml(value: unknown, indent = 0): string {
  const pad = ' '.repeat(indent);
  if (Array.isArray(value)) {
    if (value.length === 0) return `${pad}[]`;
    return value
      .map((item) => {
        if (item && typeof item === 'object') {
          const body = toYaml(item, indent + 2).trimStart();
          return `${pad}- ${body}`;
        }
        return `${pad}- ${scalar(item)}`;
      })
      .join('\n');
  }
  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>).filter(([, v]) => v !== undefined);
    if (entries.length === 0) return `${pad}{}`;
    return entries
      .map(([key, v]) => {
        if (typeof v === 'string' && v.includes('\n')) {
          const lines = v.replace(/\n$/, '').split('\n').map((line) => (line ? `${pad}  ${line}` : ''));
          return `${pad}${key}: |\n${lines.join('\n')}`;
        }
        if (v && typeof v === 'object' && (Array.isArray(v) ? v.length > 0 : Object.keys(v).length > 0)) {
          return `${pad}${key}:\n${toYaml(v, indent + 2)}`;
        }
        return `${pad}${key}: ${Array.isArray(v) ? '[]' : v && typeof v === 'object' ? '{}' : scalar(v)}`;
      })
      .join('\n');
  }
  return `${pad}${scalar(value)}`;
}
