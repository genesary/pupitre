import { describe, expect, it } from 'vitest';

import { buildTree } from '@/lib/group-tree';

const group = (id: string, parentId = '', depth = 0) => ({ id, name: id, parentId, depth, memberCount: 0 });

describe('group tree', () => {
  it('nests subgroups under their parent', () => {
    const roots = buildTree([group('a'), group('a1', 'a', 1), group('b'), group('a2', 'a', 1), group('a1x', 'a1', 2)]);
    expect(roots.map((n) => n.id)).toEqual(['a', 'b']);
    expect(roots[0]!.children.map((n) => n.id)).toEqual(['a1', 'a2']);
    expect(roots[0]!.children[0]!.children.map((n) => n.id)).toEqual(['a1x']);
  });

  it('keeps a subgroup whose parent is outside the scope as a root', () => {
    expect(buildTree([group('sub', 'elsewhere', 1)]).map((n) => n.id)).toEqual(['sub']);
  });
});

describe('subtree', () => {
  it('collects a group and all its descendants', async () => {
    const { subtreeIds } = await import('@/lib/group-tree');
    const groups = [group('a'), group('a1', 'a', 1), group('a1x', 'a1', 2), group('b'), group('b1', 'b', 1)];
    expect([...subtreeIds(groups, 'a')].sort()).toEqual(['a', 'a1', 'a1x']);
    expect([...subtreeIds(groups, 'b1')]).toEqual(['b1']);
  });
});
