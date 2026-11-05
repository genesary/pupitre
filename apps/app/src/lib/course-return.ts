import { safeRedirect } from '@pupitre/ui/lib/security';
import { readItem, removeItem, writeItem } from '@pupitre/ui/lib/storage';

// Where the end of a course leads back to, set by a learning path page.
// Kept in
// sessionStorage and cleared once used.

const RETURN_KEY = 'pupitre.courseReturnTo';

export function setCourseReturn(path: string) {
  writeItem('session', RETURN_KEY, path);
}

/** Reads and clears the stored path; only same-site paths are returned. */
export function takeCourseReturn(): string | null {
  const path = readItem('session', RETURN_KEY);
  removeItem('session', RETURN_KEY);
  return safeRedirect(path, '') || null;
}
