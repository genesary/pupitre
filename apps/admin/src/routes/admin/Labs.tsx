import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { labChecks } from '@pupitre/core/lib/api';

// /admin/labs (docs/specs/047-admin-exports-and-labs.md), markup of pupitre's
// admin/labs/index.astro, plus the verification column of the spec.

const dateTime = (iso: string) =>
  new Date(iso).toLocaleString('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });

const plural = (count: number, word: string) => `${count} ${word}${count > 1 ? 's' : ''}`;

export function Labs() {
  const [course, setCourse] = useState('');
  const all = useQuery({ queryKey: ['admin', 'lab-checks', ''], queryFn: () => labChecks() });
  const checks = useQuery({ queryKey: ['admin', 'lab-checks', course], queryFn: () => labChecks(course) });

  const courses = [...new Set((all.data ?? []).map((r) => r.courseSlug))].sort();
  const rows = [...(checks.data ?? [])].sort((a, b) => b.checkedAt.localeCompare(a.checkedAt));
  const passed = rows.filter((r) => r.allow).length;
  const learners = new Set(rows.map((r) => r.username)).size;

  const stats = [
    { label: 'Total vérifications', value: rows.length, className: 'text-blue' },
    { label: 'Labs validés', value: passed, className: 'text-good' },
    { label: 'Taux de réussite', value: rows.length ? `${Math.round((passed / rows.length) * 100)}%` : '—', className: 'text-ink' },
    { label: 'Apprenants actifs', value: learners, className: 'text-ink' },
  ];

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="text-2xl">Suivi des Labs</h1>
          <p className="text-soft text-sm mt-1">
            {checks.isSuccess
              ? `${plural(rows.length, 'vérification')} — ${plural(learners, 'apprenant')}`
              : 'Chargement…'}
          </p>
        </div>
        <div className="flex gap-2 items-center flex-wrap">
          <select
            className="input select-sm min-w-[200px]"
            aria-label="Cours"
            value={course}
            onChange={(event) => setCourse(event.target.value)}
          >
            <option value="">Tous les cours</option>
            {courses.map((slug) => (
              <option key={slug} value={slug}>
                {slug}
              </option>
            ))}
          </select>
          <button className="btn btn-secondary" disabled={checks.isFetching} onClick={() => void checks.refetch()}>
            Actualiser
          </button>
        </div>
      </div>

      {checks.isSuccess && (
        <div className="auto-grid-160">
          {stats.map((stat) => (
            <div key={stat.label} className="card stat-card">
              <div className={`stat-card-value ${stat.className}`}>{stat.value}</div>
              <div className="stat-card-label">{stat.label}</div>
            </div>
          ))}
        </div>
      )}

      <div className="card !p-0 overflow-x-auto">
        <table className="admin-table">
          <thead>
            <tr className="admin-tr-head">
              <th className="admin-th">Apprenant</th>
              <th className="admin-th">Cours</th>
              <th className="admin-th">Lab</th>
              <th className="admin-th">Statut</th>
              <th className="admin-th">Violations</th>
              <th className="admin-th">Vérification</th>
              <th className="admin-th">Vérifié le</th>
            </tr>
          </thead>
          <tbody>
            {checks.isPending ? (
              <tr>
                <td colSpan={7} className="p-8 text-center">
                  <div className="loading-state !p-0">
                    <div className="spinner" />
                  </div>
                </td>
              </tr>
            ) : checks.isError ? (
              <tr>
                <td colSpan={7} className="p-8 text-center text-bad">
                  Erreur : {checks.error.message}
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="p-10 text-center text-soft">
                  Aucune vérification enregistrée.
                </td>
              </tr>
            ) : (
              rows.map((row) => {
                const violations = row.violations ?? [];
                return (
                  <tr key={row.id} className="admin-tr">
                    <td className="admin-td">
                      <div className="flex-row-gap2">
                        <div className="user-avatar-sm">{(row.username[0] ?? '?').toUpperCase()}</div>
                        <span className="font-medium">{row.username}</span>
                      </div>
                    </td>
                    <td className="admin-td text-soft text-sm">{row.courseSlug}</td>
                    <td className="admin-td text-sm">{row.moduleName}</td>
                    <td className="admin-td">
                      {row.allow ? <span className="labs-status-ok">✓ Validé</span> : <span className="labs-status-fail">✗ Échec</span>}
                    </td>
                    <td className="admin-td max-w-[340px]">
                      {violations.length === 0 ? (
                        <span className="text-soft text-sm">—</span>
                      ) : violations.length === 1 ? (
                        <div className="violation-item">• {violations[0]}</div>
                      ) : (
                        <details>
                          <summary className="cursor-pointer text-sm">{violations.length} violations</summary>
                          {violations.map((violation, index) => (
                            <div key={index} className="violation-item">
                              • {violation}
                            </div>
                          ))}
                        </details>
                      )}
                    </td>
                    <td className="admin-td">
                      <span className={`badge ${row.verified ? 'badge-green' : 'badge-gray'}`}>
                        {row.verified ? 'Serveur' : 'Auto-déclaré'}
                      </span>
                    </td>
                    <td className="admin-td text-soft text-sm whitespace-nowrap">{dateTime(row.checkedAt)}</td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
