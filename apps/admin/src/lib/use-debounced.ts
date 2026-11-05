import { useEffect, useState } from 'react';

/**
 * `value`, once it has stopped changing for `delay` ms. Values are compared by
 * identity: an object rebuilt on every render never settles and re-renders the
 * caller for ever, so memoise it.
 */
export function useDebounced<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}
