// Slug rule of docs/specs/042-admin-courses.md (course-service admin_definition.go
// accepts ^[a-z0-9][a-z0-9-]*$; the spec is stricter: no leading, trailing or
// double hyphen), and the reserved path segments of the admin routes.

const SLUG = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const RESERVED = new Set(['batch', 'import', 'paths', 'sessions', 'cache']);

/** The error to show below the slug field, or null when the slug is valid. */
export function slugError(slug: string): string | null {
  if (!slug) return 'Le slug est requis.';
  if (!SLUG.test(slug)) return 'Lettres minuscules, chiffres et tirets uniquement (ex : mon-cours).';
  if (RESERVED.has(slug)) return `« ${slug} » est réservé.`;
  return null;
}
