import { describe, expect, it } from 'vitest';

import { renderLab } from '@/lib/render-lab';

describe('renderLab (spec 014)', () => {
  const md = '## Lab\n\n### Étape 1\n\nFaire A.\n\n<!-- check:0 -->\n\n### Étape 2\n\n<!-- check:1 -->\n\nFin.';

  it('places each step at its marker', () => {
    const { segments, placed } = renderLab(md, {}, 2);
    expect(segments.map((s) => s.step)).toEqual([0, 1, null]);
    expect(segments[0]!.html).toContain('Faire A.');
    expect(segments[2]!.html).toContain('Fin.');
    expect([...placed]).toEqual([0, 1]);
  });

  it('ignores a marker without a step', () => {
    const { segments, placed } = renderLab('Avant\n\n<!-- check:5 -->\n\nAprès', {}, 1);
    expect(segments).toHaveLength(1);
    expect(placed.size).toBe(0);
  });

  it('drops the first heading and collects the table of contents across segments', () => {
    const { segments, toc } = renderLab(md, {}, 2);
    expect(segments[0]!.html).not.toContain('>Lab<');
    expect(toc.map((t) => t.text)).toEqual(['Étape 1', 'Étape 2']);
  });
});
