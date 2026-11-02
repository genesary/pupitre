import DOMPurify from 'dompurify';

// Security helpers of docs/specs/003-security.md. Every redirect driven by a
// parameter, every dynamic href/src and every piece of HTML built from course
// content goes through one of these functions.

/**
 * Returns `raw` only if it is a same-origin relative path ("/courses/x?y#z"),
 * otherwise `fallback`. Rejects absolute URLs, protocol-relative "//host",
 * backslash tricks and any scheme such as `javascript:` (genesary/pupitre#194).
 */
export function safeRedirect(raw: string | null | undefined, fallback: string): string {
  if (!raw || !/^\/(?![/\\])/.test(raw)) return fallback;
  // Control characters and whitespace can be stripped by URL parsers and turn
  // "/\t/evil.com" into "//evil.com".
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001F\u007F\s\\]/.test(raw)) return fallback;
  try {
    const url = new URL(raw, window.location.origin);
    if (url.origin !== window.location.origin) return fallback;
    return url.pathname + url.search + url.hash;
  } catch {
    return fallback;
  }
}

const SAFE_PROTOCOLS = new Set(['http:', 'https:']);

// Browsers disagree on hostnames: Chrome accepts and percent-encodes
// `https://x" onerror="…`, jsdom rejects it. Only plain DNS names and IPv4
// addresses are allowed, so the result is the same everywhere.
const PLAIN_HOSTNAME = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/i;

/**
 * Returns a URL that is safe to put in `href` or `src`: a relative path or an
 * http(s) URL. Anything else (`javascript:`, `data:`, `vbscript:`…) yields null.
 */
export function safeUrl(raw: string | null | undefined): string | null {
  if (!raw) return null;
  const value = raw.trim();
  if (value === '') return null;
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001F\u007F]/.test(value)) return null;
  if (/["'<>`\s]/.test(value)) return null;
  try {
    const url = new URL(value, window.location.origin);
    if (!SAFE_PROTOCOLS.has(url.protocol)) return null;
    if (!PLAIN_HOSTNAME.test(url.hostname)) return null;
    return value.startsWith('/') && !value.startsWith('//') ? value : url.href;
  } catch {
    return null;
  }
}

/**
 * Like safeUrl, but for URLs that take the user away from the application
 * (Keycloak, a lab's URL): https only, plus http on localhost during development.
 */
export function safeExternalUrl(raw: string | null | undefined): string | null {
  const url = safeUrl(raw);
  if (!url) return null;
  const parsed = new URL(url, window.location.origin);
  if (parsed.protocol === 'https:') return parsed.href;
  if (parsed.origin === window.location.origin) return parsed.href;
  if (import.meta.env.DEV && ['localhost', '127.0.0.1'].includes(parsed.hostname)) {
    return parsed.href;
  }
  return null;
}

const FORBIDDEN_TAGS = ['script', 'style', 'iframe', 'object', 'embed', 'form', 'button', 'textarea', 'select'];
const LINK_ATTRIBUTES = ['href', 'src', 'xlink:href', 'action', 'formaction'];

let hooksInstalled = false;

function installHooks() {
  if (hooksInstalled) return;
  hooksInstalled = true;

  DOMPurify.addHook('uponSanitizeElement', (node, data) => {
    // GFM task lists render <input type="checkbox" disabled>: keep those, drop
    // every other input.
    if (data.tagName === 'input' && node instanceof Element) {
      if (node.getAttribute('type') !== 'checkbox') node.remove();
      else node.setAttribute('disabled', '');
    }
  });

  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    for (const attr of LINK_ATTRIBUTES) {
      const value = node.getAttribute(attr);
      if (value === null) continue;
      const allowed =
        value.startsWith('#') || value.startsWith('mailto:') ? value : safeUrl(value);
      if (allowed === null) node.removeAttribute(attr);
    }
    if (node.tagName === 'A' && node.hasAttribute('href')) {
      const href = node.getAttribute('href') ?? '';
      const external = /^https?:\/\//i.test(href) && !href.startsWith(window.location.origin);
      if (external) {
        node.setAttribute('target', '_blank');
        node.setAttribute('rel', 'noopener noreferrer');
      }
    }
  });
}

/**
 * Sanitizes HTML produced from course markdown, patterns included
 * (spec 003 S2). The result is the only kind of HTML <SafeHtml> accepts.
 */
export function sanitizeHtml(html: string): string {
  installHooks();
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    ADD_ATTR: ['target'],
    FORBID_TAGS: FORBIDDEN_TAGS,
    ALLOW_DATA_ATTR: false,
  });
}

/** True for a single emoji, the only non-URL avatar value we render (spec 003 S3). */
export function isEmoji(value: string): boolean {
  return /^\p{Extended_Pictographic}(️|‍\p{Extended_Pictographic})*️?$/u.test(value);
}
