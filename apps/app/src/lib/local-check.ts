import type { CheckResult } from '@pupitre/core/lib/api-types';

// Local lab checks of the pupitre-desktop app (docs/specs/014-labs.md). Same
// behaviour and signature as pupitre's frontend/src/lib/local-check.ts, so the
// desktop app keeps working with the new frontend.

export type LocalCheckMeta = {
  checkProvider?: 'local' | 'gitlab';
  checkType?: string;
  checkParams?: Record<string, unknown>;
};

export function isTauri(): boolean {
  return typeof window !== 'undefined' && ('__TAURI__' in window || '__TAURI_INTERNALS__' in window);
}

async function invokeLocalCheck(checkType: string, params: Record<string, unknown>): Promise<CheckResult> {
  // Dynamic import: kept out of the web bundle's main chunk.
  const { invoke } = await import('@tauri-apps/api/core');
  return invoke<CheckResult>('local_check', { checkType, params });
}

/**
 * - provider `local` inside Tauri → the Rust `local_check` command, then the
 *   result is recorded (the server keeps it as not verified, pupitre#86);
 * - otherwise → the course-service check.
 */
export async function resolveCheck(
  meta: LocalCheckMeta,
  remoteCheck: () => Promise<CheckResult>,
  record?: (result: CheckResult) => Promise<unknown>,
): Promise<CheckResult> {
  if (meta.checkProvider === 'local' && isTauri()) {
    if (!meta.checkType) return { allow: false, violations: ['checkType manquant dans le module'] };
    const result = await invokeLocalCheck(meta.checkType, meta.checkParams ?? {});
    record?.(result).catch(() => {});
    return result;
  }
  return remoteCheck();
}
