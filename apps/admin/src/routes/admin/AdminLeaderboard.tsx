import { useQuery } from '@tanstack/react-query';

import { adminLeaderboard } from '@pupitre/core/lib/api';
import { ScoreLeaderboardRows } from '@/components/admin/ScoreLeaderboard';

// /admin/leaderboard (docs/specs/040-admin-dashboard.md), markup of pupitre's
// admin/leaderboard/index.astro, without its « Recalculer » button (defect A23).

export function AdminLeaderboard() {
  const board = useQuery({ queryKey: ['admin', 'leaderboard'], queryFn: adminLeaderboard });
  const entries = board.data?.leaderboard ?? [];

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Classement</h1>
          <p className="text-soft text-sm mt-1">Progression globale des apprenants sur la plateforme</p>
        </div>
      </div>
      <div className="card !p-0 overflow-hidden">
        <div className="flex items-center justify-between px-5 py-[18px] border-b border-line">
          <h2 className="text-[1.1rem]">Classement général</h2>
          {board.isSuccess && <span className="badge badge-gray">{entries.length} participants</span>}
        </div>
        {board.isPending ? (
          <div className="loading-state">
            <div className="spinner" />
            <span className="text-soft text-sm">Chargement…</span>
          </div>
        ) : board.isError ? (
          <p className="text-bad p-6">{board.error.message}</p>
        ) : entries.length === 0 ? (
          <p className="text-soft text-sm p-6">Aucune donnée de classement.</p>
        ) : (
          <ScoreLeaderboardRows entries={entries} showEmail />
        )}
      </div>
    </>
  );
}
