import { HttpResponse } from 'msw';

import { findAccount } from './data';
import { readMockToken } from './jwt';

/** The { "error": "…" } envelope of internal/httperr. */
export function apiError(status: number, message: string) {
  return HttpResponse.json({ error: message }, { status });
}

/** The account behind the request's bearer token, or null (missing, invalid, expired). */
export function currentAccount(request: Request) {
  const claims = readMockToken(request.headers.get('Authorization'));
  if (!claims || claims.exp * 1000 <= Date.now()) return null;
  return findAccount(claims.sub) ?? null;
}
