import { useState, type ReactNode } from 'react';

// An ordered list with the pupitre module-row markup (.mod-item), reordered by
// drag and drop on the handle or with the ↑ ↓ buttons, which keyboard users
// can reach (docs/specs/042, 043).

type SortableListProps<T> = {
  items: T[];
  keyOf: (item: T, index: number) => string;
  onMove: (from: number, to: number) => void;
  /** The row content after the handle and the position number. */
  render: (item: T, index: number) => ReactNode;
  /** Hide the handles and arrows (the course editor shows them in edit mode only). */
  locked?: boolean;
  labelOf?: (item: T) => string;
};

export function SortableList<T>({ items, keyOf, onMove, render, locked = false, labelOf }: SortableListProps<T>) {
  const [dragging, setDragging] = useState<number | null>(null);
  const [over, setOver] = useState<number | null>(null);

  return (
    <div className="stack-sm mb-3">
      {items.map((item, index) => {
        const label = labelOf?.(item) ?? `élément ${index + 1}`;
        return (
          <div
            key={keyOf(item, index)}
            className={`mod-item${dragging === index ? ' dragging' : ''}${over === index && dragging !== index ? ' drag-over' : ''}`}
            onDragOver={(event) => {
              if (dragging === null) return;
              event.preventDefault();
              setOver(index);
            }}
            onDragLeave={() => setOver((current) => (current === index ? null : current))}
            onDrop={(event) => {
              event.preventDefault();
              if (dragging !== null) onMove(dragging, index);
              setDragging(null);
              setOver(null);
            }}
          >
            {!locked && (
              <span
                className="mod-drag-handle"
                draggable
                title="Glisser pour réordonner"
                aria-hidden="true"
                onDragStart={(event) => {
                  event.dataTransfer.effectAllowed = 'move';
                  setDragging(index);
                }}
                onDragEnd={() => {
                  setDragging(null);
                  setOver(null);
                }}
              >
                ⠿
              </span>
            )}
            <span className="mod-idx">{index + 1}</span>
            {render(item, index)}
            {!locked && (
              <span className="flex flex-col shrink-0">
                <button
                  type="button"
                  className="link-btn gray leading-none"
                  aria-label={`Monter ${label}`}
                  disabled={index === 0}
                  onClick={() => onMove(index, index - 1)}
                >
                  ↑
                </button>
                <button
                  type="button"
                  className="link-btn gray leading-none"
                  aria-label={`Descendre ${label}`}
                  disabled={index === items.length - 1}
                  onClick={() => onMove(index, index + 1)}
                >
                  ↓
                </button>
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}
