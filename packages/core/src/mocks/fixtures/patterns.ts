import type { MarkdownPattern } from '../../lib/api-types';

// Two markdown patterns for the mock mode: an inline one and a block one
// (docs/specs/045-admin-patterns.md). The `js` field is filled on purpose: it
// must never run (spec 003 S2).

const createdAt = '2026-09-01T09:00:00Z';

export const MOCK_PATTERNS: MarkdownPattern[] = [
  {
    id: 'b1c2d3e4-0000-4000-8000-000000000001',
    name: 'note',
    label: 'Note',
    description: 'Une note en ligne',
    parameter: '',
    html: '<span class="pattern-note">📌 {{content}}</span>',
    css: '.pattern-note{background:var(--color-orange-soft);color:#b86800;border-radius:6px;padding:2px 8px;font-weight:600}',
    js: "alert('xss-pattern-js')",
    scope: 'global',
    fromConfig: true,
    createdAt,
    updatedAt: createdAt,
  },
  {
    id: 'b1c2d3e4-0000-4000-8000-000000000002',
    name: 'callout',
    label: 'Encadré',
    description: 'Un encadré bloc',
    parameter: '',
    html: '<div class="pattern-callout">💡 {{content}}</div>',
    css: '.pattern-callout{border:1.5px dashed var(--color-purple);background:var(--color-purple-soft);border-radius:10px;padding:12px 16px;margin:16px 0}',
    js: '',
    scope: 'global',
    fromConfig: true,
    createdAt,
    updatedAt: createdAt,
  },
];
