import type { UserPublic } from './api-types';

/** Default destination after sign-in (docs/specs/001). */
export function landingPath(user: Pick<UserPublic, 'role'>): string {
  return user.role === 'admin' ? '/admin' : '/courses';
}

/** /login, remembering the page to come back to. */
export function loginPath(location: { pathname: string; search: string; hash?: string }): string {
  const target = `${location.pathname}${location.search}${location.hash ?? ''}`;
  return `/login?redirect=${encodeURIComponent(target)}`;
}
