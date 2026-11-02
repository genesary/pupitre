import { useMutation, useQueries, useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { toast } from 'sonner';

import { exportCategories, exportDownload, exportPreview, type ExportSource } from '@pupitre/core/lib/api';
import type { ExportCategory, ExportRequest } from '@pupitre/core/lib/api-types';

// /admin/exports (docs/specs/047-admin-exports-and-labs.md), markup of pupitre's
// admin/exports/index.astro. Two sources: user-service and the lab checks of
// course-service; each category remembers its own. Preview cells are text.

type Category = ExportCategory & { source: ExportSource };

const SOURCES: ExportSource[] = ['/admin/exports', '/admin/exports/lab-checks'];

function useDebounced<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

function FilterControl({
  filter,
  value,
  onChange,
}: {
  filter: ExportCategory['filters'][number];
  value: string;
  onChange: (value: string) => void;
}) {
  const id = `filter-${filter.id}`;
  return (
    <div className="flex flex-col gap-1">
      <label className="text-sm font-medium" htmlFor={id}>
        {filter.label}
      </label>
      {filter.type === 'select' && filter.options ? (
        <select id={id} className="input input-sm" value={value} onChange={(event) => onChange(event.target.value)}>
          <option value="">— Tous —</option>
          {filter.options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      ) : (
        <input
          id={id}
          className="input input-sm"
          type={filter.type === 'date' ? 'date' : 'text'}
          placeholder={filter.type === 'text' ? filter.label : ''}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </div>
  );
}

export function Exports() {
  // A source the user may not read (lab checks are admin-only) is left out.
  const sources = useQueries({
    queries: SOURCES.map((source) => ({
      queryKey: ['exports', source, 'categories'],
      queryFn: () => exportCategories(source),
      retry: false,
    })),
  });
  const categories: Category[] = sources.flatMap((q, i) =>
    (q.data?.categories ?? []).map((c) => ({ ...c, source: SOURCES[i]! })),
  );
  const loading = sources.some((q) => q.isPending);
  const failed = sources.every((q) => q.isError);

  const [categoryId, setCategoryId] = useState('');
  const [fields, setFields] = useState<string[]>([]);
  const [filters, setFilters] = useState<Record<string, string>>({});
  const category = categories.find((c) => c.id === categoryId);

  const choose = (next: Category) => {
    setCategoryId(next.id);
    setFields(next.defaultFields);
    setFilters({});
  };

  const body: ExportRequest | null = category && fields.length > 0 ? { category: category.id, fields, filters } : null;
  const debounced = useDebounced(body, 400);
  const preview = useQuery({
    queryKey: ['exports', 'preview', category?.source, debounced],
    queryFn: () => exportPreview(category!.source, debounced!),
    enabled: Boolean(category && debounced && debounced.category === category.id),
    retry: false,
  });

  const download = useMutation({
    mutationFn: () => exportDownload(category!.source, body!),
    onSuccess: ({ blob, filename }) => saveBlob(blob, filename ?? `${category!.id}-export.csv`),
    onError: (error) => toast.error(error.message || 'Erreur lors du téléchargement'),
  });

  const labelFor = (id: string) => category?.fields.find((f) => f.id === id)?.label ?? id;
  const rows = preview.data?.rows ?? [];
  const headers = preview.data?.headers ?? [];
  const total = preview.data?.totalCount ?? 0;

  return (
    <>
      <div className="flex items-center justify-between mb-7 flex-wrap gap-3">
        <div>
          <h1 className="text-2xl">Exports CSV</h1>
          <p className="text-soft text-sm mt-1">Exportez les données de la plateforme au format CSV.</p>
        </div>
        <button className="btn btn-primary" disabled={!body || download.isPending} onClick={() => download.mutate()}>
          {download.isPending ? 'Téléchargement…' : '⬇ Télécharger'}
        </button>
      </div>

      {loading ? (
        <div className="loading-state !py-12 !px-0">
          <div className="spinner" />
          <span className="text-soft text-sm">Chargement…</span>
        </div>
      ) : failed ? (
        <p className="text-bad p-6">{sources[0]?.error?.message}</p>
      ) : (
        <div className="export-grid">
          <div className="card export-config-panel">
            <h2 className="mb-4">Catégorie</h2>
            <div className="flex flex-col gap-2 mb-6">
              {categories.map((c) => (
                <button
                  key={`${c.source}|${c.id}`}
                  className={`export-cat-btn${c.id === categoryId ? ' active' : ''}`}
                  aria-pressed={c.id === categoryId}
                  onClick={() => choose(c)}
                >
                  {c.label}
                </button>
              ))}
            </div>

            {category && (
              <div>
                <h2 className="mb-3">Champs</h2>
                <div className="flex gap-2 mb-3">
                  <button className="btn btn-secondary btn-sm" onClick={() => setFields(category.fields.map((f) => f.id))}>
                    Tout sélectionner
                  </button>
                  <button className="btn btn-secondary btn-sm" onClick={() => setFields(category.defaultFields)}>
                    Réinitialiser
                  </button>
                </div>
                <div className="flex flex-col gap-1.5">
                  {category.fields.map((field) => (
                    <label key={field.id} className="export-field-row">
                      <input
                        type="checkbox"
                        className="w-4 h-4 accent-blue-500"
                        checked={fields.includes(field.id)}
                        onChange={(event) =>
                          setFields((current) =>
                            event.target.checked
                              ? category.fields.map((f) => f.id).filter((id) => id === field.id || current.includes(id))
                              : current.filter((id) => id !== field.id),
                          )
                        }
                      />
                      <span className="text-sm">{field.label}</span>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {category && category.filters.length > 0 && (
              <div>
                <h2 className="mt-6 mb-3">Filtres</h2>
                <div className="flex flex-col gap-3">
                  {category.filters.map((filter) => (
                    <FilterControl
                      key={filter.id}
                      filter={filter}
                      value={filters[filter.id] ?? ''}
                      onChange={(value) =>
                        setFilters((current) => {
                          const next = { ...current };
                          if (value) next[filter.id] = value;
                          else delete next[filter.id];
                          return next;
                        })
                      }
                    />
                  ))}
                </div>
              </div>
            )}
          </div>

          <div className="card export-preview-panel">
            <div className="flex items-baseline justify-between mb-4 gap-3 flex-wrap">
              <h2>Aperçu</h2>
              <span className="text-soft text-sm">
                {preview.isError
                  ? preview.error.message
                  : preview.isSuccess && body
                    ? rows.length < total
                      ? `${rows.length} lignes affichées sur ${total.toLocaleString('fr-FR')}`
                      : `${total.toLocaleString('fr-FR')} ligne${total > 1 ? 's' : ''}`
                    : ''}
              </span>
            </div>
            {!body ? (
              <div className="text-soft text-sm py-8 text-center">Sélectionnez une catégorie et au moins un champ.</div>
            ) : preview.isError ? (
              <div className="text-soft text-sm py-8 text-center">Impossible de charger l'aperçu.</div>
            ) : !preview.isSuccess ? (
              <div className="loading-state !py-8 !px-0">
                <div className="spinner" />
              </div>
            ) : (
              <div className="admin-table-wrap overflow-x-auto">
                <table className="admin-table">
                  <thead>
                    <tr className="admin-tr-head">
                      {headers.map((header) => (
                        <th key={header} className="admin-th">
                          {labelFor(header)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.length === 0 ? (
                      <tr>
                        <td colSpan={headers.length} className="admin-td text-soft text-center py-6">
                          Aucune donnée.
                        </td>
                      </tr>
                    ) : (
                      rows.map((row, index) => (
                        <tr key={index} className="admin-tr">
                          {headers.map((header) => (
                            <td key={header} className="admin-td preview-td" title={row[header] ?? ''}>
                              {row[header] ?? ''}
                            </td>
                          ))}
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}
    </>
  );
}
