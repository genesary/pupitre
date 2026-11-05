import { useEffect } from 'react';

/** Pattern CSS lives in one <style> element per page. */
export function usePatternStyles(css: string) {
  useEffect(() => {
    const style = document.createElement('style');
    style.id = 'pattern-styles';
    style.textContent = css;
    document.head.appendChild(style);
    return () => style.remove();
  }, [css]);
}
