import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useDebounced } from '@/lib/use-debounced';

describe('useDebounced', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('follows the value once it stops changing', () => {
    const { result, rerender } = renderHook(({ value }) => useDebounced(value, 400), { initialProps: { value: 'a' } });

    rerender({ value: 'b' });
    act(() => {
      vi.advanceTimersByTime(399);
    });
    expect(result.current).toBe('a');

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe('b');
  });

  it('settles on a stable object without rendering again', () => {
    const value = { category: 'users' };
    let renders = 0;
    renderHook(() => {
      renders += 1;
      return useDebounced(value, 400);
    });

    act(() => {
      vi.advanceTimersByTime(4000);
    });
    expect(renders).toBe(1);
  });
});
