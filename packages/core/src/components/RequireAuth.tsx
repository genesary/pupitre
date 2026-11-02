import type { ReactNode } from 'react';
import { Outlet, useLocation } from 'react-router';

import type { Role } from '../lib/api-types';
import { AccessDenied } from './AccessDenied';
import { SiteNavigate } from './SiteLink';

import { loginPath } from '../lib/auth-paths';
import { useSession } from '../lib/session';

// Route guards of docs/specs/002-roles-and-guards.md. They guide the user; the
// backend remains the only authority over data. The sign-in page belongs to the
// learner app: from the admin app, going there is a page load.

export function RequireAuth({ children }: { children?: ReactNode }) {
  const session = useSession();
  const location = useLocation();
  if (!session) return <SiteNavigate to={loginPath(location)} />;
  return children ?? <Outlet />;
}

export function RequireRole({ roles, children }: { roles: Role[]; children?: ReactNode }) {
  const session = useSession();
  const location = useLocation();
  if (!session) return <SiteNavigate to={loginPath(location)} />;
  if (!roles.includes(session.user.role)) return <AccessDenied />;
  return children ?? <Outlet />;
}
