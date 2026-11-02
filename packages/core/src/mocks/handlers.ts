import { http, HttpResponse } from 'msw';

import { readItem, writeItem } from '@pupitre/ui/lib/storage';

import { findAccount } from './data';
import { adminHandlers, publicSettings } from './admin-handlers';
import { apiError, currentAccount } from './http';
import { mockToken } from './jwt';
import { learnerHandlers } from './learner-handlers';
import { spaceHandlers } from './space-handlers';
import { managerHandlers } from './manager-handlers';

// Mock API of `pnpm dev:mock` and of the tests. Each handler mirrors a Go route
// and its response shape; errors use the same { "error": "…" } envelope.

// MSW runs inside the page, and the sign-in flow reloads the page twice
// (to the mock Keycloak page, then back to /auth/callback). Issued states are
// kept in sessionStorage so they survive, as they would on a real server.
const STATES_KEY = 'pupitre.mock.oidc-states';

const issuedStates = {
  read(): string[] {
    try {
      return JSON.parse(readItem('session', STATES_KEY) ?? '[]') as string[];
    } catch {
      return [];
    }
  },
  add(state: string) {
    writeItem('session', STATES_KEY, JSON.stringify([...this.read(), state].slice(-10)));
  },
  /** Removes the state and reports whether it had been issued (single use). */
  delete(state: string): boolean {
    const states = this.read();
    if (!states.includes(state)) return false;
    writeItem('session', STATES_KEY, JSON.stringify(states.filter((s) => s !== state)));
    return true;
  },
};


export const handlers = [
  http.get('/api/settings/public', () => HttpResponse.json(publicSettings())),

  // The real route returns Keycloak's authorization URL; here it points to the
  // mock Keycloak page, which sends the user back to /auth/callback.
  http.get('/api/auth/oidc/authorize', ({ request }) => {
    const state = crypto.randomUUID();
    issuedStates.add(state);
    const url = new URL('/mock-keycloak', new URL(request.url).origin);
    url.searchParams.set('state', state);
    return HttpResponse.json({ url: url.href, state });
  }),

  http.post('/api/auth/oidc/callback', async ({ request }) => {
    const body = (await request.json()) as { code?: string; state?: string };
    if (!body.state || !issuedStates.delete(body.state)) {
      return apiError(401, 'Invalid or expired OIDC state');
    }
    const account = findAccount(body.code);
    if (!account) return apiError(401, 'Token exchange failed');
    return HttpResponse.json({
      token: mockToken(account.user.id, account.user.role),
      user: account.user,
    });
  }),

  // user-service sessions_auth.go: the mock tokens are not revoked, the
  // browser forgets them.
  http.post('/api/auth/logout', ({ request }) => {
    if (!currentAccount(request)) return apiError(401, 'Invalid token');
    return HttpResponse.json({ message: 'Signed out' });
  }),

  http.get('/api/auth/me', ({ request }) => {
    const account = currentAccount(request);
    if (!account) return apiError(401, 'Invalid token');
    return HttpResponse.json(account.user);
  }),

  ...learnerHandlers,
  ...spaceHandlers,
  ...managerHandlers,
  ...adminHandlers,

  // Anything not simulated yet answers clearly instead of reaching the network.
  http.all('/api/*', ({ request }) =>
    apiError(501, `Route non simulée en mode mock : ${request.method} ${new URL(request.url).pathname}`),
  ),
];
