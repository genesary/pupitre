import { describe, expect, it } from 'vitest';

import { applyPatterns, processAdmonitions, renderInline, renderMarkdown } from './markdown';

const patterns = {
  note: { html: '<span class="pattern-note">📌 {{content}}</span>', css: '.pattern-note{}' },
  callout: { html: '<div class="pattern-callout">{{content}}</div>', css: '' },
};

describe('admonitions (spec 012)', () => {
  it('renders :::type Title blocks with their icon and markdown body', () => {
    const { html } = renderMarkdown(':::warning Attention\nDu **gras**.\n:::');
    expect(html).toContain('admonition admonition-warning');
    expect(html).toContain('<strong>Attention</strong>');
    expect(html).toContain('⚠️');
    expect(html).toContain('<strong>gras</strong>');
  });

  it('accepts :::type[Title] and defaults the title to the type', () => {
    expect(processAdmonitions(':::tip[Astuce]\nx\n:::')).toContain('<strong>Astuce</strong>');
    expect(processAdmonitions(':::note\nx\n:::')).toContain('<strong>Note</strong>');
  });

  it('leaves markers inside fenced code alone', () => {
    const md = '```text\n:::note\nreste du texte\n:::\n```';
    expect(processAdmonitions(md)).toBe(md);
  });

  it('escapes the title', () => {
    expect(processAdmonitions(':::note <img src=x onerror=alert(1)>\nx\n:::')).not.toContain('<img');
  });
});

describe('patterns (spec 012, defect A1)', () => {
  it('replaces inline and block patterns', () => {
    const out = applyPatterns('|||note Bonjour|||\n\n|||callout\nUn **bloc**\n|||', patterns);
    expect(out).toContain('<span class="pattern-note">📌 Bonjour</span>');
    expect(out).toContain('<div class="pattern-callout">Un <strong>bloc</strong></div>');
  });

  it('sanitises pattern HTML with the rest of the content', () => {
    const { html } = renderMarkdown('|||evil x|||', {
      patterns: { evil: { html: '<span onclick="alert(1)">{{content}}</span><script>alert(1)</script>', css: '' } },
    });
    expect(html).not.toMatch(/onclick|script/);
  });
});

describe('renderMarkdown', () => {
  it('builds the table of contents from h2 and h3 with stable ids', () => {
    const { html, toc } = renderMarkdown('## Première partie\n\n### Détail\n\n## Première partie');
    expect(toc).toEqual([
      { id: 'premiere-partie', text: 'Première partie', level: 2 },
      { id: 'detail', text: 'Détail', level: 3 },
      { id: 'premiere-partie-2', text: 'Première partie', level: 2 },
    ]);
    expect(html).toContain('id="premiere-partie"');
  });

  it('drops the first heading when asked, since it repeats the module title', () => {
    const { html, toc } = renderMarkdown('## Titre\n\nTexte\n\n## Suite', { dropFirstHeading: true });
    expect(html).not.toContain('Titre');
    expect(toc.map((t) => t.text)).toEqual(['Suite']);
  });

  it('turns mermaid fences into mermaid blocks', () => {
    const { html } = renderMarkdown('```mermaid\ngraph LR\n  A --> B\n```');
    expect(html).toContain('<div class="mermaid">graph LR\n  A --&gt; B\n</div>');
  });

  it('neutralises the XSS payloads of the audit (C3)', () => {
    const { html } = renderMarkdown(
      '<img src="x" onerror="alert(1)">\n\n<a href="javascript:alert(2)">lien</a>\n\n<script>alert(3)</script>\n\n<iframe src="https://example.com"></iframe>',
    );
    expect(html).not.toMatch(/onerror|javascript:|<script|<iframe/i);
    expect(html).toContain('lien');
  });
});

describe('renderInline', () => {
  it('renders code and bold, but no HTML payload', () => {
    expect(renderInline('`kubectl` et **pods**')).toBe('<code>kubectl</code> et <strong>pods</strong>');
    expect(renderInline('<img src=x onerror=alert(1)>')).not.toContain('onerror');
  });
});
