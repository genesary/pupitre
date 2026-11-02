import type { ManagerGroup } from '@pupitre/core/lib/api-types';

// Group hierarchies of the manager space and the admin groups page
// (docs/specs/030, 044), built from a flat list with parentId.

type Flat = { id: string; parentId?: string | null };

export type TreeNode<T extends Flat> = T & { children: TreeNode<T>[] };

export type GroupNode = TreeNode<ManagerGroup>;

/** A group whose parent is not in the list is a root, in the order of the list. */
export function buildTree<T extends Flat>(groups: T[]): TreeNode<T>[] {
  const byId = new Map(groups.map((g) => [g.id, { ...g, children: [] as TreeNode<T>[] }]));
  const roots: TreeNode<T>[] = [];
  for (const group of groups) {
    const node = byId.get(group.id)!;
    const parent = group.parentId ? byId.get(group.parentId) : undefined;
    if (parent) parent.children.push(node);
    else roots.push(node);
  }
  return roots;
}

/** The ids of a group and all its descendants: places it cannot be moved under. */
export function subtreeIds<T extends Flat>(groups: T[], id: string): Set<string> {
  const ids = new Set([id]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const group of groups) {
      if (group.parentId && ids.has(group.parentId) && !ids.has(group.id)) {
        ids.add(group.id);
        grew = true;
      }
    }
  }
  return ids;
}
