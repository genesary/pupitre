# 047 - Admin: CSV exports and lab tracking

Features: F095 (lab tracking), F182 (CSV exports), F183 (lab check export).

## CSV exports - `/admin/exports`

Access: `admin` and `manager` ([002](002-roles-and-guards.md)).

### Categories

Two sources, merged into a single list:

| Source | Catalogue | Preview | Download |
| --- | --- | --- | --- |
| user-service | `GET /api/admin/exports/categories` | `POST /api/admin/exports/preview` | `POST /api/admin/exports/download` |
| course-service (lab checks) | `GET /api/admin/exports/lab-checks/categories` | `POST /api/admin/exports/lab-checks/preview` | `POST /api/admin/exports/lab-checks/download` |

A category: `id`, `label`, `fields[] {id, label}`, `filters[] {id, label, type, options}`. Each
category keeps the source it comes from, so the right route is called.

### Flow

1. **Category**: a list of cards. Choosing one shows its fields and filters.
2. **Fields**: checkboxes, all checked by default, with « Tout sélectionner » and
   « Réinitialiser ».
3. **Filters**: one control per filter (select, date, text), depending on `type`. The section is
   hidden if the category has none.
4. **Preview**: `POST …/preview` with `{ category, fields, filters }` →
   `{ headers, rows, totalCount }`. Table of the first rows and « N lignes au total ». The preview
   runs again when the fields or filters change (400 ms debounce).
5. **« ⬇ Télécharger »** (disabled without a category or a field): `POST …/download` with the same
   body. The response is a UTF-8 CSV (BOM, `;` separator), downloaded under the name given by
   `Content-Disposition` (`<category>-export-<date>.csv`).

### Security

- Preview cells are shown as text ([003](003-security.md) S1).
- The CSV comes from the server as is. Formula injection (a cell starting with `=`, `+`, `-` or
  `@`, finding M4) is fixed on the server.
- For a manager, the server exports the members of the groups they manage only (finding H2,
  fixed in [190](190-backend-defects.md) B6).

## Lab tracking - `/admin/labs`

Access: `admin`.

- Data: `GET /api/admin/lab-checks[?course=<slug>]` → array of
  `{ id, username, courseSlug, moduleIndex, moduleName, allow, violations[], verified, checkedAt }`.
- « Suivi des Labs » title, « Tous les cours » or a course filter, « Actualiser » button.
- Statistics: checks, passed, pass rate.
- Table: learner, course, lab (`moduleName`), status (« ✓ Validé » or « ✗ Échec »), violations
  (expandable), verification (« Serveur » if `verified`, « Auto-déclaré » otherwise), date
  (`fr-FR`).
- Sorted by date, descending.

## Acceptance criteria

- Choosing the lab-check category calls the `/api/admin/exports/lab-checks/*` routes.
- The downloaded file keeps the name given by the server.
- A local check appears as « Auto-déclaré ».

## Sources

`frontend/src/pages/admin/exports/index.astro`, `admin/labs/index.astro`;
`user-service/internal/handlers/exports.go`; `course-service/internal/handlers/exports.go`,
`lab_results.go`.
