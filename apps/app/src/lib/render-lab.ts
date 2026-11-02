import { renderMarkdown, type PatternMap, type TocEntry } from '@pupitre/core/lib/markdown';

// Lab markdown: `<!-- check:N -->` markers split it into sanitised segments,
// between which the step blocks are mounted (docs/specs/014-labs.md).

const MARKER = /<!--\s*check:(\d+)\s*-->/g;

type Segment = { html: string; step: number | null };

/** Splits the lab markdown at its step markers and renders each part. */
export function renderLab(md: string, patterns: PatternMap, stepCount: number): { segments: Segment[]; toc: TocEntry[]; placed: Set<number> } {
  const segments: Segment[] = [];
  const toc: TocEntry[] = [];
  const placed = new Set<number>();
  let last = 0;
  let first = true;
  const push = (text: string, step: number | null) => {
    const rendered = renderMarkdown(text, { patterns, dropFirstHeading: first });
    first = false;
    toc.push(...rendered.toc);
    segments.push({ html: rendered.html, step });
  };
  for (const match of md.matchAll(MARKER)) {
    const step = Number(match[1]);
    if (step >= stepCount) continue; // a marker without a step is ignored
    push(md.slice(last, match.index), step);
    placed.add(step);
    last = match.index + match[0].length;
  }
  push(md.slice(last), null);
  return { segments, toc, placed };
}
