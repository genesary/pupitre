import { useState } from 'react';

import { signOut } from '../lib/sign-out';
import { useSession } from '../lib/session';
import { Brand } from '@pupitre/ui/components/brand';
import { HamburgerIcon, MoonIcon, SunIcon } from '@pupitre/ui/components/icons';
import { UserAvatar } from '@pupitre/ui/components/user-avatar';
import { toggleTheme, useTheme } from '@pupitre/ui/lib/theme';

import { SideMenu } from './SideMenu';
import { SiteLink, SiteNavLink } from './SiteLink';

// The top bar of layouts/App.astro, same markup and classes, in both apps: its
// links to the other app are page loads (SiteLink).

export function TopBar({ onStartTour }: { onStartTour?: () => void }) {
  const session = useSession();
  const theme = useTheme();
  const [menuOpen, setMenuOpen] = useState(false);
  const user = session?.user;

  return (
    <>
      <nav className="topnav">
        <button className="nav-hamburger" aria-label="Menu" data-tour="nav-menu" onClick={() => setMenuOpen(true)}>
          <HamburgerIcon />
        </button>
        <SiteLink to="/courses" className="brand">
          <Brand />
        </SiteLink>
        <SiteNavLink to="/courses" end data-tour="nav-catalogue" className={({ isActive }) => `navlink${isActive ? ' active' : ''}`}>
          Catalogue
        </SiteNavLink>
        <SiteNavLink to="/my-courses" data-tour="nav-my-courses" className={({ isActive }) => `navlink${isActive ? ' active' : ''}`}>
          Mes cours
        </SiteNavLink>
        {user?.role === 'manager' && (
          <SiteNavLink
            to="/manager/users"
            id="nav-manager"
            data-tour="nav-manager"
            className={({ isActive }) => `navlink${isActive ? ' active' : ''}`}
          >
            Mon espace
          </SiteNavLink>
        )}
        <div className="nav-spacer" />
        <button
          className="nav-hamburger"
          aria-label="Basculer le thème"
          title="Mode sombre / clair"
          data-tour="nav-theme"
          onClick={toggleTheme}
        >
          {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
        </button>
        <div className="flex items-center gap-2">
          {user ? (
            <>
              <SiteLink to="/profile" data-tour="nav-profile" className="flex items-center gap-2 no-underline text-inherit" title={user.username}>
                <UserAvatar username={user.username} avatarUrl={user.avatarUrl} />
                <span className="text-[.875rem] font-medium text-ink-soft max-w-[120px] overflow-hidden text-ellipsis whitespace-nowrap hidden sm:inline">
                  {user.username}
                </span>
              </SiteLink>
              {user.role === 'admin' && (
                <SiteLink to="/admin" data-tour="nav-admin" className="btn btn-sm btn-admin-nav">
                  ⚙ Admin
                </SiteLink>
              )}
              <button id="nav-logout" className="btn btn-ghost btn-sm" onClick={() => signOut()}>
                Déconnexion
              </button>
            </>
          ) : (
            <SiteLink to="/login" className="btn btn-secondary btn-sm">
              Se connecter
            </SiteLink>
          )}
        </div>
      </nav>
      <SideMenu open={menuOpen} onOpenChange={setMenuOpen} onStartTour={onStartTour} />
    </>
  );
}
