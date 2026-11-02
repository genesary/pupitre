import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { adminLeaderboard } from '@pupitre/core/lib/api';
import { ADMIN_MENU } from '@pupitre/core/lib/navigation';
import { AdminIcon } from '@pupitre/ui/components/icons';
import { UserAvatar } from '@pupitre/ui/components/user-avatar';
import { AdminStatCards } from '@/components/admin/AdminStatCards';

// /admin (docs/specs/040-admin-dashboard.md), markup of pupitre's admin/index.astro.

export function Dashboard() {
  const board = useQuery({ queryKey: ['admin', 'leaderboard'], queryFn: adminLeaderboard });
  const top = (board.data?.leaderboard ?? []).slice(0, 5);
  const shortcuts = ADMIN_MENU.filter((item) => item.to !== '/admin');

  return (
    <div>
      <div className="mb-7">
        <h1 className="text-2xl">Tableau de bord</h1>
        <p className="text-soft text-sm mt-1">Vue d'ensemble de la plateforme pupitre</p>
      </div>

      <AdminStatCards />

      <div className="card mb-7">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-[1.1rem]">Classement</h2>
          <Link to="/admin/leaderboard" className="btn btn-secondary btn-sm">
            Voir tout →
          </Link>
        </div>
        {board.isPending ? (
          <div className="loading-state">
            <div className="spinner" />
          </div>
        ) : board.isError ? (
          <p className="text-soft text-sm">Classement indisponible.</p>
        ) : top.length === 0 ? (
          <p className="text-soft text-sm">Aucune donnée.</p>
        ) : (
          top.map((entry, index) => (
            <div key={entry.id} className="lb-row">
              <span className={`lb-rank${index < 3 ? ' top' : ''}`}>{index + 1}</span>
              <div className="lb-avatar">
                <UserAvatar
                  username={entry.username}
                  avatarUrl={entry.avatarUrl}
                  imageClassName="w-full h-full object-cover rounded-full"
                  fallbackClassName=""
                  emojiClassName=""
                />
              </div>
              <div className="lb-info">
                <div className="lb-name">{entry.username}</div>
                <div className="lb-sub">
                  {entry.passedModules} modules · {entry.enrolledCourses} cours
                </div>
              </div>
              <span className="lb-score">{entry.totalScore} pts</span>
            </div>
          ))
        )}
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        {shortcuts.map((item) => {
          const Icon = item.icon;
          return (
            <Link
              key={item.to}
              to={item.to}
              className="card-sm flex flex-col items-center gap-2 p-4 text-center no-underline text-inherit hover:border-blue"
            >
              <span className="sidebar-icon" style={{ color: item.color }}>
                {typeof Icon === 'string' ? <AdminIcon name={Icon} /> : <Icon strokeWidth={1.5} />}
              </span>
              <span className="text-sm font-medium">{item.label}</span>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
