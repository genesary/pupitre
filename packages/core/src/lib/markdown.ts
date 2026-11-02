import { marked } from 'marked';

import type { MarkdownPattern } from './api-types';

import { sanitizeHtml } from '@pupitre/ui/lib/security';

// Markdown pipeline of docs/specs/012-lesson-player.md:
//   normalise → admonitions → patterns → marked → DOMPurify → DOM pass.
// The rules (admonition syntax, pattern syntax) are those of pupitre's
// pages/courses/[slug]/lessons/[index].astro. Sanitisation runs on the final
// HTML, after the patterns are inserted (spec 003 S2).

export type TocEntry = { id: string; text: string; level: 2 | 3 };
export type RenderedMarkdown = { html: string; toc: TocEntry[] };

export type PatternMap = Record<string, Pick<MarkdownPattern, 'html' | 'css'>>;

const ADMONITION_ICONS: Record<string, string> = {
  note: '📝',
  tip: '💡',
  info: 'ℹ️',
  warning: '⚠️',
  danger: '🚨',
  caution: '⚡',
};

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

/** Tracks fenced code blocks line by line; returns true while inside one. */
function fenceTracker() {
  const fenceRe = /^ {0,3}(`{3,}|~{3,})/;
  let fence = '';
  return (line: string): boolean => {
    const match = fenceRe.exec(line);
    if (!fence) {
      if (match) {
        fence = match[1]!;
        return true;
      }
      return false;
    }
    if (match && match[1]![0] === fence[0] && match[1]!.length >= fence.length && !line.slice(match[0].length).trim()) {
      fence = '';
    }
    return true;
  };
}

/**
 * Docusaurus-style admonitions: `:::type`, `:::type Title` or `:::type[Title]`,
 * closed by `:::`. Not nestable; markers inside fenced code are left alone.
 */
export function processAdmonitions(md: string): string {
  const types = Object.keys(ADMONITION_ICONS).join('|');
  const openRe = new RegExp(`^:::(${types})(?:\\[([^\\]\\n]*)\\]| +(.*?))?[ \\t]*$`);
  const closeRe = /^:::[ \t]*$/;
  const lines = md.split('\n');
  const out: string[] = [];
  const inOuterFence = fenceTracker();

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;
    const open = inOuterFence(line) ? null : openRe.exec(line);
    if (!open) {
      out.push(line);
      continue;
    }
    const inBodyFence = fenceTracker();
    let end = -1;
    for (let j = i + 1; j < lines.length; j++) {
      if (!inBodyFence(lines[j]!) && closeRe.test(lines[j]!)) {
        end = j;
        break;
      }
    }
    if (end < 0) {
      out.push(line);
      continue;
    }
    const [, type, bracketTitle, spaceTitle] = open;
    const title = (bracketTitle ?? spaceTitle)?.trim() || type!.charAt(0).toUpperCase() + type!.slice(1);
    const body = lines.slice(i + 1, end).join('\n').trim();
    // Blank lines around the body let marked parse it as markdown inside the block.
    out.push(
      `<div class="admonition admonition-${type}">\n` +
        `<div class="admonition-heading"><span class="admonition-icon">${ADMONITION_ICONS[type!]}</span><strong>${escapeHtml(title)}</strong></div>\n` +
        `<div class="admonition-content">\n\n${body}\n\n</div>\n</div>`,
    );
    i = end;
  }
  return out.join('\n');
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Patterns: `|||name` / content / `|||` (block, content rendered as inline
 * markdown) and `|||name text|||` (inline). `{{content}}` in the pattern HTML
 * receives the content.
 */
export function applyPatterns(md: string, patterns: PatternMap): string {
  let out = md;
  for (const [name, pattern] of Object.entries(patterns)) {
    if (!pattern.html) continue;
    const esc = escapeRegExp(name);
    out = out.replace(new RegExp(`\\|\\|\\|${esc}\\n([\\s\\S]*?)\\n\\|\\|\\|`, 'g'), (_, content: string) =>
      pattern.html.replace(/\{\{content\}\}/g, () => marked.parseInline(content.trim(), { async: false })),
    );
    out = out.replace(new RegExp(`\\|\\|\\|${esc} ([^|\\n]+?)\\|\\|\\|`, 'g'), (_, content: string) =>
      pattern.html.replace(/\{\{content\}\}/g, () => content.trim()),
    );
  }
  return out;
}

function headingId(text: string, used: Set<string>): string {
  const base =
    text
      .toLowerCase()
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'section';
  let id = base;
  for (let n = 2; used.has(id); n++) id = `${base}-${n}`;
  used.add(id);
  return id;
}

type RenderOptions = {
  patterns?: PatternMap;
  /** Drop the first h1/h2, which repeats the module title. */
  dropFirstHeading?: boolean;
};

/** Markdown of a lesson → sanitised HTML and its table of contents. */
export function renderMarkdown(md: string, options: RenderOptions = {}): RenderedMarkdown {
  const normalized = md.replace(/\r\n?/g, '\n');
  const withBlocks = applyPatterns(processAdmonitions(normalized), options.patterns ?? {});
  const html = sanitizeHtml(marked.parse(withBlocks, { async: false, gfm: true }));

  // DOM pass on already sanitised HTML: only removals, attributes and text.
  const doc = new DOMParser().parseFromString(html, 'text/html');
  if (options.dropFirstHeading) doc.body.querySelector('h1, h2')?.remove();

  const used = new Set<string>();
  const toc: TocEntry[] = [];
  doc.body.querySelectorAll('h2, h3').forEach((heading) => {
    const text = heading.textContent?.trim() ?? '';
    heading.id = headingId(text, used);
    toc.push({ id: heading.id, text, level: heading.tagName === 'H3' ? 3 : 2 });
  });

  // ```mermaid fences become <div class="mermaid">, rendered later by mermaid.
  doc.body.querySelectorAll('pre > code.language-mermaid').forEach((code) => {
    const div = doc.createElement('div');
    div.className = 'mermaid';
    div.textContent = code.textContent ?? '';
    code.parentElement?.replaceWith(div);
  });

  return { html: doc.body.innerHTML, toc };
}

/** Inline markdown (quiz questions, answers, feedback), sanitised. */
export function renderInline(text: string): string {
  return sanitizeHtml(marked.parseInline(text, { async: false }));
}

/** Pattern CSS for the single <style> element of a lesson. */
export function patternCss(patterns: PatternMap): string {
  return Object.values(patterns)
    .map((pattern) => pattern.css)
    .filter(Boolean)
    .join('\n');
}

export function toPatternMap(patterns: MarkdownPattern[] | undefined): PatternMap {
  return Object.fromEntries((patterns ?? []).map((p) => [p.name, { html: p.html, css: p.css }]));
}
