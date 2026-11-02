import { Activity, Puzzle, Settings, type LucideIcon } from 'lucide-react';

import type { AdminIconName } from '@pupitre/ui/components/icons';

// Navigation entries of docs/specs/005-ui-theme-navigation.md, with the labels,
// emojis, icons and colours of the pupitre Astro frontend.

export type NavItem = { to: string; label: string; icon: string };
export type NavSection = { label: string; items: NavItem[] };

export const SIDE_MENU: NavSection[] = [
  {
    label: 'Mon apprentissage',
    items: [
      { to: '/courses', label: 'Catalogue', icon: '📚' },
      { to: '/my-courses', label: 'Mes cours', icon: '🎓' },
      { to: '/my-paths', label: 'Mes parcours', icon: '🗺️' },
      { to: '/my-groups', label: 'Mes groupes', icon: '👥' },
    ],
  },
  {
    label: 'Progression & Gamification',
    items: [
      { to: '/my-badges', label: 'Mes badges', icon: '🏅' },
      { to: '/my-xp', label: 'Mon XP', icon: '⚡' },
      { to: '/leaderboard', label: 'Classement', icon: '🏆' },
    ],
  },
  {
    label: 'Compétences & Planning',
    items: [
      { to: '/my-skills', label: 'Mes compétences', icon: '🎯' },
      { to: '/agenda', label: 'Agenda', icon: '📅' },
    ],
  },
];

export type AdminNavItem = {
  to: string;
  label: string;
  color: string;
  icon: AdminIconName | LucideIcon;
  managerToo?: boolean;
};

/**
 * One list for every admin page (the Astro frontend had two, defect A4). The
 * first eight entries are those of layouts/Admin.astro; Patterns, Paramètres and
 * Monitoring come from the old dashboard and get matching icons.
 */
export const ADMIN_MENU: AdminNavItem[] = [
  { to: '/admin', label: 'Tableau de bord', color: '#3b82f6', icon: 'dashboard' },
  { to: '/admin/users', label: 'Utilisateurs', color: '#8b5cf6', icon: 'users' },
  { to: '/admin/courses', label: 'Cours', color: '#10b981', icon: 'courses' },
  { to: '/admin/paths', label: 'Parcours', color: '#06b6d4', icon: 'paths' },
  { to: '/admin/groups', label: 'Groupes', color: '#f97316', icon: 'groups' },
  { to: '/admin/leaderboard', label: 'Classement', color: '#f59e0b', icon: 'leaderboard' },
  { to: '/admin/labs', label: 'Labs', color: '#6366f1', icon: 'labs' },
  { to: '/admin/exports', label: 'Exports CSV', color: '#64748b', icon: 'exports', managerToo: true },
  { to: '/admin/patterns', label: 'Patterns', color: '#d946ef', icon: Puzzle },
  { to: '/admin/settings', label: 'Paramètres', color: '#475569', icon: Settings },
  { to: '/admin/monitoring', label: 'Monitoring', color: '#14b8a6', icon: Activity },
];

/** layouts/Manager.astro */
export const MANAGER_MENU = [
  { to: '/manager/users', label: '👥 Mes membres' },
  { to: '/manager/groups', label: '🏷️ Mes groupes' },
  { to: '/manager/paths', label: '🗺️ Parcours' },
];
