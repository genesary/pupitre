import { ApiError } from '@pupitre/core/lib/api-client';

// The message shown when a lab check fails (docs/specs/014-labs.md). Over
// the lab check limit, the API answers 429 "Too many lab checks, retry in N s"
// (docs/specs/190-backend-defects.md B9): the learner reads it in French.

const RETRY_IN = /retry in (\d+) s/;

export function checkErrorMessage(error: Error | null | undefined): string | undefined {
  if (!error) return undefined;
  if (error instanceof ApiError && error.status === 429) {
    const seconds = RETRY_IN.exec(error.message)?.[1];
    return seconds
      ? `Trop de vérifications d'affilée : réessayez dans ${seconds} s.`
      : "Trop de vérifications d'affilée : réessayez dans un instant.";
  }
  return error.message;
}
