import { createContext, useContext } from 'react';
import { useNavigate, type NavigateOptions } from 'react-router';

// The frontend is two applications on one origin (docs/specs/000-scope-and-stack.md):
// the learner app, and the admin app served under /admin and /manager. Each
// one routes its own paths; a path of the other one needs a page load. The
// session lives in sessionStorage, shared by both on the same origin.

export type AppName = 'learner' | 'admin';

const ADMIN_PREFIXES = ['/admin', '/manager'];

/** The application that serves a path. */
export function appOf(path: string): AppName {
  const pathname = path.split(/[?#]/)[0] ?? '';
  return ADMIN_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)) ? 'admin' : 'learner';
}

export const SiteContext = createContext<AppName>('learner');

/** The page load towards the other application, in one place so tests can watch it. */
export const pageLoad = {
  go(to: string, replace = false) {
    if (replace) window.location.replace(to);
    else window.location.assign(to);
  },
};

export function useCurrentApp(): AppName {
  return useContext(SiteContext);
}

/** navigate() inside the current app, a page load towards the other one. */
export function useSiteNavigate() {
  const navigate = useNavigate();
  const app = useCurrentApp();
  return (to: string, options?: NavigateOptions) => {
    if (appOf(to) === app) void navigate(to, options);
    else pageLoad.go(to, options?.replace);
  };
}
