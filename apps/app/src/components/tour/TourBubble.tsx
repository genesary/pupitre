import { useId, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from 'react';

import { SPOTLIGHT_MARGIN } from './TourSpotlight';

// The bird of the logo and its speech bubble (docs/specs/007-guided-tour.md, "Display").
// The bubble goes below the target, above it when there is no room, and is
// docked at the bottom of narrow screens. Moving it is the bird's flight.

const GAP = 14;
const EDGE = 16;
/** Below this width the bubble is docked at the bottom of the screen. */
const DOCK_WIDTH = 640;

type Position = { x: number; y: number };

function place(rect: DOMRect | null, width: number, height: number): Position {
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const clamp = (value: number, max: number) => Math.min(Math.max(value, EDGE), Math.max(EDGE, max));
  if (!rect) return { x: (vw - width) / 2, y: (vh - height) / 2 };
  const x = clamp(rect.left + rect.width / 2 - width / 2, vw - EDGE - width);
  const below = rect.bottom + SPOTLIGHT_MARGIN + GAP;
  if (below + height <= vh - EDGE) return { x, y: below };
  const above = rect.top - SPOTLIGHT_MARGIN - GAP - height;
  if (above >= EDGE) return { x, y: above };
  return { x, y: clamp(vh - EDGE - height, vh - EDGE - height) };
}

function useDocked() {
  const [docked, setDocked] = useState(() => window.innerWidth < DOCK_WIDTH);
  useLayoutEffect(() => {
    const update = () => setDocked(window.innerWidth < DOCK_WIDTH);
    window.addEventListener('resize', update);
    return () => window.removeEventListener('resize', update);
  }, []);
  return docked;
}

/**
 * Keeps Tab inside the bubble while the tour runs, also while a step waits for
 * its target and the bubble itself holds focus.
 */
function trapTab(event: KeyboardEvent<HTMLDivElement>) {
  if (event.key !== 'Tab') return;
  const focusable = [...event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled)')];
  if (focusable.length === 0) {
    event.preventDefault();
    return;
  }
  const first = focusable[0]!;
  const last = focusable[focusable.length - 1]!;
  const onBubble = document.activeElement === event.currentTarget;
  if (event.shiftKey && (onBubble || document.activeElement === first)) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

type TourBubbleProps = {
  /** The highlighted target, or null for a centred bubble. */
  rect: DOMRect | null;
  /** Changes at every step: the bird flies and focus moves to `focusRef`. */
  stepKey: string;
  title: ReactNode;
  counter?: string;
  children: ReactNode;
  actions: ReactNode;
  focusRef?: RefObject<HTMLButtonElement | null>;
  /** The first-visit offer: bottom right, not placed on a target. */
  offer?: boolean;
  onClose?: () => void;
  closeLabel?: string;
};

export function TourBubble({
  rect,
  stepKey,
  title,
  counter,
  children,
  actions,
  focusRef,
  offer = false,
  onClose,
  closeLabel,
}: TourBubbleProps) {
  const docked = useDocked();
  const ref = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const textId = useId();
  const [size, setSize] = useState({ width: 320, height: 160 });

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => setSize({ width: element.offsetWidth, height: element.offsetHeight }));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  // While a step waits for its target there is no button: the bubble holds
  // focus, so the keyboard never reaches the page under the layer.
  useLayoutEffect(() => {
    if (offer) return;
    (focusRef?.current ?? ref.current)?.focus({ preventScroll: true });
  }, [stepKey, focusRef, offer]);

  const position = place(rect, size.width, size.height);
  const placed = !docked && !offer;

  return (
    <div
      ref={ref}
      className={`tour-coach${docked ? ' docked' : ''}${offer ? ' offer' : ''}`}
      style={placed ? { transform: `translate(${position.x}px, ${position.y}px)` } : undefined}
      role="dialog"
      tabIndex={offer ? undefined : -1}
      aria-modal={offer ? undefined : true}
      aria-labelledby={titleId}
      aria-describedby={textId}
      onKeyDown={offer ? undefined : trapTab}
    >
      {/* Keyed by step: the card fades in and the bird flaps again at each step. */}
      <div className="tour-card" key={stepKey}>
        <div className="tour-head">
          <img src="/pioupitre.png" alt="" className="tour-bird" />
          <div>
            <div id={titleId} className="tour-title">
              {title}
            </div>
            {counter && <div className="tour-count">{counter}</div>}
          </div>
          {onClose && (
            <button type="button" className="tour-close" aria-label={closeLabel} onClick={onClose}>
              ×
            </button>
          )}
        </div>
        <div id={textId} className="tour-text">
          {children}
        </div>
        <div className="tour-actions">{actions}</div>
      </div>
    </div>
  );
}
