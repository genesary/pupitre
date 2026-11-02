import { useQuery } from '@tanstack/react-query';

import { adminStats } from '@pupitre/core/lib/api';
import type { AdminStats } from '@pupitre/core/lib/api-types';

// The statistic cards of the admin dashboard and monitoring pages
// (docs/specs/040-admin-dashboard.md). total_labs, total_submissions and
// success_rate are always 0 on the server (defect A3): they are not shown.

const CARDS: { key: keyof AdminStats; label: string; className: string }[] = [
  { key: 'total_users', label: 'Utilisateurs', className: 'text-blue' },
  { key: 'totalCourses', label: 'Cours', className: 'text-blue' },
  { key: 'total_enrollments', label: 'Inscriptions', className: 'text-orange' },
];

export function AdminStatCards({ className = '' }: { className?: string }) {
  const stats = useQuery({ queryKey: ['admin', 'stats'], queryFn: adminStats });

  if (stats.isError) {
    return (
      <div className={`stats-grid ${className}`}>
        <div className="text-soft text-sm col-span-full">Statistiques indisponibles</div>
      </div>
    );
  }

  return (
    <div className={`stats-grid ${className}`}>
      {CARDS.map(({ key, label, className: colour }) =>
        stats.isPending ? (
          <div key={key} className="card-sm text-center p-5">
            <div className="spinner mx-auto mb-2" />
            <div className="text-soft text-xs">Chargement…</div>
          </div>
        ) : (
          <div key={key} className="card-sm stat-card">
            <div className={`stat-card-value ${colour}`}>{stats.data[key] ?? '—'}</div>
            <div className="stat-card-label">{label}</div>
          </div>
        ),
      )}
    </div>
  );
}
