import { describe, expect, it } from 'vitest';

import { isEmoji, safeExternalUrl, safeRedirect, safeUrl, sanitizeHtml } from './security';

describe('safeRedirect (genesary/pupitre#194)', () => {
  it.each([
    ['/courses', '/courses'],
    ['/courses/kubernetes-basics?tab=modules#intro', '/courses/kubernetes-basics?tab=modules#intro'],
    ['/admin', '/admin'],
  ])('keeps the relative path %s', (raw, expected) => {
    expect(safeRedirect(raw, '/fallback')).toBe(expected);
  });

  it.each([
    'https://example.com/',
    '//example.com',
    '/\\example.com',
    '\\\\example.com',
    'javascript:alert(1)',
    'JaVaScRiPt:alert(1)',
    'data:text/html,<script>alert(1)</script>',
    '/\t/example.com',
    ' /courses',
    'courses',
    '',
  ])('rejects %j', (raw) => {
    expect(safeRedirect(raw, '/fallback')).toBe('/fallback');
  });

  it('uses the fallback when the parameter is missing', () => {
    expect(safeRedirect(null, '/courses')).toBe('/courses');
  });
});

describe('safeUrl', () => {
  it.each(['https://example.com/a.png', 'http://example.com/', '/uploads/video.mp4'])(
    'accepts %s',
    (raw) => expect(safeUrl(raw)).not.toBeNull(),
  );

  it.each([
    'javascript:alert(1)',
    'data:image/svg+xml;base64,AAAA',
    'vbscript:msgbox',
    '',
    '  ',
    'https://x" onerror="alert(1)',
    "https://x' onerror='alert(1)",
    'https://exa<mple.com/',
    'https://exa%22mple.com/',
  ])('rejects %j', (raw) => expect(safeUrl(raw)).toBeNull());
});

describe('safeExternalUrl', () => {
  it('accepts https', () => {
    expect(safeExternalUrl('https://keycloak.example.com/auth')).toBe('https://keycloak.example.com/auth');
  });

  it('rejects plain http on another host', () => {
    expect(safeExternalUrl('http://evil.example.com/')).toBeNull();
  });
});

describe('sanitizeHtml (spec 003 S2)', () => {
  it('removes scripts and event handlers', () => {
    const html = sanitizeHtml('<p>ok</p><script>alert(1)</script><img src="x" onerror="alert(1)">');
    expect(html).toContain('<p>ok</p>');
    expect(html).not.toMatch(/script|onerror/i);
  });

  it('drops javascript: links but keeps the text', () => {
    const html = sanitizeHtml('<a href="javascript:alert(1)">clic</a>');
    expect(html).not.toContain('javascript');
    expect(html).toContain('clic');
  });

  it('removes frames, forms and style elements', () => {
    const html = sanitizeHtml(
      '<iframe src="https://example.com"></iframe><form><button>go</button></form><style>body{}</style>',
    );
    expect(html).not.toMatch(/iframe|form|button|style/i);
  });

  it('keeps disabled task-list checkboxes only', () => {
    const html = sanitizeHtml('<input type="checkbox" checked><input type="text" value="x">');
    expect(html).toContain('type="checkbox"');
    expect(html).toContain('disabled');
    expect(html).not.toContain('type="text"');
  });

  it('opens external links in a new tab without opener', () => {
    const html = sanitizeHtml('<a href="https://example.com">doc</a>');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it('keeps markdown output intact', () => {
    const html = sanitizeHtml('<h2 id="intro">Intro</h2><pre><code class="language-bash">ls</code></pre>');
    expect(html).toBe('<h2 id="intro">Intro</h2><pre><code class="language-bash">ls</code></pre>');
  });
});

describe('isEmoji', () => {
  it.each(['🦊', '🏅', '👩‍💻', '❤️'])('accepts %s', (value) => expect(isEmoji(value)).toBe(true));
  it.each(['a', '<b>', '🦊🦊x', '', 'https://x'])('rejects %j', (value) =>
    expect(isEmoji(value)).toBe(false),
  );
});
