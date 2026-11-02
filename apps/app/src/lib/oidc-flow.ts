import { readItem, removeItem, writeItem } from '@pupitre/ui/lib/storage';

// Values kept across the round trip to Keycloak (docs/specs/001). The state is
// compared on return to refuse a callback this browser did not start (login CSRF,
// audit finding M3).

const STATE_KEY = 'pupitre.oidc.state';
const REDIRECT_KEY = 'pupitre.oidc.redirect';

export function rememberFlow(state: string, redirect: string | null) {
  writeItem('session', STATE_KEY, state);
  if (redirect) writeItem('session', REDIRECT_KEY, redirect);
  else removeItem('session', REDIRECT_KEY);
}

export function recallFlow(): { state: string | null; redirect: string | null } {
  return { state: readItem('session', STATE_KEY), redirect: readItem('session', REDIRECT_KEY) };
}

export function clearFlow() {
  removeItem('session', STATE_KEY);
  removeItem('session', REDIRECT_KEY);
}
