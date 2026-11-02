import { logout } from './api';
import { endSession } from './session';

/**
 * The « Déconnexion » button (docs/specs/001-keycloak-authentication.md):
 * ends the session on the server, then in the browser. The request reads the
 * token as it starts, before the session is cleared; its outcome does not
 * matter, the browser forgets the token either way.
 */
export function signOut() {
  void logout().catch(() => undefined);
  endSession('logout');
}
