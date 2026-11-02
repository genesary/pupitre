// Web storage can be missing or throw (private mode, blocked site data, some
// embedded webviews). Every read and write goes through these helpers so a
// storage failure degrades to "nothing stored" instead of breaking the page.

type Area = 'local' | 'session';

function area(kind: Area): Storage | null {
  try {
    return kind === 'local' ? window.localStorage : window.sessionStorage;
  } catch {
    return null;
  }
}

export function readItem(kind: Area, key: string): string | null {
  try {
    return area(kind)?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

export function writeItem(kind: Area, key: string, value: string): void {
  try {
    area(kind)?.setItem(key, value);
  } catch {
    // Storage full or blocked: the value is simply not kept.
  }
}

export function removeItem(kind: Area, key: string): void {
  try {
    area(kind)?.removeItem(key);
  } catch {
    // Nothing to remove.
  }
}
