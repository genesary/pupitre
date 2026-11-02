import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { adminCourses, adminLeaderboard } from '@pupitre/core/lib/api';
import { AdminStatCards } from '@/components/admin/AdminStatCards';
import { ScoreLeaderboardRows } from '@/components/admin/ScoreLeaderboard';

// /admin/monitoring (docs/specs/040-admin-dashboard.md), markup of pupitre's
// admin/monitoring/index.astro. Its « Recalculer la progression » button is not
// carried over: the route it calls marks one lesson for one user (defect A23).

export function Monitoring() {
  const courses = useQuery({ queryKey: ['admin', 'courses'], queryFn: adminCourses });
  const board = useQuery({ queryKey: ['admin', 'leaderboard'], queryFn: adminLeaderboard });

  const list = [...(courses.data?.courses ?? [])].sort((a, b) => (b.enrollmentCount ?? 0) - (a.enrollmentCount ?? 0));
  const top = (board.data?.leaderboard ?? []).slice(0, 10);

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Monitoring</h1>
          <p className="text-soft text-sm mt-1">Statistiques et activité de la plateforme</p>
        </div>
        <Link to="/admin/leaderboard" className="btn btn-secondary btn-sm">
          🏆 Classement complet
        </Link>
      </div>

      <AdminStatCards className="mb-7" />

      <div className="card !p-0 overflow-x-auto mb-7">
        <div className="px-5 py-[18px] border-b border-line">
          <h2 className="text-[1.1rem]">Aperçu des cours</h2>
        </div>
        {courses.isPending ? (
          <div className="loading-state">
            <div className="spinner" />
            <span className="text-soft text-sm">Chargement…</span>
          </div>
        ) : courses.isError ? (
          <p className="text-bad p-5">{courses.error.message}</p>
        ) : list.length === 0 ? (
          <p className="text-soft text-sm p-5">Aucun cours.</p>
        ) : (
          <table className="admin-table">
            <thead>
              <tr className="admin-tr-head">
                <th className="admin-th-sm">Titre</th>
                <th className="admin-th-sm">Catégorie</th>
                <th className="admin-th-center">Modules</th>
                <th className="admin-th-center">Inscrits</th>
                <th className="admin-th-center">Accès</th>
              </tr>
            </thead>
            <tbody>
              {list.map((course, index) => (
                <tr key={course.slug} className={index < list.length - 1 ? 'admin-tr' : undefined}>
                  <td className="admin-td">
                    <div className="username-text">{course.title}</div>
                    <div className="mono-id">{course.id}</div>
                  </td>
                  <td className="admin-td text-soft">{course.category || '—'}</td>
                  <td className="admin-td-center">{course.moduleCount ?? '—'}</td>
                  <td className="admin-td-center">{course.enrollmentCount ?? 0}</td>
                  <td className="admin-td-center">
                    <span className={`badge ${course.isPublic ? 'badge-green' : 'badge-gray'} badge-xs`}>
                      {course.isPublic ? 'Public' : 'Privé'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="card !p-0 overflow-hidden">
        <div className="flex items-center justify-between px-5 py-[18px] border-b border-line">
          <h2 className="text-[1.1rem]">Top 10 — Classement</h2>
          <Link to="/admin/leaderboard" className="text-blue text-sm">
            Voir tout →
          </Link>
        </div>
        {board.isPending ? (
          <div className="loading-state">
            <div className="spinner" />
            <span className="text-soft text-sm">Chargement…</span>
          </div>
        ) : board.isError ? (
          <p className="text-bad p-6">{board.error.message}</p>
        ) : top.length === 0 ? (
          <p className="text-soft text-sm p-6">Aucune donnée.</p>
        ) : (
          <ScoreLeaderboardRows entries={top} />
        )}
      </div>
    </>
  );
}
