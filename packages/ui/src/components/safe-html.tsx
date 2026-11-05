import { useMemo } from 'react';

import { sanitizeHtml } from '../lib/security';

type SafeHtmlProps = {
  html: string;
  className?: string;
  /** Element to render; `span` for inline text inside buttons or paragraphs. */
  as?: 'div' | 'span' | 'p';
};

/**
 * The only component allowed to render raw HTML. It sanitizes
 * its input itself, so a caller cannot forget to.
 */
export function SafeHtml({ html, className, as: Tag = 'div' }: SafeHtmlProps) {
  // Memoised: a quiz re-renders every second while a cooldown runs.
  const clean = useMemo(() => sanitizeHtml(html), [html]);
  return <Tag className={className} dangerouslySetInnerHTML={{ __html: clean }} />;
}
