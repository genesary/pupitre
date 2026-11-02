import { useEffect, useRef } from 'react';

import { SafeHtml } from '@pupitre/ui/components/safe-html';
import { useTheme } from '@pupitre/ui/lib/theme';

// Sanitised lesson HTML plus the two enhancements of the Astro lesson page:
// « Copier » buttons on code blocks and mermaid diagrams that follow the theme
// (docs/specs/012-lesson-player.md). Both build DOM nodes with text only.

function addCopyButtons(root: HTMLElement) {
  root.querySelectorAll('pre').forEach((pre) => {
    if (pre.parentElement?.classList.contains('code-wrapper')) return;
    const wrapper = document.createElement('div');
    wrapper.className = 'code-wrapper';
    pre.parentNode?.insertBefore(wrapper, pre);
    wrapper.appendChild(pre);
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'copy-btn';
    button.textContent = 'Copier';
    button.addEventListener('click', () => {
      const text = pre.querySelector('code')?.textContent ?? pre.textContent ?? '';
      void navigator.clipboard.writeText(text).then(() => {
        button.textContent = '✓ Copié';
        button.classList.add('copied');
        setTimeout(() => {
          button.textContent = 'Copier';
          button.classList.remove('copied');
        }, 2000);
      });
    });
    wrapper.appendChild(button);
  });
}

async function renderMermaid(root: HTMLElement, dark: boolean) {
  const nodes = [...root.querySelectorAll<HTMLElement>('.mermaid')];
  if (nodes.length === 0) return;
  for (const node of nodes) {
    node.dataset.src ??= node.textContent ?? '';
    node.removeAttribute('data-processed');
    node.textContent = node.dataset.src;
  }
  const { default: mermaid } = await import('mermaid');
  mermaid.initialize({ startOnLoad: false, theme: dark ? 'dark' : 'default', securityLevel: 'strict' });
  await mermaid.run({ nodes, suppressErrors: true });
}

export function MarkdownContent({ html, className = 'prose' }: { html: string; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const theme = useTheme();

  useEffect(() => {
    if (ref.current) addCopyButtons(ref.current);
  }, [html]);

  useEffect(() => {
    if (ref.current) renderMermaid(ref.current, theme === 'dark').catch((error) => console.warn('Mermaid:', error));
  }, [html, theme]);

  return (
    <div ref={ref}>
      <SafeHtml html={html} className={className} />
    </div>
  );
}
