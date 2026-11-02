import { useEffect, type ComponentProps, type ReactNode } from 'react';
import { Link, Navigate, NavLink } from 'react-router';

import { appOf, pageLoad, useCurrentApp } from '../lib/site';

// Links and redirects that cross from one application to the other
// (lib/site.ts): a router link inside the current app, a plain <a> otherwise.

type SiteLinkProps = Omit<ComponentProps<'a'>, 'href'> & { to: string };

export function SiteLink({ to, ...props }: SiteLinkProps) {
  const app = useCurrentApp();
  if (appOf(to) === app) return <Link to={to} {...props} />;
  return <a href={to} {...props} />;
}

type SiteNavLinkProps = Omit<ComponentProps<'a'>, 'href' | 'className'> & {
  to: string;
  end?: boolean;
  className: (state: { isActive: boolean }) => string;
  children?: ReactNode;
};

/** A NavLink in the current app; a link to the other app is never active here. */
export function SiteNavLink({ to, end, className, ...props }: SiteNavLinkProps) {
  const app = useCurrentApp();
  if (appOf(to) === app) return <NavLink to={to} end={end} className={className} {...props} />;
  return <a href={to} className={className({ isActive: false })} {...props} />;
}

function PageLoad({ to }: { to: string }) {
  useEffect(() => {
    pageLoad.go(to, true);
  }, [to]);
  return null;
}

/** <Navigate replace> that also works towards the other application. */
export function SiteNavigate({ to }: { to: string }) {
  const app = useCurrentApp();
  if (appOf(to) === app) return <Navigate to={to} replace />;
  return <PageLoad to={to} />;
}
