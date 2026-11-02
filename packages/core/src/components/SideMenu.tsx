import { Dialog } from 'radix-ui';

import { signOut } from '../lib/sign-out';
import { useSession } from '../lib/session';
import { CloseIcon } from '@pupitre/ui/components/icons';
import { toggleTheme, useTheme } from '@pupitre/ui/lib/theme';

import { SIDE_MENU } from '../lib/navigation';
import { SiteNavLink } from './SiteLink';

// The side drawer of layouts/App.astro, same markup and classes. Radix Dialog
// brings focus trapping, Escape and the backdrop click; the look stays pupitre's.

type SideMenuProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Shows « Visite guidée » (learner app, docs/specs/007-guided-tour.md). */
  onStartTour?: () => void;
};

export function SideMenu({ open, onOpenChange, onStartTour }: SideMenuProps) {
  const session = useSession();
  const theme = useTheme();
  const close = () => onOpenChange(false);

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="nav-drawer-overlay open" />
        <Dialog.Content className="nav-drawer open" aria-describedby={undefined}>
          <div className="nav-drawer-header">
            <Dialog.Title className="brand-font text-2xl font-bold brand-logo-text">pupitre</Dialog.Title>
            <Dialog.Close className="nav-drawer-close" aria-label="Fermer">
              <CloseIcon />
            </Dialog.Close>
          </div>
          <div className="nav-drawer-body">
            {SIDE_MENU.map((section) => (
              <div key={section.label} className="nav-drawer-section">
                <div className="nav-drawer-label">{section.label}</div>
                {section.items.map((item) => (
                  <SiteNavLink
                    key={item.to}
                    to={item.to}
                    onClick={close}
                    className={({ isActive }) => `nav-drawer-link${isActive ? ' active' : ''}`}
                  >
                    <span className="nav-drawer-icon">{item.icon}</span>
                    {item.label}
                  </SiteNavLink>
                ))}
              </div>
            ))}
            <div className="nav-drawer-section">
              {session && onStartTour && (
                <button
                  type="button"
                  className="nav-drawer-link w-full text-left"
                  onClick={() => {
                    close();
                    onStartTour();
                  }}
                >
                  <span className="nav-drawer-icon">🐤</span>
                  Visite guidée
                </button>
              )}
              <button type="button" className="nav-drawer-link w-full text-left" onClick={toggleTheme}>
                <span className="nav-drawer-icon">{theme === 'dark' ? '☀️' : '🌙'}</span>
                <span>{theme === 'dark' ? 'Mode clair' : 'Mode sombre'}</span>
              </button>
              {session && (
                <button
                  type="button"
                  className="nav-drawer-link w-full text-left text-bad"
                  onClick={() => {
                    close();
                    signOut();
                  }}
                >
                  <span className="nav-drawer-icon">🚪</span>
                  Déconnexion
                </button>
              )}
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
