import { describe, expect, it } from 'vitest';

import { moveItem } from '@/lib/reorder';

describe('moveItem', () => {
  it('moves an item up or down', () => {
    expect(moveItem(['a', 'b', 'c'], 0, 2)).toEqual(['b', 'c', 'a']);
    expect(moveItem(['a', 'b', 'c'], 2, 0)).toEqual(['c', 'a', 'b']);
  });

  it('leaves the list unchanged out of bounds', () => {
    const list = ['a', 'b'];
    expect(moveItem(list, 0, 5)).toBe(list);
    expect(moveItem(list, 1, 1)).toBe(list);
  });
});
