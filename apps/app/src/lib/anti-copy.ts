import type { ClipboardEvent, MouseEvent } from 'react';

/** Anti-copy of the quiz containers (spec 013): a deterrent, not a protection. */
export const antiCopy = {
  onCopy: (event: ClipboardEvent) => event.preventDefault(),
  onCut: (event: ClipboardEvent) => event.preventDefault(),
  onContextMenu: (event: MouseEvent) => {
    if (!(event.target as HTMLElement).closest('input, textarea')) event.preventDefault();
  },
};
